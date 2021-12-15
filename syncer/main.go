package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/namsral/flag"
	dbc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	"google.golang.org/grpc"
)

func main() {
	fs := flag.NewFlagSetWithEnvPrefix(os.Args[0], "SYNCER", 0)
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	client, err := dbc.NewClient("0.0.0.0:6006", grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}

	id := thread.NewIDV1(thread.Raw, 32)
	if err := client.NewDB(context.Background(), id); err != nil {
		log.Fatal(err)
	}
	info, err := client.GetDBInfo(context.Background(), id)
	if err != nil {
		log.Fatal(err)
	}

	// t.Run("test new db with missing read key", func(t *testing.T) {
	// 	if err = client2.NewDBFromAddr(
	// 		context.Background(),
	// 		info.Addrs[0],
	// 		thread.NewServiceKey(info.Key.Service()),
	// 	); err == nil || !strings.Contains(err.Error(), db.ErrThreadReadKeyRequired.Error()) {
	// 		t.Fatal("new db from addr without read key should fail")
	// 	}
	// })
	//
	// t.Run("test new db from address", func(t *testing.T) {
	// 	if err = client2.NewDBFromAddr(context.Background(), info.Addrs[0], info.Key); err != nil {
	// 		t.Fatalf("failed to create new db from address: %v", err)
	// 	}
	// })

	fmt.Printf("%v\n", info)

	handleInterrupt(func() {
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
