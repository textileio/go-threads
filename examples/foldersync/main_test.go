package main

import (
	"bytes"
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
)

const singleUserName = "userSingle"

func TestMain(m *testing.M) {
	logging.SetLogLevel("main", "info")
	// logging.SetLogLevel("store", "debug")
	// logging.SetLogLevel("threads", "debug")
	// logging.SetLogLevel("threadstore", "debug")
	os.Exit(m.Run())
}

func TestSingleUser(t *testing.T) {
	t.Parallel()
	c1, clean1 := createClient(t, singleUserName, "")
	defer clean1()
	defer c1.close()
	err := c1.start()
	checkErr(t, err)
	invlinks, err := c1.inviteLinks()
	checkErr(t, err)
	invlink := invlinks[0]

	if invlink == "" {
		t.Fatalf("invite link can't be empty")
	}

	trees, err := c1.getDirectoryTree()
	checkErr(t, err)
	if len(trees) != 1 {
		t.Fatalf("there should be one user folder")
	}
	tree := trees[0]
	if tree.Owner != singleUserName || tree.ID == "" || len(tree.Files) != 0 {
		t.Fatalf("invalid initial tree")
	}

	tmpFilePath := path.Join(c1.shrFolderPath, singleUserName, "test.txt")
	f, err := os.OpenFile(tmpFilePath, os.O_RDWR|os.O_CREATE, 0660)
	checkErr(t, err)
	defer os.Remove(tmpFilePath)
	_, err = f.Write([]byte("This is some content for the file"))
	checkErr(t, err)
	checkErr(t, f.Close())

	time.Sleep(time.Second)
	trees, err = c1.getDirectoryTree()
	checkErr(t, err)
	if len(trees) != 1 {
		t.Fatalf("there should be one user folder")
	}
	tree = trees[0]
	if len(tree.Files) != 1 || tree.Files[0].FileRelativePath != path.Join(singleUserName, "test.txt") {
		t.Fatalf("invalid tree state")
	}
}

func TestNUsersBootstrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		totalClients     int
		totalCorePeers   int
		syncTimeout      time.Duration
		randFilesGen     int
		randFileSize     int
		checkSyncedFiles bool
	}{
		{totalClients: 2, totalCorePeers: 1, syncTimeout: time.Second * 5},
		{totalClients: 5, totalCorePeers: 1, syncTimeout: time.Second * 15},

		{totalClients: 5, totalCorePeers: 2, syncTimeout: time.Second * 20},

		{totalClients: 2, totalCorePeers: 1, syncTimeout: time.Second * 10, randFilesGen: 4, randFileSize: 10},
		{totalClients: 5, totalCorePeers: 4, syncTimeout: time.Second * 20, randFilesGen: 4, randFileSize: 10},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("Total%dCore%d", tt.totalClients, tt.totalCorePeers), func(t *testing.T) {
			t.Parallel()
			clients := make([]*client, tt.totalClients)

			client0, clean0 := createClient(t, "user0", "")
			defer clean0()
			clients[0] = client0
			invlink0s, err := clients[0].inviteLinks()
			checkErr(t, err)
			invlink0 := invlink0s[0]

			for i := 1; i < tt.totalCorePeers; i++ {
				client, clean := createClient(t, fmt.Sprintf("user%d", i), invlink0)
				defer clean()
				clients[i] = client
			}

			for i := tt.totalCorePeers; i < tt.totalClients; i++ {
				rotatedInvLinks, err := clients[i%tt.totalCorePeers].inviteLinks()
				checkErr(t, err)
				rotatedInvLink := rotatedInvLinks[0]
				client, clean := createClient(t, fmt.Sprintf("user%d", i), rotatedInvLink)
				defer clean()
				clients[i] = client
			}

			// start them all
			for i := 0; i < tt.totalClients; i++ {
				checkErr(t, clients[i].start())
			}

			blk := make([]byte, tt.randFileSize)
			for i := 0; i < tt.randFilesGen; i++ {
				for j, c := range clients {
					rf, err := ioutil.TempFile(path.Join(c.shrFolderPath, c.userName), fmt.Sprintf("user%d-", j))
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
	dtrees := make([]clientUserFolders, totalClients)
	for i := range clients {
		userFolders, err := clients[i].getDirectoryTree()
		checkErr(t, err)
		dtrees[i] = clientUserFolders{client: clients[i], userFolders: userFolders}
	}
	if !EqualTrees(totalClients, dtrees...) {
		for i := range dtrees {
			printTree(i, dtrees[i].userFolders)
		}
		t.Fatalf("trees from users aren't equal")
	}
}

type clientUserFolders struct {
	client      *client
	userFolders []*userFolder
}

func printTree(i int, folders []*userFolder) {
	sort.Slice(folders, func(i, j int) bool {
		return strings.Compare(folders[i].Owner, folders[j].Owner) < 0
	})

	fmt.Printf("Tree of user %d\n", i)
	for _, sf := range folders {
		fmt.Printf("\t%s %s\n", sf.ID, sf.Owner)
		for _, f := range sf.Files {
			fmt.Printf("\t\t %s %s\n", f.FileRelativePath, f.CID)
		}
	}
	fmt.Println()
}

func EqualTrees(numUsers int, trees ...clientUserFolders) bool {
	base := trees[0]
	if len(base.userFolders) != numUsers {
		return false
	}
	for i := 1; i < len(trees); i++ {
		if len(base.userFolders) != len(trees[i].userFolders) {
			return false
		}
		for _, baseUserFolder := range base.userFolders {
			for _, targetUserFolder := range trees[i].userFolders {
				if targetUserFolder.ID == baseUserFolder.ID && targetUserFolder.Owner == baseUserFolder.Owner {
					if !EqualFileList(base.client, baseUserFolder.Files, trees[i].client, targetUserFolder.Files) {
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

	if !f1.IsDirectory {
		f1FullPath := c1.fullPath(f1)
		f2FullPath := c2.fullPath(f2)
		if _, err := os.Stat(f1FullPath); err != nil {
			return false
		}
		if _, err := os.Stat(f2FullPath); err != nil {
			return false
		}
		r, err := os.Open(f1FullPath)
		if err != nil {
			panic(err)
		}
		defer r.Close()
		r2, err := os.Open(f2FullPath)
		if err != nil {
			panic(err)
		}
		defer r.Close()
		b1, err := ioutil.ReadAll(r)
		if err != nil {
			panic(err)
		}
		b2, err := ioutil.ReadAll(r2)
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(b1, b2) {
			return false
		}
	} else {
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

func createClient(t *testing.T, name, inviteLink string) (*client, func()) {
	shrFolder, err := ioutil.TempDir("", "")
	checkErr(t, err)
	repoPath, err := ioutil.TempDir("", "")
	checkErr(t, err)
	client, err := newClient(name, shrFolder, repoPath, inviteLink)
	checkErr(t, err)
	return client, func() {
		fmt.Printf("Closing client %v\n", name)
		err := client.close()
		checkErr(t, err)
		fmt.Printf("Client closed %v\n", name)
		os.RemoveAll(shrFolder)
		os.RemoveAll(repoPath)
	}
}

func checkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
