package foldersync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/common"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/go-threads/integrationtests/foldersync/watcher"
	"github.com/textileio/go-threads/util"
)

var (
	log = logging.Logger("foldersync")

	collectionName = "sharedFolder"

	cc = db.CollectionConfig{
		Name:   collectionName,
		Schema: util.SchemaFromInstance(folder{}, false),
	}

	errClientAlreadyStarted = errors.New("client already started")
)

type folder struct {
	ID    core.InstanceID `json:"_id"`
	Owner string
	Files []file
}

type file struct {
	ID               core.InstanceID `json:"_id"`
	FileRelativePath string
	CID              string

	IsDirectory bool
	Files       []file
}

type client struct {
	sync.Mutex
	wg             sync.WaitGroup
	started        bool
	closed         bool
	name           string
	folderPath     string
	folderInstance *folder
	db             *db.DB
	collection     *db.Collection
	net            net.Net
	peer           *ipfslite.Peer
	closeCh        chan struct{}
}

func newRootClient(name, folderPath, repoPath string) (*client, error) {
	id := thread.NewIDV1(thread.Raw, 32)

	network, err := newNetwork(repoPath)
	if err != nil {
		return nil, err
	}

	d, err := db.NewDB(context.Background(), network, id, db.WithNewRepoPath(repoPath), db.WithNewCollections(cc))
	if err != nil {
		return nil, err
	}

	return &client{
		name:       name,
		folderPath: folderPath,
		db:         d,
		collection: d.GetCollection(collectionName),
		net:        network,
		peer:       network.GetIpfsLite(),
		closeCh:    make(chan struct{}),
	}, nil
}

func newJoinerClient(name, folderPath, repoPath string, addr ma.Multiaddr, key thread.Key) (*client, error) {
	network, err := newNetwork(repoPath)
	if err != nil {
		return nil, err
	}

	d, err := db.NewDBFromAddr(context.Background(), network, addr, key, db.WithNewRepoPath(repoPath), db.WithNewCollections(cc))
	if err != nil {
		return nil, err
	}

	return &client{
		name:       name,
		folderPath: folderPath,
		db:         d,
		collection: d.GetCollection(collectionName),
		net:        network,
		peer:       network.GetIpfsLite(),
		closeCh:    make(chan struct{}),
	}, nil
}

func newNetwork(repoPath string) (common.NetBoostrapper, error) {
	network, err := common.DefaultNetwork(repoPath, common.WithNetDebug(true), common.WithNetHostAddr(util.FreeLocalAddr()))
	if err != nil {
		return nil, err
	}
	return network, nil
}

func (c *client) getDBInfo() (ma.Multiaddr, thread.Key, error) {
	addrs, key, err := c.db.GetDBInfo()
	if err != nil {
		return nil, thread.Key{}, err
	}
	if len(addrs) < 1 {
		return nil, thread.Key{}, errors.New("unable to get thread address")
	}
	return addrs[0], key, nil
}

func (c *client) getOrCreateMyFolderInstance(path string) (*folder, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err = os.MkdirAll(path, os.ModePerm); err != nil {
			return nil, err
		}
	}

	var myFolder *folder

	res, err := c.collection.Find(db.Where("Owner").Eq(c.name))
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		ownFolder := folder{ID: core.NewInstanceID(), Owner: c.name, Files: []file{}}
		jsn := util.JSONFromInstance(ownFolder)
		_, err := c.collection.Create(jsn)
		if err != nil {
			return nil, err
		}
		myFolder = &ownFolder
	} else {
		ownFolder := &folder{}
		if err := json.Unmarshal(res[0], ownFolder); err != nil {
			return nil, err
		}
		myFolder = ownFolder
	}

	return myFolder, nil
}

func (c *client) startListeningExternalChanges() error {
	l, err := c.db.Listen()
	if err != nil {
		return err
	}
	go func() {
		defer c.wg.Done()
		defer l.Close()
		for {
			select {
			case <-c.closeCh:
				log.Infof("shutting down external changes listener - %v", c.name)
				return
			case a := <-l.Channel():
				instanceBytes, err := c.collection.FindByID(a.ID)
				if err != nil {
					log.Errorf("error when getting changed user folder with ID %v: %v", a.ID, err)
					continue
				}
				uf := &folder{}
				util.InstanceFromJSON(instanceBytes, uf)
				log.Infof("%s: detected new file %s of user %s", c.name, a.ID, uf.Owner)
				for _, f := range uf.Files {
					if err := c.ensureCID(c.fullPath(f), f.CID); err != nil {
						log.Warnf("%s: error ensuring file %s: %v", c.name, c.fullPath(f), err)
					}
				}
			}
		}
	}()
	return nil
}

func (c *client) startFSWatcher() error {
	myFolderPath := path.Join(c.folderPath, c.name)
	myFolder, err := c.getOrCreateMyFolderInstance(myFolderPath)
	if err != nil {
		return fmt.Errorf("error when getting folder for %v: %v", c.name, err)
	}
	c.folderInstance = myFolder

	w, err := watcher.New(myFolderPath, func(fileName string) error {
		f, err := os.Open(fileName)
		if err != nil {
			return err
		}
		n, err := c.peer.AddFile(context.Background(), f, nil)
		if err != nil {
			return err
		}

		fileRelPath := strings.TrimPrefix(fileName, c.folderPath)
		fileRelPath = strings.TrimLeft(fileRelPath, "/")
		newFile := file{ID: core.NewInstanceID(), FileRelativePath: fileRelPath, CID: n.Cid().String(), Files: []file{}}
		c.folderInstance.Files = append(c.folderInstance.Files, newFile)
		return c.collection.Save(util.JSONFromInstance(c.folderInstance))
	})
	if err != nil {
		return fmt.Errorf("error when creating fs watcher for %v: %v", c.name, err)
	}
	w.Watch()

	go func() {
		<-c.closeCh
		w.Close()
		c.wg.Done()
	}()
	return nil
}

func (c *client) start() error {
	c.Lock()
	defer c.Unlock()

	if c.started {
		return errClientAlreadyStarted
	}

	c.wg.Add(2)
	if err := c.startListeningExternalChanges(); err != nil {
		return err
	}
	if err := c.startFSWatcher(); err != nil {
		return err
	}
	c.started = true
	return nil
}

func (c *client) getDirectoryTree() ([]*folder, error) {
	res, err := c.collection.Find(nil)
	if err != nil {
		return nil, err

	}
	folders := make([]*folder, len(res))
	for i, item := range res {
		folder := &folder{}
		if err := json.Unmarshal(item, folder); err != nil {
			return nil, err
		}
		folders[i] = folder
	}
	return folders, nil
}

func (c *client) fullPath(f file) string {
	return filepath.Join(c.folderPath, f.FileRelativePath)
}

func (c *client) ensureCID(fullPath, cidStr string) error {
	id, err := cid.Decode(cidStr)
	if err != nil {
		return err
	}
	_, err = os.Stat(fullPath)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}

	log.Infof("Fetching file %s", fullPath)
	d1 := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	str, err := c.peer.GetFile(ctx, id)
	if err != nil {
		return err
	}
	defer str.Close()
	if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = io.Copy(f, str); err != nil {
		return err
	}
	d2 := time.Now()
	d := d2.Sub(d1)
	log.Infof("Done fetching %s in %dms", fullPath, d.Milliseconds())
	return nil
}

func (c *client) close() error {
	c.Lock()
	defer c.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	close(c.closeCh)

	c.wg.Wait()

	if err := c.db.Close(); err != nil {
		return err
	}
	if err := c.net.Close(); err != nil {
		return err
	}
	return nil
}
