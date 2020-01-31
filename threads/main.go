package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/c-bata/go-prompt"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
	"log"
	"os"
	"strings"
	"time"
)

var (
	apiClient      *client.Client
	apiTimeout     = time.Second * 5
	streamTimeout  = time.Second * 30
	noStoreMessage = "Store required. `use <store id>`"
	currentStore   string
	suggestions    = []prompt.Suggest{
		{
			Text:        "use",
			Description: "Switch the store.",
		},
		{
			Text:        "exit",
			Description: "Exit.",
		},
	}
	storeSuggestions = []prompt.Suggest{
		{
			Text:        "listen",
			Description: "Stream all store updates.",
		},
		{
			Text:        "getModel",
			Description: "Get all models by model name.",
		},
	}
)

type any map[string]interface{}

func completer(in prompt.Document) []prompt.Suggest {
	w := in.GetWordBeforeCursor()
	if w == "" {
		return []prompt.Suggest{}
	}
	if currentStore != "" {
		return prompt.FilterHasPrefix(append(suggestions, storeSuggestions...), w, true)
	}
	return prompt.FilterHasPrefix(suggestions, w, true)
}

func storeExecutor(blocks []string) {
	switch blocks[0] {
	case "listen":
		listen(currentStore)
		return
	case "getModel":
		if len(blocks) < 2 {
			fmt.Println("You must provide a model name.")
			return
		}
		getModel(currentStore, blocks[1], apiTimeout)
		return
	default:
		fmt.Println("Sorry, I don't understand.")
	}
}

func executor(in string) {
	in = strings.TrimSpace(in)
	blocks := strings.Split(in, " ")
	switch blocks[0] {
	case "":
		return
	case "use":
		if len(blocks) < 2 {
			fmt.Println("You must provide a store ID.")
			return
		}
		currentStore = blocks[1]
		fmt.Printf("Switched to %s\n", blocks[1])
		return
	case "exit":
		fmt.Println("Bye!")
		os.Exit(0)
	default:
		if currentStore == "" {
			fmt.Println(noStoreMessage)
			return
		}
		storeExecutor(blocks)
	}
}

func getModel(id string, model string, maxAwaitTime time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	rawResults, err := apiClient.ModelFind(ctx, id, model, nil, []*any{})
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	entities := rawResults.([]*any)
	if len(entities) == 0 {
		fmt.Println("None found")
		return
	}
	for _, el := range entities {
		prettyPrint(el)
	}
}

func listen(id string) {
	channel, err := apiClient.Listen(context.Background(), id)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	for {
		select {
		case val, ok := <-channel:
			if !ok {
				return
			} else {
				if val.Err != nil {
					fmt.Println(val.Err)
					continue
				}
				obj := &any{}
				if err := json.Unmarshal(val.Action.Entity, obj); err != nil {
					fmt.Println("failed to unmarshal listen result")
					continue
				}
				prettyPrint(obj)
			}
		case <-time.After(streamTimeout):
			fmt.Println("timeout")
			return
		}
	}
}

func main() {
	addr := flag.String("addr", "/ip4/0.0.0.0/tcp/6006", "Threads APIs")
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
		prompt.OptionPrefix(">>> "),

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
	mapB, err := json.MarshalIndent(obj, "", " ")
	if err != nil {
		fmt.Println(obj)
		return
	}
	fmt.Println(string(mapB))
}
