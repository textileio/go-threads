package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	fuzz "github.com/google/gofuzz"
	ipldcbor "github.com/ipfs/go-ipld-cbor"
	"github.com/multiformats/go-multihash"
	"github.com/textileio/go-threads/common"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/util"
)

var fuzzer = fuzz.New()

const DefaultRepoPath = "../generated"
const ThreadIdsFilename = "thread_ids"
const NumRecords = 10
const NumThreads = 10

func main() {
	repoPath := os.Getenv("ENV_REPO_PATH")
	if repoPath == "" {
		repoPath = DefaultRepoPath
	}
	numRecords, err := getEnvInt("ENV_NUM_RECORDS")
	if err != nil {
		numRecords = NumRecords
	}
	numThreads, err := getEnvInt("ENV_NUM_THREADS")
	if err != nil {
		numThreads = NumThreads
	}
	fileName := os.Getenv("ENV_FILENAME")
	if fileName == "" {
		fileName = ThreadIdsFilename
	}

	os.RemoveAll(repoPath)
	err = genStore(repoPath, fileName, numThreads, numRecords)
	if err != nil {
		fmt.Println(err)
	}
}

func genStore(repoPath string, fileName string, numThreads int, numRecords int) error {
	net, err := newNetwork(repoPath)
	if err != nil {
		return err
	}
	ctx := context.Background()

	filePath := repoPath + "/" + fileName
	file, err := os.Create(filePath)
	defer file.Close()

	if err != nil {
		return err
	}

	for i := 0; i < numThreads; i++ {
		id, err := createThreadWithRecords(ctx, net, numRecords)
		if err != nil {
			return err
		}

		_, err = file.WriteString(id.String() + "\n")
		if err != nil {
			return err
		}

		err = file.Sync()
		if err != nil {
			return err
		}
	}

	return nil
}

func createThreadWithRecords(ctx context.Context, net common.NetBoostrapper, numRecords int) (thread.ID, error) {
	id := thread.NewIDV1(thread.Raw, 32)

	_, err := net.CreateThread(ctx, id)
	if err != nil {
		return "", err
	}

	for i := 0; i < numRecords; i++ {
		obj := make(map[string][]byte)
		fuzzer.Fuzz(&obj)
		body, err := ipldcbor.WrapObject(obj, multihash.SHA2_256, -1)
		if err != nil {
			return "", err
		}

		_, err = net.CreateRecord(ctx, id, body)
		if err != nil {
			return "", err
		}
	}

	return id, nil
}

func newNetwork(repoPath string) (common.NetBoostrapper, error) {
	network, err := common.DefaultNetwork(
		common.WithNetBadgerPersistence(repoPath),
		common.WithNetHostAddr(util.FreeLocalAddr()),
		common.WithNetDebug(true),
	)
	if err != nil {
		return nil, err
	}
	return network, nil
}

func getEnvInt(key string) (int, error) {
	s := os.Getenv(key)
	if s == "" {
		return 0, fmt.Errorf("no such variable")
	}

	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return v, nil
}
