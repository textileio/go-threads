package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/go-threads/util"
)

func TestMain(m *testing.M) {
	logging.SetLogLevel("main", "info")
	// logging.SetLogLevel("store", "debug")
	// logging.SetLogLevel("threads", "debug")
	// logging.SetLogLevel("threadstore", "debug")
	os.Exit(m.Run())
}

func TestIt(t *testing.T) {
	id := thread.NewIDV1(thread.Raw, 32)
	creds := thread.NewDefaultCreds(id)

	// db0

	repoPath0, err := ioutil.TempDir("", "")
	checkErr(t, err)

	network0, err := newNetwork(repoPath0)
	checkErr(t, err)

	db0, err := db.NewDB(context.Background(), network0, creds, db.WithRepoPath(repoPath0), db.WithCollections(cc))
	checkErr(t, err)
	defer db0.Close()

	c0 := db0.GetCollection(collectionName)

	_, addrs0, key0, err := db0.GetInviteInfo()
	checkErr(t, err)

	// db1

	repoPath1, err := ioutil.TempDir("", "")
	checkErr(t, err)

	network1, err := newNetwork(repoPath1)
	checkErr(t, err)

	db1, err := db.NewDBFromAddr(context.Background(), network1, creds, addrs0[0], key0, db.WithRepoPath(repoPath1), db.WithCollections(cc))
	checkErr(t, err)
	defer db1.Close()

	c1 := db1.GetCollection(collectionName)

	// db2

	repoPath2, err := ioutil.TempDir("", "")
	checkErr(t, err)

	network2, err := newNetwork(repoPath2)
	checkErr(t, err)

	db2, err := db.NewDBFromAddr(context.Background(), network2, creds, addrs0[0], key0, db.WithRepoPath(repoPath2), db.WithCollections(cc))
	checkErr(t, err)
	defer db2.Close()

	c2 := db2.GetCollection(collectionName)

	// db3

	repoPath3, err := ioutil.TempDir("", "")
	checkErr(t, err)

	network3, err := newNetwork(repoPath3)
	checkErr(t, err)

	db3, err := db.NewDBFromAddr(context.Background(), network3, creds, addrs0[0], key0, db.WithRepoPath(repoPath3), db.WithCollections(cc))
	checkErr(t, err)
	defer db3.Close()

	c3 := db3.GetCollection(collectionName)

	// Add some data

	folder0 := userFolder{ID: core.NewInstanceID(), Owner: "user0", Files: []file{}}
	folder1 := userFolder{ID: core.NewInstanceID(), Owner: "user1", Files: []file{}}
	folder2 := userFolder{ID: core.NewInstanceID(), Owner: "user2", Files: []file{}}
	folder3 := userFolder{ID: core.NewInstanceID(), Owner: "user3", Files: []file{}}

	_, err = c0.Create(util.JSONFromInstance(folder0))
	checkErr(t, err)
	_, err = c1.Create(util.JSONFromInstance(folder1))
	checkErr(t, err)
	_, err = c2.Create(util.JSONFromInstance(folder2))
	checkErr(t, err)
	_, err = c3.Create(util.JSONFromInstance(folder3))
	checkErr(t, err)

	time.Sleep(time.Second * 5)
}

func TestThat(t *testing.T) {
	tests := []struct {
		totalClients     int
		totalCorePeers   int
		syncTimeout      time.Duration
		randFilesGen     int
		randFileSize     int
		checkSyncedFiles bool
	}{
		// {totalClients: 2, totalCorePeers: 1, syncTimeout: time.Second * 5},
		{totalClients: 5, totalCorePeers: 1, syncTimeout: time.Second * 15},

		// {totalClients: 5, totalCorePeers: 2, syncTimeout: time.Second * 20},

		// {totalClients: 2, totalCorePeers: 1, syncTimeout: time.Second * 10, randFilesGen: 4, randFileSize: 10},
		// {totalClients: 5, totalCorePeers: 4, syncTimeout: time.Second * 20, randFilesGen: 4, randFileSize: 10},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("Total%dCore%d", tt.totalClients, tt.totalCorePeers), func(t *testing.T) {
			t.Parallel()
			var users []*user

			user0, clean0 := createRootUser(t, fmt.Sprintf("user%d", 0))
			defer clean0()
			users = append(users, user0)

			tid, addr, key, err := user0.getInviteInfo()
			checkErr(t, err)

			for i := 1; i < tt.totalCorePeers; i++ {
				user, clean := createJoinerUser(t, fmt.Sprintf("user%d", i), tid, addr, key)
				defer clean()
				users = append(users, user)
			}

			for i := tt.totalCorePeers; i < tt.totalClients; i++ {
				tid, addr, key, err := users[i%tt.totalCorePeers].getInviteInfo()
				checkErr(t, err)

				user, clean := createJoinerUser(t, fmt.Sprintf("user%d", i), tid, addr, key)
				defer clean()
				users = append(users, user)
			}

			for i := 0; i < tt.totalClients; i++ {
				createFolder := false
				if i == 0 || i == 3 || i == 2 {
					createFolder = true
				}
				err := users[i].start(createFolder)
				checkErr(t, err)
			}

			time.Sleep(tt.syncTimeout)
		})
	}
}

func createRootUser(t *testing.T, name string) (*user, func()) {
	repoPath, err := ioutil.TempDir("", "")
	checkErr(t, err)
	folderPath, err := ioutil.TempDir("", "")
	checkErr(t, err)
	user, err := newRootUser(name, folderPath, repoPath)
	checkErr(t, err)
	return user, func() {
		fmt.Printf("Closing root user %v\n", user.name)
		err := user.close()
		checkErr(t, err)
		os.RemoveAll(repoPath)
		fmt.Printf("Root user %v closed\n", user.name)
	}
}

func createJoinerUser(t *testing.T, name string, threadID thread.ID, addr ma.Multiaddr, key thread.Key) (*user, func()) {
	repoPath, err := ioutil.TempDir("", "")
	checkErr(t, err)
	folderPath, err := ioutil.TempDir("", "")
	checkErr(t, err)
	user, err := newJoinerUser(name, folderPath, repoPath, threadID, addr, key)
	checkErr(t, err)
	return user, func() {
		fmt.Printf("Closing joiner user %v\n", user.name)
		err := user.close()
		checkErr(t, err)
		os.RemoveAll(repoPath)
		fmt.Printf("Joiner user %v closed\n", user.name)
	}
}

func checkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
