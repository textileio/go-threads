package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/multiformats/go-multiaddr"
	"github.com/namsral/flag"
	dbc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/go-threads/syncer/clock"
	"github.com/textileio/go-threads/util"
	"github.com/tjarratt/babble"
	"google.golang.org/grpc"
)

const dbname = "Syncer"

const collection = "Babble"

const schema = `{
		"$id": "https://example.com/person.schema.json",
		"$schema": "http://json-schema.org/draft-07/schema#",
		"title": "` + collection + `",
		"type": "object",
		"properties": {
			"_id": {
				"type": "string",
				"description": "The instance's id."
			},
			"Words": {
				"type": "string",
				"description": "Random words."
			}
		}
	}`

type Babble struct {
	ID    string `json:"_id"`
	Words string `json:"words,omitempty"`
}

type arrayFlags []string

func (i *arrayFlags) String() string {
	return "my string representation"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var babbler = babble.NewBabbler()

func main() {
	fs := flag.NewFlagSetWithEnvPrefix(os.Args[0], strings.ToUpper(dbname), 0)

	var joins arrayFlags
	fs.Var(&joins, "join", "Thread to join with key, e.g., address:key")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	client, err := dbc.NewClient("0.0.0.0:6006", grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}

	threads := make(map[thread.ID]db.Info)

	dbs, err := client.ListDBs(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, i := range dbs {
		if i.Name == dbname {
			id, err := thread.FromAddr(i.Addrs[0])
			if err != nil {
				log.Fatal(err)
			}
			j, err := json.MarshalIndent(threadInfo(i), "", "  ")
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("using thread:\n")
			fmt.Printf("%s\n", j)
			threads[id] = i
		}
	}

	if len(threads) == 0 {
		id := thread.NewIDV1(thread.Raw, 32)
		opts := []db.NewManagedOption{
			db.WithNewManagedName("Syncer"),
			db.WithNewManagedCollections(
				db.CollectionConfig{
					Name:   collection,
					Schema: util.SchemaFromSchemaString(schema),
				},
			),
		}
		if err := client.NewDB(context.Background(), id, opts...); err != nil {
			log.Fatal(err)
		}
		info, err := client.GetDBInfo(context.Background(), id)
		if err != nil {
			log.Fatal(err)
		}
		j, err := json.MarshalIndent(threadInfo(info), "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("created thread:\n")
		fmt.Printf("%s\n", j)
		threads[id] = info
	}

	for _, j := range joins {
		parts := strings.Split(j, ":")
		if len(parts) != 2 {
			log.Fatal("invalid join arg; must be address:key format")
		}
		var addrs []multiaddr.Multiaddr
		a, err := multiaddr.NewMultiaddr(parts[0])
		if err != nil {
			log.Fatal(err)
		}
		addrs = append(addrs, a)
		id, err := thread.FromAddr(addrs[0])
		if err != nil {
			log.Fatal(err)
		}
		key, err := thread.KeyFromString(parts[1])
		if err != nil {
			log.Fatal(err)
		}
		opts := []db.NewManagedOption{
			db.WithNewManagedName("Syncer"),
			db.WithNewManagedCollections(
				db.CollectionConfig{
					Name:   collection,
					Schema: util.SchemaFromSchemaString(schema),
				},
			),
		}
		if err := client.NewDBFromAddr(context.Background(), addrs[0], key, opts...); err != nil {
			log.Fatal(err)
		}
		info, err := client.GetDBInfo(context.Background(), id)
		if err != nil {
			log.Fatal(err)
		}
		j, err := json.MarshalIndent(threadInfo(info), "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("joined thread:\n")
		fmt.Printf("%s\n", j)
		threads[id] = info
	}

	ctx, cancel := context.WithCancel(context.Background())
	var clocks []*clock.RandomTicker
	for id := range threads {
		c := clock.NewRandomTicker(time.Millisecond*100, time.Second*3)
		clocks = append(clocks, c)
		go func(id thread.ID, c *clock.RandomTicker) {
			events, err := client.Listen(ctx, id, []dbc.ListenOption{{
				Type: dbc.ListenAll,
			}})
			if err != nil {
				log.Fatal(err)
			}

			for {
				select {
				case _, ok := <-c.C:
					if !ok {
						return
					}
					b := newBabble()
					_, err := client.Create(ctx, id, collection, dbc.Instances{b})
					if err != nil {
						log.Fatal(err)
					}
					// fmt.Printf("babbled: %s %s\n", ids[0], b.Words)
				case e := <-events:
					if e.Err != nil {
						fmt.Printf("got error: %s n", e.Err)
					} else if e.Action.Instance != nil {
						var b Babble
						if err := json.Unmarshal(e.Action.Instance, &b); err != nil {
							log.Fatal(err)
						}
						fmt.Printf("babble: %s %s\n", b.ID, b.Words)
					}
				}
			}
		}(id, c)
	}

	handleInterrupt(func() {
		cancel()
		for _, c := range clocks {
			c.Stop()
		}
		if err := client.Close(); err != nil {
			log.Fatal(err)
		}
	})
}

func handleInterrupt(stop func()) {
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	fmt.Println("Gracefully stopping... (press Ctrl+C again to force)")
	stop()
	os.Exit(1)
}

func threadInfo(info db.Info) interface{} {
	return struct {
		Name  string
		Addrs []multiaddr.Multiaddr
		Key   string
	}{
		Name:  info.Name,
		Addrs: info.Addrs,
		Key:   info.Key.String(),
	}
}

func newBabble() *Babble {
	return &Babble{
		Words: babbler.Babble(),
	}
}
