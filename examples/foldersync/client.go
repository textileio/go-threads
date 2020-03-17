package main

import (
	"context"
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
	"github.com/mr-tron/base58"
	"github.com/multiformats/go-multiaddr"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-foldersync/watcher"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
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

	ts         db.ServiceBoostrapper
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

func newClient(name, sharedFolderPath, repoPath string) (*client, error) {
	ts, err := db.DefaultService(repoPath)
	if err != nil {
		return nil, err
	}

	service, err := db.DefaultService(repoPath)
	if err != nil {
		return nil, err
	}

	threadID := thread.NewIDV1(thread.Raw, 32)

	// db.NewDBFromAddr(context.Background(), service, )

	d, err := db.NewDB(context.Background(), service, threadID)
	if err != nil {
		return nil, fmt.Errorf("error when creating db: %v", err)
	}

	c, err := d.NewCollectionFromInstance("shardFolder", &userFolder{})
	if err != nil {
		return nil, fmt.Errorf("error when creating new collection from instance: %v", err)
	}

	//ipfspeer, closeIpfsLite, err := createIPFSLite(ts.Host())
	ipfspeer := ts.GetIpfsLite()
	if err != nil {
		return nil, fmt.Errorf("error when creating ipfs lite peer: %v", err)
	}

	return &client{
		closeIpfsLite: func() error { return nil },
		closeCh:       make(chan struct{}),
		ts:            ts,
		db:            d,
		collection:    c,
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
	if err := c.ts.Close(); err != nil {
		return err
	}

	return nil
}

func (c *client) start() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.started {
		return errClientAlreadyStarted
	}

	log.Info("Starting/resuming a new thread for shared folders")
	if err := c.startInternal(); err != nil {
		return err
	}
	c.started = true
	return nil
}

func (c *client) startFromInvitation(link string) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.started {
		return errClientAlreadyStarted
	}
	addr, fk, rk := parseInviteLink(link)
	log.Infof("Starting from addr: %s", addr)
	if err := c.db.StartFromAddr(addr, fk, rk); err != nil {
		return err
	}

	if err := c.startInternal(); err != nil {
		return err
	}
	c.started = true
	return nil
}

func (c *client) getDirectoryTree() ([]*userFolder, error) {
	var res []*userFolder
	if err := c.collection.Find(&res, nil); err != nil {
		return nil, err

	}
	return res, nil
}

func (c *client) inviteLinks() ([]string, error) {
	peerAddr := c.ts.Host().Addrs()[0]
	peerID, err := multiaddr.NewComponent("p2p", c.ts.Host().ID().String())

	threadComp, err := multiaddr.NewComponent("thread", id1.String())
	addr := peerAddr.Encapsulate(peerID).Encapsulate(threadComp)

	host := c.db.Service().Host()
	tid, _, err := c.db.ThreadID()
	if err != nil {
		return nil, err
	}
	tinfo, err := c.db.Service().GetThread(context.Background(), tid)
	if err != nil {
		return nil, err
	}

	id, _ := ma.NewComponent("p2p", host.ID().String())
	thread, _ := ma.NewComponent("thread", tid.String())

	addrs := host.Addrs()
	res := make([]string, len(addrs))
	for i := range addrs {
		addr := addrs[i].Encapsulate(id).Encapsulate(thread).String()
		fKey := base58.Encode(tinfo.FollowKey.Bytes())
		rKey := base58.Encode(tinfo.ReadKey.Bytes())

		res[i] = addr + "?" + fKey + "&" + rKey
	}
	return res, nil
}

func (c *client) fullPath(f file) string {
	return filepath.Join(c.shrFolderPath, f.FileRelativePath)
}

func (c *client) startInternal() error {
	c.wg.Add(2)
	c.startFileSystemWatcher()
	c.startListeningExternalChanges()

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
				var uf userFolder
				if err := c.collection.FindByID(a.ID, &uf); err != nil {
					log.Errorf("error when getting changed user folder with ID %s", a.ID)
					continue
				}
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
		return c.collection.Save(c.folderInstance)
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

	var res []*userFolder
	if err := c.collection.Find(&res, db.Where("Owner").Eq(c.userName)); err != nil {
		return nil, err
	}

	var myFolder *userFolder
	if len(res) == 0 {
		ownFolder := &userFolder{Owner: c.userName, Files: []file{}}
		if err := c.collection.Create(ownFolder); err != nil {
			return nil, err
		}
		myFolder = ownFolder
	} else {
		myFolder = res[0]
	}

	return myFolder, nil
}
