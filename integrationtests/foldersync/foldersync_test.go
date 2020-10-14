package foldersync

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"sort"
	"strings"
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
	_ = logging.SetLogLevel("foldersync", "info")
	// _ = logging.SetLogLevel("db", "debug")
	// _ = logging.SetLogLevel("threads", "debug")
	// _ = logging.SetLogLevel("threadstore", "debug")
	os.Exit(m.Run())
}

func TestSimple(t *testing.T) {
	if os.Getenv("SKIP_FOLDERSYNC") != "" {
		t.Skip("Skipping foldersync tests")
	}

	id := thread.NewIDV1(thread.Raw, 32)

	// db0

	repoPath0, err := ioutil.TempDir("", "")
	checkErr(t, err)

	network0, err := newNetwork(repoPath0)
	checkErr(t, err)

	db0, err := db.NewDB(context.Background(), network0, id, db.WithNewRepoPath(repoPath0), db.WithNewCollections(cc))
	checkErr(t, err)
	defer db0.Close()

	c0 := db0.GetCollection(collectionName)

	info0, err := db0.GetDBInfo()
	checkErr(t, err)

	// db1

	repoPath1, err := ioutil.TempDir("", "")
	checkErr(t, err)

	network1, err := newNetwork(repoPath1)
	checkErr(t, err)

	db1, err := db.NewDBFromAddr(context.Background(), network1, info0.Addrs[0], info0.Key, db.WithNewRepoPath(repoPath1), db.WithNewCollections(cc))
	checkErr(t, err)
	defer db1.Close()

	c1 := db1.GetCollection(collectionName)

	// db2

	repoPath2, err := ioutil.TempDir("", "")
	checkErr(t, err)

	network2, err := newNetwork(repoPath2)
	checkErr(t, err)

	db2, err := db.NewDBFromAddr(context.Background(), network2, info0.Addrs[0], info0.Key, db.WithNewRepoPath(repoPath2), db.WithNewCollections(cc))
	checkErr(t, err)
	defer db2.Close()

	c2 := db2.GetCollection(collectionName)

	// db3

	repoPath3, err := ioutil.TempDir("", "")
	checkErr(t, err)

	network3, err := newNetwork(repoPath3)
	checkErr(t, err)

	db3, err := db.NewDBFromAddr(context.Background(), network3, info0.Addrs[0], info0.Key, db.WithNewRepoPath(repoPath3), db.WithNewCollections(cc))
	checkErr(t, err)
	defer db3.Close()

	c3 := db3.GetCollection(collectionName)

	// Add some data

	folder0 := folder{ID: core.NewInstanceID(), Owner: "client0", Files: []file{}}
	folder1 := folder{ID: core.NewInstanceID(), Owner: "client1", Files: []file{}}
	folder2 := folder{ID: core.NewInstanceID(), Owner: "client2", Files: []file{}}
	folder3 := folder{ID: core.NewInstanceID(), Owner: "client3", Files: []file{}}

	_, err = c0.Create(util.JSONFromInstance(folder0))
	checkErr(t, err)
	_, err = c1.Create(util.JSONFromInstance(folder1))
	checkErr(t, err)
	_, err = c2.Create(util.JSONFromInstance(folder2))
	checkErr(t, err)
	_, err = c3.Create(util.JSONFromInstance(folder3))
	checkErr(t, err)

	time.Sleep(time.Second * 15)

	instances0, err := c0.Find(nil)
	checkErr(t, err)
	if len(instances0) != 4 {
		t.Fatalf("expected 4 instances, but got %v", len(instances0))
	}

	instances1, err := c1.Find(nil)
	checkErr(t, err)
	if len(instances1) != 4 {
		t.Fatalf("expected 4 instances, but got %v", len(instances1))
	}

	instances2, err := c2.Find(nil)
	checkErr(t, err)
	if len(instances2) != 4 {
		t.Fatalf("expected 4 instances, but got %v", len(instances2))
	}

	instances3, err := c3.Find(nil)
	checkErr(t, err)
	if len(instances3) != 4 {
		t.Fatalf("expected 4 instances, but got %v", len(instances3))
	}
}

func TestNUsersBootstrap(t *testing.T) {
	if os.Getenv("SKIP_FOLDERSYNC") != "" {
		t.Skip("Skipping foldersync tests")
	}
	tests := []struct {
		totalClients     int
		totalCorePeers   int
		syncTimeout      time.Duration
		randFilesGen     int
		randFileSize     int
		checkSyncedFiles bool
	}{
		{totalClients: 2, totalCorePeers: 1, syncTimeout: time.Second * 15},
		{totalClients: 3, totalCorePeers: 1, syncTimeout: time.Second * 30},

		{totalClients: 3, totalCorePeers: 2, syncTimeout: time.Second * 30},

		{totalClients: 2, totalCorePeers: 1, syncTimeout: time.Second * 20, randFilesGen: 4, randFileSize: 10},
		{totalClients: 3, totalCorePeers: 2, syncTimeout: time.Second * 30, randFilesGen: 4, randFileSize: 10},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("Total%dCore%d", tt.totalClients, tt.totalCorePeers), func(t *testing.T) {
			var clients []*client

			client0, clean0 := createRootClient(t, fmt.Sprintf("client%d", 0))
			defer clean0()
			clients = append(clients, client0)

			info, err := client0.getDBInfo()
			checkErr(t, err)

			for i := 1; i < tt.totalCorePeers; i++ {
				client, clean := createJoinerClient(t, fmt.Sprintf("client%d", i), info.Addrs[0], info.Key)
				defer clean()
				clients = append(clients, client)
			}

			for i := tt.totalCorePeers; i < tt.totalClients; i++ {
				info, err := clients[i%tt.totalCorePeers].getDBInfo()
				checkErr(t, err)

				client, clean := createJoinerClient(t, fmt.Sprintf("client%d", i), info.Addrs[0], info.Key)
				defer clean()
				clients = append(clients, client)
			}

			for i := 0; i < tt.totalClients; i++ {
				err := clients[i].start()
				checkErr(t, err)
			}

			blk := make([]byte, tt.randFileSize)
			for i := 0; i < tt.randFilesGen; i++ {
				for j, c := range clients {
					rf, err := ioutil.TempFile(path.Join(c.folderPath, c.name), fmt.Sprintf("client%d-", j))
					checkErr(t, err)
					_, err = rand.Read(blk)
					checkErr(t, err)
					_, err = rf.Write(blk)
					checkErr(t, err)
					time.Sleep(time.Millisecond * time.Duration(rand.Intn(300)))
				}
			}

			time.Sleep(tt.syncTimeout)
			assertClientsEqualTrees(t, clients)
		})
	}
}

func assertClientsEqualTrees(t *testing.T, clients []*client) {
	totalClients := len(clients)
	dtrees := make([]clientFolders, totalClients)
	for i := range clients {
		folders, err := clients[i].getDirectoryTree()
		checkErr(t, err)
		dtrees[i] = clientFolders{client: clients[i], folders: folders}
	}
	if !EqualTrees(totalClients, dtrees...) {
		for i := range dtrees {
			printTree(i, dtrees[i].folders)
		}
		t.Fatalf("trees from clients aren't equal")
	}
}

type clientFolders struct {
	client  *client
	folders []*folder
}

func printTree(i int, folders []*folder) {
	sort.Slice(folders, func(i, j int) bool {
		return strings.Compare(folders[i].Owner, folders[j].Owner) < 0
	})

	fmt.Printf("Tree of client %d\n", i)
	for _, sf := range folders {
		fmt.Printf("\t%s %s\n", sf.ID, sf.Owner)
		for _, f := range sf.Files {
			fmt.Printf("\t\t %s %s\n", f.FileRelativePath, f.CID)
		}
	}
	fmt.Println()
}

func EqualTrees(numClients int, trees ...clientFolders) bool {
	base := trees[0]
	if len(base.folders) != numClients {
		return false
	}
	for i := 1; i < len(trees); i++ {
		if len(base.folders) != len(trees[i].folders) {
			return false
		}
		for _, baseFolder := range base.folders {
			for _, targetFolder := range trees[i].folders {
				if targetFolder.ID == baseFolder.ID && targetFolder.Owner == baseFolder.Owner {
					if !EqualFileList(base.client, baseFolder.Files, trees[i].client, targetFolder.Files) {
						return false
					}
				}
			}
		}
	}
	return true
}

func EqualFileList(c1 *client, f1s []file, c2 *client, f2s []file) bool {
	if len(f1s) != len(f2s) {
		return false
	}
	for _, f := range f1s {
		exist := false
		for _, f2 := range f2s {
			if f.ID == f2.ID {
				if !EqualFiles(c1, f, c2, f2) {
					return false
				}
				exist = true
				break
			}
		}
		if !exist {
			return false
		}
	}
	return true
}

func EqualFiles(c1 *client, f1 file, c2 *client, f2 file) bool {
	if f1.FileRelativePath != f2.FileRelativePath || f1.IsDirectory != f2.IsDirectory ||
		f1.CID != f2.CID || len(f1.Files) != len(f2.Files) {
		return false
	}

	if f1.IsDirectory {
		for _, ff := range f1.Files {
			exist := false
			for _, ff2 := range f2.Files {
				if ff.ID == ff2.ID {
					if !EqualFiles(c1, ff, c2, ff2) {
						return false
					}
					exist = true
					break
				}
			}
			if !exist {
				return false
			}
		}
	}
	return true
}

func createRootClient(t *testing.T, name string) (*client, func()) {
	repoPath, err := ioutil.TempDir("", "")
	checkErr(t, err)
	folderPath, err := ioutil.TempDir("", "")
	checkErr(t, err)
	client, err := newRootClient(name, folderPath, repoPath)
	checkErr(t, err)
	return client, func() {
		fmt.Printf("Closing root client %v\n", client.name)
		err := client.close()
		checkErr(t, err)
		os.RemoveAll(repoPath)
		os.RemoveAll(folderPath)
		fmt.Printf("Root client %v closed\n", client.name)
	}
}

func createJoinerClient(t *testing.T, name string, addr ma.Multiaddr, key thread.Key) (*client, func()) {
	repoPath, err := ioutil.TempDir("", "")
	checkErr(t, err)
	folderPath, err := ioutil.TempDir("", "")
	checkErr(t, err)
	client, err := newJoinerClient(name, folderPath, repoPath, addr, key)
	checkErr(t, err)
	return client, func() {
		fmt.Printf("Closing joiner client %v\n", client.name)
		err := client.close()
		checkErr(t, err)
		os.RemoveAll(repoPath)
		os.RemoveAll(folderPath)
		fmt.Printf("Joiner client %v closed\n", client.name)
	}
}

func checkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
