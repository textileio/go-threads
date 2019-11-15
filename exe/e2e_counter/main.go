package main

import (
	"flag"
	"os"

	"github.com/mitchellh/go-homedir"
	"github.com/textileio/go-textile-threads/util"
)

func main() {
	writer := flag.Bool("writer", false, "peer role")
	repo := flag.String("repo", ".thread", "repo path for the store")
	port := flag.Int("port", 5000, "porto to listen to")
	clean := flag.Bool("clean", true, "deletes any previous state. a fresh start")
	flag.Parse()

	if *clean {
		repop, err := homedir.Expand(*repo)
		if err != nil {
			panic(err)
		}
		if err = os.RemoveAll(repop); err != nil {
			panic(err)
		}
	}

	util.SetupDefaultLoggingConfig(*repo)
	// Run different things depending on flag
	if *writer {
		runWriterPeer(*repo, *port)
	} else {
		runReaderPeer(*repo, *port)
	}
}
