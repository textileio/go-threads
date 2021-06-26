package migration

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/textileio/go-threads/common"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/util"
)

const DefaultRepoPath = "generated"
const ThreadIdsFilename = "thread_ids"
const NumRecords = 10

func TestMigration(t *testing.T) {
	// reading environment
	repoPath := os.Getenv("ENV_REPO_PATH")
	if repoPath == "" {
		repoPath = DefaultRepoPath
	}
	numRecords, err := getEnvInt("ENV_NUM_RECORDS")
	if err != nil {
		numRecords = NumRecords
	}
	fileName := os.Getenv("ENV_FILENAME")
	if fileName == "" {
		fileName = ThreadIdsFilename
	}
	net, err := newNetwork(repoPath)
	if err != nil {
		t.Fatalf("error in creating network")
	}

	// reading all thread ids from file
	file, err := os.Open(repoPath + "/" + fileName)
	if err != nil {
		t.Fatalf("error in opening file")
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var line string
	var threadIds []thread.ID
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			break
		}
		line = line[:len(line)-1]
		id, err := thread.Decode(line)
		if err != nil {
			t.Fatalf("error while decoding string %s", err)
		}
		threadIds = append(threadIds, id)
	}
	if err != io.EOF {
		t.Fatalf("got error while reading from file")
	}

	ctx := context.Background()
	for _, id := range threadIds {
		info, err := net.GetThread(ctx, id)
		if err != nil {
			t.Fatalf("could not get thread with id: %s", err)
		}

		for _, log := range info.Logs {
			if log.Head.Counter != int64(numRecords) {
				t.Errorf("incorrect number of records, expected %d, but got %d", numRecords, log.Head.Counter)
			}
		}
	}
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
