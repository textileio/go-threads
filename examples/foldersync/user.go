package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	ipfslite "github.com/hsanjuan/ipfs-lite"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/phayes/freeport"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/go-threads/examples/foldersync/watcher"
	"github.com/textileio/go-threads/util"
)

var (
	collectionName = "sharedFolder"

	cc = db.CollectionConfig{
		Name:   collectionName,
		Schema: util.SchemaFromInstance(userFolder{}, false),
	}
)

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

type user struct {
	sync.Mutex
	name           string
	folderPath     string
	folderInstance *userFolder
	db             *db.DB
	collection     *db.Collection
	net            net.Net
	peer           *ipfslite.Peer
	closeCh        chan struct{}
}

func newRootUser(name, folderPath, repoPath string) (*user, error) {
	id := thread.NewIDV1(thread.Raw, 32)
	creds := thread.NewDefaultCreds(id)

	network, err := newNetwork(repoPath)
	if err != nil {
		return nil, err
	}

	d, err := db.NewDB(context.Background(), network, creds, db.WithRepoPath(repoPath), db.WithCollections(cc))
	if err != nil {
		return nil, err
	}

	return &user{
		name:       name,
		folderPath: folderPath,
		db:         d,
		collection: d.GetCollection(collectionName),
		net:        network,
		peer:       network.GetIpfsLite(),
		closeCh:    make(chan struct{}),
	}, nil
}

func newJoinerUser(name, folderPath, repoPath string, threadID thread.ID, addr ma.Multiaddr, key thread.Key) (*user, error) {
	creds := thread.NewDefaultCreds(threadID)

	network, err := newNetwork(repoPath)
	if err != nil {
		return nil, err
	}

	d, err := db.NewDBFromAddr(context.Background(), network, creds, addr, key, db.WithRepoPath(repoPath), db.WithCollections(cc))
	if err != nil {
		return nil, err
	}

	return &user{
		name:       name,
		folderPath: folderPath,
		db:         d,
		collection: d.GetCollection(collectionName),
		net:        network,
		peer:       network.GetIpfsLite(),
		closeCh:    make(chan struct{}),
	}, nil
}

func newNetwork(repoPath string) (db.NetBoostrapper, error) {
	hostPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, err
	}
	hostAddr := util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", hostPort))

	network, err := db.DefaultNetwork(repoPath, db.WithNetHostAddr(hostAddr))
	if err != nil {
		return nil, err
	}
	return network, nil
}

func (u *user) getInviteInfo() (thread.ID, ma.Multiaddr, thread.Key, error) {
	id, addrs, key, err := u.db.GetInviteInfo()
	if err != nil {
		return thread.ID{}, nil, thread.Key{}, err
	}
	if len(addrs) < 1 {
		return thread.ID{}, nil, thread.Key{}, errors.New("unable to get thread address")
	}
	return id, addrs[0], key, nil
}

func (u *user) getOrCreateMyFolderInstance(path string, createFolder bool) (*userFolder, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err = os.MkdirAll(path, os.ModePerm); err != nil {
			return nil, err
		}
	}

	var myFolder *userFolder

	res, err := u.collection.Find(db.Where("Owner").Eq(u.name))
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		ownFolder := userFolder{ID: core.NewInstanceID(), Owner: u.name, Files: []file{}}
		if createFolder {
			json := util.JSONFromInstance(ownFolder)
			_, err := u.collection.Create(json)
			if err != nil {
				return nil, err
			}
		}
		myFolder = &ownFolder
	} else {
		ownFolder := &userFolder{}
		if err := json.Unmarshal(res[0], ownFolder); err != nil {
			return nil, err
		}
		myFolder = ownFolder
	}

	return myFolder, nil
}

func (u *user) startFSWatcher(createFolder bool) error {
	myFolderPath := path.Join(u.folderPath, u.name)
	myFolder, err := u.getOrCreateMyFolderInstance(myFolderPath, createFolder)
	if err != nil {
		return fmt.Errorf("error when getting user folder for %v: %v", u.name, err)
	}
	u.folderInstance = myFolder

	watcher, err := watcher.New(myFolderPath, func(fileName string) error {
		f, err := os.Open(fileName)
		if err != nil {
			return err
		}
		n, err := u.peer.AddFile(context.Background(), f, nil)
		if err != nil {
			return err
		}

		fileRelPath := strings.TrimPrefix(fileName, u.folderPath)
		fileRelPath = strings.TrimLeft(fileRelPath, "/")
		newFile := file{ID: core.NewInstanceID(), FileRelativePath: fileRelPath, CID: n.Cid().String(), Files: []file{}}
		u.folderInstance.Files = append(u.folderInstance.Files, newFile)
		return u.collection.Save(util.JSONFromInstance(u.folderInstance))
	})
	if err != nil {
		return fmt.Errorf("error when creating fs watcher for %v: %v", u.name, err)
	}
	watcher.Watch()

	go func() {
		<-u.closeCh
		fmt.Printf("received closeCh for %v %p\n", u.name, u)
		watcher.Close()
		// c.wg.Done()
		fmt.Printf("fs watcher shutdown for %v %p\n", u.name, u)
		return
	}()
	return nil
}

func (u *user) start(createFolder bool) error {
	fmt.Printf("entering user.start for %v %p\n", u.name, u)
	u.Lock()
	defer func() {
		fmt.Printf("exiting user.start for %v %p\n", u.name, u)
		u.Unlock()
	}()

	if err := u.startFSWatcher(createFolder); err != nil {
		return err
	}

	return nil
}

func (u *user) close() error {
	fmt.Printf("entering user.close for %v %p\n", u.name, u)
	u.Lock()
	defer func() {
		fmt.Printf("exiting user.close for %v %p\n", u.name, u)
		u.Unlock()
	}()

	close(u.closeCh)

	if err := u.db.Close(); err != nil {
		return err
	}
	if err := u.net.Close(); err != nil {
		return err
	}
	return nil
}
