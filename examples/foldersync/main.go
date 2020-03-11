package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	logging "github.com/ipfs/go-log"
)

var (
	log = logging.Logger("main")
)

func main() {
	name := flag.String("name", "guest", "name of the user")
	sharedFolderPath := flag.String("folder", "sharedFolder", "path of the shared folder")
	inviteLink := flag.String("inviteLink", "", "thread addr to join a shared folder")
	debug := flag.Bool("debug", true, "debug mode")
	repoPath := flag.String("repo", "repo", "path of the db repo")
	flag.Parse()

	if *debug {
		logging.SetAllLoggers(1)
		logging.SetLogLevel("main", "debug")
		logging.SetLogLevel("watcher", "debug")
	}

	client, err := NewClient(*name, *sharedFolderPath, *repoPath)
	if err != nil {
		log.Fatalf("error when creating the client: %v", err)
	}

	log.Info("Starting client...")
	if *inviteLink == "" {
		err := client.Start()
		if err != nil {
			log.Fatalf("error when starting peer without invitation: %v", err)
		}
		invLinks, err := client.InviteLinks()
		if err != nil {
			log.Fatalf("error when generating invitation link: %v", err)
		}
		for i := range invLinks {
			log.Infof("Invitation link: %s", invLinks[i])
		}
	} else {
		if err := client.StartFromInvitation(*inviteLink); err != nil {
			log.Fatalf("error when starting peer from invitation: %v", err)
		}
	}
	log.Infof("Client started!")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Info("Closing...")
	if err = client.Close(); err != nil {
		log.Fatalf("error when closing the client: %v", err)
	}
}
