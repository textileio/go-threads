package main

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
	cid "github.com/ipfs/go-cid"
	"github.com/mr-tron/base58"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/go-threads/examples/foldersync/watcher"
	"github.com/textileio/go-threads/util"
)

var (
	errClientAlreadyStarted = errors.New("client already started")
)

type client struct {
	lock          sync.Mutex
	closeIpfsLite func() error
	wg            sync.WaitGroup
	closeCh       chan struct{}
	started       bool
	closed        bool

	shrFolderPath  string
	userName       string
	folderInstance *userFolder

	net        net.Net
	db         *db.DB
	collection *db.Collection
	peer       *ipfslite.Peer
}

type userFolder struct {
	ID    core.InstanceID
	Owner string
	Files []file
}

type file struct {
	ID               core.InstanceID
	FileRelativePath string
	CID              string

	IsDirectory bool
	Files       []file
}

func newClient(name, sharedFolderPath, repoPath, inviteLink string) (*client, error) {
	n, err := db.DefaultNetwork(repoPath)
	if err != nil {
		return nil, err
	}

	id := thread.NewIDV1(thread.Raw, 32)
	creds := thread.NewDefaultCreds(id)

	cc := db.CollectionConfig{
		Name:   "sharedFolder",
		Schema: util.SchemaFromInstance(userFolder{}, false),
	}

	var d *db.DB
	if inviteLink == "" {
		d, err = db.NewDB(context.Background(), n, creds, db.WithRepoPath(repoPath), db.WithCollections(cc))
		if err != nil {
			return nil, fmt.Errorf("error when creating db: %v", err)
		}
	} else {
		addr, key := parseInviteLink(inviteLink)
		d, err = db.NewDBFromAddr(context.Background(), n, creds, addr, key, db.WithRepoPath(repoPath), db.WithCollections(cc))
	}

	ipfspeer := n.GetIpfsLite()
	if err != nil {
		return nil, fmt.Errorf("error when creating ipfs lite peer: %v", err)
	}

	return &client{
		closeIpfsLite: func() error { return nil },
		closeCh:       make(chan struct{}),
		net:           n,
		db:            d,
		collection:    d.GetCollection("sharedFolder"),
		peer:          ipfspeer,
		shrFolderPath: sharedFolderPath,
		userName:      name,
	}, nil
}

func (c *client) close() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	close(c.closeCh)
	c.wg.Wait()
	if err := c.closeIpfsLite(); err != nil {
		return err
	}
	c.db.Close()
	if err := c.net.Close(); err != nil {
		return err
	}

	return nil
}

func (c *client) getDirectoryTree() ([]*userFolder, error) {
	res, err := c.collection.Find(nil)
	if err != nil {
		return nil, err

	}
	folders := make([]*userFolder, len(res))
	for i, item := range res {
		folder := &userFolder{}
		if err := json.Unmarshal(item, folder); err != nil {
			return nil, err
		}
		folders[i] = folder
	}
	return folders, nil
}

func (c *client) inviteLinks() ([]string, error) {
	addrs, key, err := c.db.GetInviteInfo()
	if err != nil {
		return nil, err
	}
	res := make([]string, len(addrs))
	for i := range addrs {
		res[i] = addrs[i].String() + "?" + base58.Encode(key.Bytes())
	}
	return res, nil
}

func (c *client) fullPath(f file) string {
	return filepath.Join(c.shrFolderPath, f.FileRelativePath)
}

func (c *client) start() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.started {
		return errClientAlreadyStarted
	}

	c.wg.Add(2)
	if err := c.startFileSystemWatcher(); err != nil {
		return err
	}
	if err := c.startListeningExternalChanges(); err != nil {
		return err
	}

	c.started = true

	return nil
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
				log.Info("shutting down external changes listener")
				return
			case a := <-l.Channel():
				instanceBytes, err := c.collection.FindByID(a.ID)
				if err != nil {
					log.Errorf("error when getting changed user folder with ID %s", a.ID)
					continue
				}
				uf := &userFolder{}
				util.InstanceFromJSON(instanceBytes, uf)
				log.Infof("%s: detected new file %s of user %s", c.userName, a.ID, uf.Owner)
				for _, f := range uf.Files {
					if err := c.ensureCID(c.fullPath(f), f.CID); err != nil {
						log.Warningf("%s: error ensuring file %s: %v", c.userName, c.fullPath(f), err)
					}
				}
			}
		}
	}()
	return nil
}

func (c *client) startFileSystemWatcher() error {
	myFolderPath := path.Join(c.shrFolderPath, c.userName)
	myFolder, err := c.getOrCreateMyFolderInstance(myFolderPath)
	if err != nil {
		return fmt.Errorf("error when getting client folder instance: %v", err)
	}
	c.folderInstance = myFolder

	watcher, err := watcher.New(myFolderPath, func(fileName string) error {
		f, err := os.Open(fileName)
		if err != nil {
			return err
		}
		n, err := c.peer.AddFile(context.Background(), f, nil)
		if err != nil {
			return err
		}

		fileRelPath := strings.TrimPrefix(fileName, c.shrFolderPath)
		fileRelPath = strings.TrimLeft(fileRelPath, "/")
		newFile := file{ID: core.NewInstanceID(), FileRelativePath: fileRelPath, CID: n.Cid().String(), Files: []file{}}
		c.folderInstance.Files = append(c.folderInstance.Files, newFile)
		return c.collection.Save(util.JSONFromInstance(c.folderInstance))
	})

	if err != nil {
		return fmt.Errorf("error when creating folder watcher: %v", err)
	}
	watcher.Watch()
	go func() {
		<-c.closeCh
		watcher.Close()
		c.wg.Done()
	}()
	return nil
}

func (c *client) ensureFiles() error {
	res, err := c.getDirectoryTree()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	for _, userFolder := range res {
		for _, f := range userFolder.Files {
			if err := c.ensureCID(c.fullPath(f), f.CID); err != nil {
				log.Errorf("%s: error ensuring file %s: %v", c.userName, c.fullPath(f), err)
			}
		}
	}
	wg.Wait()
	return nil
}

func (c *client) ensureCID(fullPath, cidStr string) error {
	cid, err := cid.Decode(cidStr)
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
	str, err := c.peer.GetFile(ctx, cid)
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

func (c *client) getOrCreateMyFolderInstance(path string) (*userFolder, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err = os.MkdirAll(path, os.ModePerm); err != nil {
			return nil, err
		}
	}

	res, err := c.collection.Find(db.Where("Owner").Eq(c.userName))
	if err != nil {
		return nil, err
	}

	folders := make([]*userFolder, len(res))
	for i, item := range res {
		folder := &userFolder{}
		if err := json.Unmarshal(item, folder); err != nil {
			return nil, err
		}
		folders[i] = folder
	}

	var myFolder *userFolder
	if len(folders) == 0 {
		ownFolder := &userFolder{ID: core.NewInstanceID(), Owner: c.userName, Files: []file{}}
		if _, err := c.collection.Create(util.JSONFromInstance(ownFolder)); err != nil {
			return nil, err
		}
		myFolder = ownFolder
	} else {
		myFolder = folders[0]
	}

	return myFolder, nil
}
