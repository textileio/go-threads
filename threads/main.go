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
	"github.com/textileio/go-threads/store"
	"github.com/textileio/go-threads/util"
	"google.golang.org/grpc"
)

var (
	apiClient      *client.Client
	apiTimeout     = time.Second * 5
	streamTimeout  = time.Second * 30
	openStream     = false
	promptPrefix   = ">>> "
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
			Text:        "modelFind",
			Description: "Find all entities by model name.",
		},
		{
			Text:        "modelFindByID",
			Description: "Find entity by model name and entity ID.",
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
	case "modelFind":
		if len(blocks) < 2 {
			fmt.Println("You must provide a model name.")
			return
		}
		modelFind(currentStore, blocks[1])
		return
	case "modelFindByID":
		if len(blocks) < 2 {
			fmt.Println("You must provide a model name.")
			return
		}
		if len(blocks) < 3 {
			fmt.Println("You must provide an entity ID.")
			return
		}
		modelFindByID(currentStore, blocks[1], blocks[2])
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
			fmt.Println("You must provide a store ID.")
			return
		}
		if currentStore == blocks[1] {
			fmt.Printf("Already using %s\n", blocks[1])
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

func modelFind(id string, model string) {
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	rawResults, err := apiClient.ModelFind(ctx, id, model, &store.JSONQuery{}, []*any{})
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

func modelFindByID(id string, model string, modelID string) {
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	entity := &any{}
	err := apiClient.ModelFindByID(ctx, id, model, modelID, entity)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	prettyPrint(entity)
}

func backgroundListen(ctx context.Context, id string) {
	channel, err := apiClient.Listen(ctx, id)
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
			if err := json.Unmarshal(val.Action.Entity, obj); err != nil {
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

func listen(id string) {
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
	cleaned := []string{}
	for _, n := range blocks {
		cleaned = append(cleaned, strings.TrimSpace(n))
	}
	return cleaned
}
