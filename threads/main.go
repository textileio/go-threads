package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/c-bata/go-prompt"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
)

var (
	apiClient    *client.Client
	apiTimeout   = time.Second * 5
	promptPrefix = ">>> "
	noDBMessage  = "DB required. `use <db id>`"
	currentDB    thread.ID
	suggestions  = []prompt.Suggest{
		{
			Text:        "use",
			Description: "Switch the DB.",
		},
		{
			Text:        "exit",
			Description: "Exit.",
		},
	}
	dbSuggestions = []prompt.Suggest{
		{
			Text:        "listen",
			Description: "Stream all db updates.",
		},
		{
			Text:        "find",
			Description: "Find all instances by collection name.",
		},
		{
			Text:        "findByID",
			Description: "Find an instance by collection name and instance ID.",
		},
	}
)

var listenState struct {
	LivePrefix string
	IsEnable   bool
	Cancel     context.CancelFunc
}

func resetPrefix() {
	listenState.LivePrefix = promptPrefix
	listenState.IsEnable = false
	fmt.Println("done")
}

func getLivePrefix() (string, bool) {
	return listenState.LivePrefix, listenState.IsEnable
}

type any map[string]interface{}

func completer(in prompt.Document) []prompt.Suggest {
	w := in.GetWordBeforeCursor()
	if w == "" {
		return []prompt.Suggest{}
	}
	if currentDB.Defined() {
		return prompt.FilterHasPrefix(append(suggestions, dbSuggestions...), w, true)
	}
	return prompt.FilterHasPrefix(suggestions, w, true)
}

func dbExecutor(blocks []string) {
	switch blocks[0] {
	case "listen":
		listen(currentDB)
		return
	case "find":
		if len(blocks) < 2 {
			fmt.Println("You must provide a collection name.")
			return
		}
		find(currentDB, blocks[1])
		return
	case "findByID":
		if len(blocks) < 2 {
			fmt.Println("You must provide a collection name.")
			return
		}
		if len(blocks) < 3 {
			fmt.Println("You must provide an instance ID.")
			return
		}
		findByID(currentDB, blocks[1], blocks[2])
		return
	default:
		fmt.Println("Sorry, I don't understand.")
	}
}

func executor(in string) {
	blocks := trimmedBlocks(in)
	switch blocks[0] {
	case "":
		if listenState.IsEnable {
			listenState.Cancel()
		}
		return
	case "use":
		if len(blocks) < 2 {
			fmt.Println("You must provide a db ID.")
			return
		}
		if err := currentDB.Validate(); err != nil {
			fmt.Printf("Invalid thread id: %v\n", err)
			return
		}
		if currentDB.String() == blocks[1] {
			fmt.Printf("Already using %s\n", blocks[1])
			return
		}
		var err error
		currentDB, err = thread.Decode(blocks[1])
		if err != nil {
			fmt.Printf("Please enter a valid db ID")
		} else {
			fmt.Printf("Switched to %s\n", blocks[1])
		}
		return
	case "exit":
		fmt.Println("Bye!")
		os.Exit(0)
	default:
		if err := currentDB.Validate(); err != nil {
			fmt.Printf("Invalid thread id: %v\n", err)
			return
		}
		if currentDB.String() == "" {
			fmt.Println(noDBMessage)
			return
		}
		dbExecutor(blocks)
	}
}

func find(id thread.ID, collection string) {
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	rawResults, err := apiClient.Find(ctx, id, collection, &db.Query{}, &any{})
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	instances := rawResults.([]*any)
	if len(instances) == 0 {
		fmt.Println("None found")
		return
	}
	for _, el := range instances {
		prettyPrint(el)
	}
}

func findByID(id thread.ID, collection string, instanceID string) {
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	instance := &any{}
	err := apiClient.FindByID(ctx, id, collection, instanceID, instance)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	prettyPrint(instance)
}

func backgroundListen(ctx context.Context, id thread.ID) {
	channel, err := apiClient.Listen(ctx, id, []client.ListenOption{})
	if err != nil {
		fmt.Println(err.Error())
		resetPrefix()
		return
	}
	for {
		select {
		case val, ok := <-channel:
			if !ok {
				resetPrefix()
				return
			}
			if val.Err != nil {
				fmt.Println(val.Err)
				continue
			}
			obj := &any{}
			if err := json.Unmarshal(val.Action.Instance, obj); err != nil {
				fmt.Println("failed to unmarshal listen result")
				continue
			}
			prettyPrint(obj)
		case <-ctx.Done():
			resetPrefix()
			return
		}
	}
}

func listen(id thread.ID) {
	ctx, cancel := context.WithCancel(context.Background())
	listenState.LivePrefix = ""
	listenState.IsEnable = true
	listenState.Cancel = cancel
	fmt.Println("<enter> to cancel")
	go backgroundListen(ctx, id)
}

func main() {
	addr := flag.String("addr", "/ip4/0.0.0.0/tcp/6006", "Threads API address")
	flag.Parse()
	maddr, err := ma.NewMultiaddr(*addr)
	if err != nil {
		log.Fatalf("addr error: %v", err)
	}

	target, err := util.TCPAddrFromMultiAddr(maddr)
	if err != nil {
		log.Fatalf("addr error: %v", err)
	}

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithBlock())
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		log.Fatalf("connection error: %v", err)
	}
	defer conn.Close()

	apiClient, err = client.NewClient(target, opts...)
	if err != nil {
		log.Fatalf("connection error: %v", err)
	}

	fmt.Println("Successfully connected.")

	p := prompt.New(
		executor,
		completer,
		prompt.OptionTitle("Threads"),

		prompt.OptionPrefix(promptPrefix),
		prompt.OptionLivePrefix(getLivePrefix),

		prompt.OptionPreviewSuggestionTextColor(prompt.Green),
		prompt.OptionPreviewSuggestionBGColor(prompt.Black),

		prompt.OptionSelectedSuggestionTextColor(prompt.White),
		prompt.OptionSelectedSuggestionBGColor(prompt.DarkGray),

		prompt.OptionSuggestionTextColor(prompt.LightGray),
		prompt.OptionSuggestionBGColor(prompt.Black),

		prompt.OptionDescriptionTextColor(prompt.Turquoise),
		prompt.OptionDescriptionBGColor(prompt.Black),
		prompt.OptionSelectedDescriptionTextColor(prompt.Turquoise),
		prompt.OptionSelectedDescriptionBGColor(prompt.Black),
	)
	p.Run()
}

func prettyPrint(obj interface{}) {
	mapB, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		fmt.Printf("error printing object %v: %v\n", obj, err)
		return
	}
	fmt.Println(string(mapB))
}

func trimmedBlocks(in string) []string {
	trimmed := strings.TrimSpace(in)
	blocks := strings.Split(trimmed, " ")
	var cleaned []string
	for _, n := range blocks {
		cleaned = append(cleaned, strings.TrimSpace(n))
	}
	return cleaned
}
