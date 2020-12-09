package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	logging "github.com/ipfs/go-log/v2"
	"github.com/namsral/flag"
	badger "github.com/textileio/go-ds-badger"
	mongods "github.com/textileio/go-ds-mongo"
)

var log = logging.Logger("dscopy")

type store struct {
	name  string
	files []string
}

var stores = []store{
	{
		name: "eventstore",
	},
	{
		name: "logstore",
	},
	{
		name:  "ipfslite",
		files: []string{"key"},
	},
}

func main() {
	fs := flag.NewFlagSet(os.Args[0], 0)

	fromBadgerRepos := fs.String("fromBadgerRepos", "", "Source badger repos path")
	toBadgerRepos := fs.String("toBadgerRepos", "", "Destination badger repos path")

	fromMongoUri := fs.String("fromMongoUri", "", "Source MongoDB URI")
	fromMongoDatabase := fs.String("fromMongoDatabase", "", "Source MongoDB database")
	toMongoUri := fs.String("toMongoUri", "", "Destination MongoDB URI")
	toMongoDatabase := fs.String("toMongoDatabase", "", "Destination MongoDB database")

	parallel := fs.Int("parallel", 1000, "Number of parallel copy operations")

	verbose := fs.Bool("verbose", false, "More verbose output")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	logging.SetupLogging(logging.Config{
		Format: logging.ColorizedOutput,
		Stderr: true,
		Level:  logging.LevelError,
	})
	if err := logging.SetLogLevel("dscopy", "info"); err != nil {
		log.Fatal(err)
	}

	start := time.Now()

	for _, s := range stores {
		if err := copyDatastore(
			s,
			*fromBadgerRepos,
			*toBadgerRepos,
			*fromMongoUri,
			*toMongoUri,
			*fromMongoDatabase,
			*toMongoDatabase,
			*parallel,
			*verbose,
		); err != nil {
			log.Fatal(err)
		}
	}

	log.Infof("done in %s", time.Since(start))
}

func copyDatastore(
	s store,
	fromBadgerRepos, toBadgerRepos string,
	fromMongoUri, toMongoUri string,
	fromMongoDatabase, toMongoDatabase string,
	parallel int,
	verbose bool,
) error {
	if len(fromBadgerRepos) != 0 && len(fromMongoUri) != 0 {
		return fmt.Errorf("multiple sources specified")
	}
	if len(fromBadgerRepos) == 0 && len(fromMongoUri) == 0 {
		return fmt.Errorf("source not specified")
	}
	if len(toBadgerRepos) != 0 && len(toMongoUri) != 0 {
		return fmt.Errorf("multiple destinations specified")
	}
	if len(toBadgerRepos) == 0 && len(toMongoUri) == 0 {
		return fmt.Errorf("destination not specified")
	}

	var from, to ds.Datastore
	var err error
	if len(fromBadgerRepos) != 0 {
		path := filepath.Join(fromBadgerRepos, s.name)
		from, err = badger.NewDatastore(path, &badger.DefaultOptions)
		if err != nil {
			return fmt.Errorf("connecting to badger source: %v", err)
		}
		log.Infof("connected to badger source: %s", path)
	}
	if len(toBadgerRepos) != 0 {
		path := filepath.Join(toBadgerRepos, s.name)
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return fmt.Errorf("making destination path: %v", err)
		}
		to, err = badger.NewDatastore(path, &badger.DefaultOptions)
		if err != nil {
			return fmt.Errorf("connecting to badger destination: %v", err)
		}
		log.Infof("connected to badger destination: %s", path)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if len(fromMongoUri) != 0 {
		uri, err := url.Parse(fromMongoUri)
		if err != nil {
			return fmt.Errorf("parsing source mongo URI: %v", err)
		}
		if len(fromMongoDatabase) == 0 {
			return fmt.Errorf("source mongo database not specified")
		}
		from, err = mongods.New(ctx, fromMongoUri, fromMongoDatabase, mongods.WithCollName(s.name))
		if err != nil {
			return fmt.Errorf("connecting to mongo source: %v", err)
		}
		log.Infof("connected to mongo source: %s", uri.Redacted())
	}
	if len(toMongoUri) != 0 {
		uri, err := url.Parse(toMongoUri)
		if err != nil {
			return fmt.Errorf("parsing destination mongo URI: %v", err)
		}
		if len(toMongoDatabase) == 0 {
			return fmt.Errorf("destination mongo database not specified")
		}
		to, err = mongods.New(ctx, toMongoUri, toMongoDatabase, mongods.WithCollName(s.name))
		if err != nil {
			return fmt.Errorf("connecting to mongo destination: %v", err)
		}
		log.Infof("connected to mongo destination: %s", uri.Redacted())
	}

	res, err := from.Query(query.Query{})
	if err != nil {
		return fmt.Errorf("querying source: %v", err)
	}
	defer res.Close()

	var lock sync.Mutex
	var errors []string
	var count int
	start := time.Now()
	lim := make(chan struct{}, parallel)
	for r := range res.Next() {
		if r.Error != nil {
			return fmt.Errorf("getting next source result: %v", r.Error)
		}
		lim <- struct{}{}

		r := r
		go func() {
			defer func() { <-lim }()

			if err := to.Put(ds.NewKey(r.Key), r.Value); err != nil {
				lock.Lock()
				errors = append(errors, fmt.Sprintf("copying %s: %v", r.Key, err))
				lock.Unlock()
				return
			}
			if verbose {
				log.Infof("copied %s", r.Key)
			}
			lock.Lock()
			count++
			if count%parallel == 0 {
				log.Infof("copied %d keys", count)
			}
			lock.Unlock()
		}()
	}
	for i := 0; i < cap(lim); i++ {
		lim <- struct{}{}
	}

	if len(errors) > 0 {
		for _, m := range errors {
			log.Error(m)
		}
		return fmt.Errorf("had %d errors", len(errors))
	}

	log.Infof("copied %d keys in %s", count, time.Since(start))

	for _, f := range s.files {
		var file []byte
		if len(fromBadgerRepos) != 0 {
			dir := filepath.Join(fromBadgerRepos, s.name)
			pth := filepath.Join(dir, f)
			_, err := os.Stat(pth)
			if os.IsNotExist(err) {
				return fmt.Errorf("loading file %s: %v", pth, err)
			} else {
				file, err = ioutil.ReadFile(pth)
				if err != nil {
					return fmt.Errorf("reading file %s: %v", pth, err)
				}
			}
		} else {
			file, err = from.Get(ds.NewKey(f))
			if err != nil {
				return fmt.Errorf("getting file %s: %v", f, err)
			}
		}

		if len(toBadgerRepos) != 0 {
			dir := filepath.Join(toBadgerRepos, s.name)
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return fmt.Errorf("making dir %s: %v", dir, err)
			}
			pth := filepath.Join(dir, f)
			if err = ioutil.WriteFile(pth, file, 0400); err != nil {
				return fmt.Errorf("writing file %s: %v", pth, err)
			}
		} else {
			if err := to.Put(ds.NewKey(f), file); err != nil {
				return fmt.Errorf("putting file %s: %v", f, err)
			}
		}

		log.Infof("copied file %s", f)
	}
	return nil
}
