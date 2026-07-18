package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	pb "github.com/miru-project/miru-core/proto/generate/proto"
)

// runAttach handles attach mode - connects to a running instance and executes subcommands
func runAttach(args []string) {
	if len(args) < 1 {
		fmt.Println("error: attach mode requires a subcommand")
		printAttachUsage()
		os.Exit(1)
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "search":
		runAttachWithEvents("search", func(client pb.ExtensionServiceClient, eventClient pb.EventServiceClient) {
			runAttachSearchWithEvents(subArgs, client, eventClient)
		})
	case "latest":
		runAttachWithEvents("latest", func(client pb.ExtensionServiceClient, eventClient pb.EventServiceClient) {
			runAttachLatestWithEvents(subArgs, client, eventClient)
		})
	case "detail":
		runAttachWithEvents("detail", func(client pb.ExtensionServiceClient, eventClient pb.EventServiceClient) {
			runAttachDetailWithEvents(subArgs, client, eventClient)
		})
	case "watch":
		runAttachWithEvents("watch", func(client pb.ExtensionServiceClient, eventClient pb.EventServiceClient) {
			runAttachWatchWithEvents(subArgs, client, eventClient)
		})
	case "mirror":
		runAttachWithEvents("mirror", func(client pb.ExtensionServiceClient, eventClient pb.EventServiceClient) {
			runAttachMirrorWithEvents(subArgs, client, eventClient)
		})
	default:
		fmt.Printf("unknown subcommand: %s\n", subcommand)
		printAttachUsage()
		os.Exit(1)
	}
}

// runAttachWithEvents connects to the server, starts event streaming in a goroutine,
// executes the given command function, then closes the event stream and returns.
func runAttachWithEvents(cmdName string, fn func(pb.ExtensionServiceClient, pb.EventServiceClient)) {
	client, _, eventClient, conn, err := checkInstance()
	if err != nil {
		log.Fatalf("miru_core not running: %v", err)
	}
	defer conn.Close()

	// Start event stream to capture dev logs and network events
	eventCtx, cancelEventStream := context.WithCancel(context.Background())
	defer cancelEventStream()

	eventStream, err := eventClient.WatchEvents(eventCtx, &pb.WatchEventsRequest{})
	if err != nil {
		log.Fatalf("Failed to start event stream: %v", err)
	}

	// Start a goroutine to stream events and print them to stdout
	eventDone := make(chan struct{})
	go func() {
		defer close(eventDone)
		for {
			resp, err := eventStream.Recv()
			if err != nil {
				return
			}
			if resp == nil || resp.Event == nil {
				continue
			}
			switch e := resp.Event.(type) {
			case *pb.WatchEventsResponse_DevLogEvent:
				logEvent := e.DevLogEvent
				fmt.Printf("[LOG] [%s] %s: %s\n", logEvent.Level, logEvent.Package, logEvent.Message)
			case *pb.WatchEventsResponse_DevNetworkEvent:
				netEvent := e.DevNetworkEvent
				fmt.Printf("[NET] [%s] %s %s\n", netEvent.Package, netEvent.Method, netEvent.Url)
			}
		}
	}()

	// Execute the command
	fn(client, eventClient)

	// Give a moment for any pending async events (sent via goroutines) to arrive
	time.Sleep(300 * time.Millisecond)

	// Close the event stream gracefully: cancel context first, then wait briefly
	cancelEventStream()
	select {
	case <-eventDone:
	case <-time.After(2 * time.Second):
	}
}

func runAttachSearchWithEvents(args []string, client pb.ExtensionServiceClient, _ pb.EventServiceClient) {
	if len(args) < 2 {
		fmt.Println("error: search requires <pkg> <kw> [page] [filter]")
		os.Exit(1)
	}
	pkg := args[0]
	kw := args[1]
	page := 1
	filter := ""
	if len(args) >= 3 {
		if p, err := strconv.Atoi(args[2]); err == nil {
			page = p
		}
	}
	if len(args) >= 4 {
		filter = args[3]
	}

	resp, err := client.Search(context.Background(), &pb.SearchRequest{Pkg: pkg, Kw: kw, Page: int32(page), Filter: filter})
	if err != nil {
		log.Fatalf("Search RPC failed: %v", err)
	}

	fmt.Printf("Search results for '%s' in %s (page %d):\n", kw, pkg, page)

	if len(resp.Items) == 0 {
		fmt.Println("  No items returned.")
		fmt.Println("\nExpected schema for ExtensionListItem:")
		printSearchItemSchema()
		return
	}

	for i, item := range resp.Items {
		if item.Title == "" {
			fmt.Printf("  %d. [WARNING: Item has empty title - data may be malformed]\n", i+1)
			fmt.Printf("     URL: %s\n", item.Url)
			fmt.Printf("     Cover: %s\n", item.Cover)
			fmt.Printf("     Update: %s\n", item.Update)
			if i == 0 {
				fmt.Println("\nExpected schema for ExtensionListItem:")
				printSearchItemSchema()
			}
			continue
		}
		fmt.Printf("  %d. %s\n", i+1, item.Title)
		fmt.Printf("     URL: %s\n", item.Url)
		if item.Cover != "" {
			fmt.Printf("     Cover: %s\n", item.Cover)
		}
		if item.Update != "" {
			fmt.Printf("     Update: %s\n", item.Update)
		}
		fmt.Println()
	}
}

func runAttachLatestWithEvents(args []string, client pb.ExtensionServiceClient, _ pb.EventServiceClient) {
	if len(args) < 1 {
		fmt.Println("error: latest requires <pkg> [page]")
		os.Exit(1)
	}
	pkg := args[0]
	page := 1
	if len(args) >= 2 {
		if p, err := strconv.Atoi(args[1]); err == nil {
			page = p
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Latest(ctx, &pb.LatestRequest{Pkg: pkg, Page: int32(page)})
	if err != nil {
		log.Fatalf("Latest RPC failed: %v", err)
	}

	fmt.Printf("Latest items from %s (page %d):\n", pkg, page)

	if len(resp.Items) == 0 {
		fmt.Println("  No items returned.")
		fmt.Println("\nExpected schema for ExtensionListItem:")
		printSearchItemSchema()
		return
	}

	for i, item := range resp.Items {
		if item.Title == "" {
			fmt.Printf("  %d. [WARNING: Item has empty title - data may be malformed]\n", i+1)
			fmt.Printf("     URL: %s\n", item.Url)
			fmt.Printf("     Cover: %s\n", item.Cover)
			fmt.Printf("     Update: %s\n", item.Update)
			if i == 0 {
				fmt.Println("\nExpected schema for ExtensionListItem:")
				printSearchItemSchema()
			}
			continue
		}
		fmt.Printf("  %d. %s\n", i+1, item.Title)
		fmt.Printf("     URL: %s\n", item.Url)
		if item.Cover != "" {
			fmt.Printf("     Cover: %s\n", item.Cover)
		}
		if item.Update != "" {
			fmt.Printf("     Update: %s\n", item.Update)
		}
		fmt.Println()
	}
}

func runAttachDetailWithEvents(args []string, client pb.ExtensionServiceClient, _ pb.EventServiceClient) {
	if len(args) < 2 {
		fmt.Println("error: detail requires <pkg> <url>")
		os.Exit(1)
	}
	pkg := args[0]
	url := args[1]

	resp, err := client.Detail(context.Background(), &pb.DetailRequest{Pkg: pkg, Url: url})
	if err != nil {
		log.Fatalf("Detail RPC failed: %v", err)
	}

	data := resp.Data
	if data == nil {
		fmt.Printf("Detail for %s:\n", pkg)
		fmt.Println("  [WARNING: No data returned - response may be malformed]")
		fmt.Println("\nExpected schema for ExtensionDetail:")
		printDetailSchema()
		return
	}

	fmt.Printf("Detail for %s:\n", pkg)

	hasContent := false
	if data.Title != nil {
		fmt.Printf("  Title: %s\n", *data.Title)
		hasContent = true
	}
	if data.Cover != nil {
		fmt.Printf("  Cover: %s\n", *data.Cover)
		hasContent = true
	}
	if data.Desc != nil {
		fmt.Printf("  Description: %s\n", *data.Desc)
		hasContent = true
	}
	if len(data.Episodes) > 0 {
		hasContent = true
		fmt.Println("  Episodes:")
		for _, group := range data.Episodes {
			fmt.Printf("    Group: %s\n", group.Title)
			for _, ep := range group.Urls {
				if ep.Name == "" && ep.Url == "" {
					fmt.Println("      [WARNING: Episode has empty name and URL - data may be malformed]")
					continue
				}
				fmt.Printf("      - %s: %s", ep.Name, ep.Url)
				if ep.Update != "" {
					fmt.Printf(" (updated: %s)", ep.Update)
				}
				if ep.Description != nil {
					fmt.Printf(" - %s", *ep.Description)
				}
				fmt.Println()
			}
		}
	}
	if len(data.Headers) > 0 {
		hasContent = true
		fmt.Println("  Headers:")
		for k, v := range data.Headers {
			fmt.Printf("    %s: %s\n", k, v)
		}
	}

	if !hasContent {
		fmt.Println("  [WARNING: Detail response returned with no content]")
		fmt.Println("\nExpected schema for ExtensionDetail:")
		printDetailSchema()
	}
}

func runAttachWatchWithEvents(args []string, client pb.ExtensionServiceClient, _ pb.EventServiceClient) {
	if len(args) < 2 {
		fmt.Println("error: watch requires <pkg> <url>")
		os.Exit(1)
	}
	pkg := args[0]
	url := args[1]

	resp, err := client.Watch(context.Background(), &pb.WatchRequest{Pkg: pkg, Url: url})
	if err != nil {
		log.Fatalf("Watch RPC failed: %v", err)
	}

	fmt.Printf("Watch URLs for %s:\n", pkg)

	if resp.Data == nil {
		fmt.Println("  [WARNING: No watch data returned - response may be malformed]")
		fmt.Println("\nExpected schema for WatchResponse (oneof data):")
		printWatchSchema()
		return
	}

	switch data := resp.Data.(type) {
	case *pb.WatchResponse_Bangumi:
		if data.Bangumi.Url == "" {
			fmt.Println("  [WARNING: Bangumi URL is empty - data may be malformed]")
			fmt.Println("\nExpected schema for ExtensionBangumiWatch:")
			printBangumiWatchSchema()
		}
		fmt.Printf("  URL: %s\n", data.Bangumi.Url)
	case *pb.WatchResponse_Manga:
		if len(data.Manga.Urls) == 0 {
			fmt.Println("  [WARNING: No manga URLs returned - data may be malformed]")
			fmt.Println("\nExpected schema for ExtensionMangaWatch:")
			printMangaWatchSchema()
		}
		for i, u := range data.Manga.Urls {
			fmt.Printf("  Page %d: %s\n", i+1, u)
		}
	case *pb.WatchResponse_Watch:
		if len(data.Watch.Groups) == 0 {
			fmt.Println("  [WARNING: No mirror groups returned - data may be malformed]")
			fmt.Println("\nExpected schema for ExtensionWatch:")
			printWatchV2Schema()
		}
		for _, group := range data.Watch.Groups {
			fmt.Printf("  Group: %s\n", group.Title)
			for i, mirror := range group.Mirrors {
				fmt.Printf("    %d. %s: %s\n", i+1, mirror.Name, mirror.Url)
			}
		}
	case *pb.WatchResponse_All:
		if data.All == nil {
			fmt.Println("  [WARNING: All watch is empty - data may be malformed]")
			fmt.Println("\nExpected schema for ExtensionAllWatch:")
			printAllWatchSchema()
		}
		if data.All.Manga != nil {
			fmt.Printf("  [MANGA] %d page(s)\n", len(data.All.Manga.Urls))
			for i, u := range data.All.Manga.Urls {
				fmt.Printf("    Page %d: %s\n", i+1, u)
			}
		}
		if data.All.Fikushon != nil {
			fmt.Printf("  [FIKUSHON] %q (%d paragraph(s))\n", data.All.Fikushon.Title, len(data.All.Fikushon.Content))
			for i, c := range data.All.Fikushon.Content {
				fmt.Printf("    %d. %s\n", i+1, c)
			}
		}
		if data.All.Bangumi != nil {
			fmt.Printf("  [BANGUMI] %s\n", data.All.Bangumi.Url)
		}
	default:
		fmt.Println("  No watch data available")
		fmt.Println("\nExpected schema for WatchResponse (oneof data):")
		printWatchSchema()
	}
}

func runAttachMirrorWithEvents(args []string, client pb.ExtensionServiceClient, _ pb.EventServiceClient) {
	if len(args) < 2 {
		fmt.Println("error: mirror requires <pkg> <url>")
		os.Exit(1)
	}
	pkg := args[0]
	url := args[1]

	resp, err := client.Mirror(context.Background(), &pb.MirrorRequest{Pkg: pkg, Url: url})
	if err != nil {
		log.Fatalf("Mirror RPC failed: %v", err)
	}

	fmt.Printf("Mirror URLs for %s:\n", pkg)

	if resp.Data == nil {
		fmt.Println("  [WARNING: No mirror data returned - response may be malformed]")
		fmt.Println("\nExpected schema for MirrorResponse (oneof data):")
		printMirrorSchema()
		return
	}

	switch data := resp.Data.(type) {
	case *pb.MirrorResponse_Bangumi:
		if data.Bangumi.Url == "" {
			fmt.Println("  [WARNING: Bangumi URL is empty - data may be malformed]")
			fmt.Println("\nExpected schema for ExtensionBangumiWatch:")
			printBangumiWatchSchema()
		}
		fmt.Printf("  URL: %s\n", data.Bangumi.Url)
	case *pb.MirrorResponse_Manga:
		if len(data.Manga.Urls) == 0 {
			fmt.Println("  [WARNING: No manga URLs returned - data may be malformed]")
			fmt.Println("\nExpected schema for ExtensionMangaWatch:")
			printMangaWatchSchema()
		}
		for i, u := range data.Manga.Urls {
			fmt.Printf("  Page %d: %s\n", i+1, u)
		}
	case *pb.MirrorResponse_Fikushon:
		if data.Fikushon.Title == "" {
			fmt.Println("  [WARNING: Fikushon title is empty]")
			fmt.Println("\nExpected schema for ExtensionFikushonWatch:")
			printFikushonWatchSchema()
		}
		fmt.Printf("  Title: %s\n", data.Fikushon.Title)
		if data.Fikushon.Subtitle != nil {
			fmt.Printf("  Subtitle: %s\n", *data.Fikushon.Subtitle)
		}
		for i, c := range data.Fikushon.Content {
			fmt.Printf("  Content %d: %s\n", i+1, c)
		}
	case *pb.MirrorResponse_All:
		if data.All == nil {
			fmt.Println("  [WARNING: All mirror is empty - data may be malformed]")
			fmt.Println("\nExpected schema for ExtensionAllWatch:")
			printAllWatchSchema()
		}
		if data.All.Manga != nil {
			fmt.Printf("  [MANGA] %d page(s)\n", len(data.All.Manga.Urls))
		}
		if data.All.Fikushon != nil {
			fmt.Printf("  [FIKUSHON] %q\n", data.All.Fikushon.Title)
		}
		if data.All.Bangumi != nil {
			fmt.Printf("  [BANGUMI] %s\n", data.All.Bangumi.Url)
		}
	default:
		fmt.Println("  No mirror data available")
		fmt.Println("\nExpected schema for MirrorResponse (oneof data):")
		printMirrorSchema()
	}
}

// Schema printers for reference
func printSearchItemSchema() {
	fmt.Println("  ExtensionListItem {")
	fmt.Println("    title:   string  (item title)")
	fmt.Println("    url:     string  (item URL)")
	fmt.Println("    cover:   string  (cover image URL)")
	fmt.Println("    update:  string  (last update timestamp)")
	fmt.Println("    headers: map<string, string>")
	fmt.Println("  }")
}

func printDetailSchema() {
	fmt.Println("  ExtensionDetail {")
	fmt.Println("    title:     optional string             (item title)")
	fmt.Println("    cover:     optional string             (cover image URL)")
	fmt.Println("    desc:      optional string             (description)")
	fmt.Println("    episodes:  repeated ExtensionEpisodeGroup")
	fmt.Println("      ExtensionEpisodeGroup {")
	fmt.Println("        title: string                       (group title e.g. 'Season 1')")
	fmt.Println("        urls:  repeated ExtensionEpisode")
	fmt.Println("          ExtensionEpisode {")
	fmt.Println("            name:        string             (episode name/number)")
	fmt.Println("            url:         string             (episode URL)")
	fmt.Println("            update:      string             (last update)")
	fmt.Println("            description: optional string    (episode description)")
	fmt.Println("          }")
	fmt.Println("      }")
	fmt.Println("    headers:   map<string, string>")
	fmt.Println("  }")
}

func printWatchSchema() {
	fmt.Println("  WatchResponse {")
	fmt.Println("    oneof data {")
	fmt.Println("      bangumi  -> ExtensionBangumiWatch  (video streaming URL)")
	fmt.Println("      manga    -> ExtensionMangaWatch    (manga page URLs)")
	fmt.Println("      fikushon -> ExtensionFikushonWatch (novel content)")
	fmt.Println("      all      -> ExtensionAllWatch      (manga + fikushon + bangumi)")
	fmt.Println("      watch    -> ExtensionWatch         (multi-mirror V2 format)")
	fmt.Println("    }")
	fmt.Println("  }")
}

func printAllWatchSchema() {
	fmt.Println("  ExtensionAllWatch {")
	fmt.Println("    manga    -> ExtensionMangaWatch    (manga page URLs)")
	fmt.Println("    fikushon -> ExtensionFikushonWatch (novel content)")
	fmt.Println("    bangumi  -> ExtensionBangumiWatch  (video streaming URL)")
	fmt.Println("  }")
}

func printWatchV2Schema() {
	fmt.Println("  ExtensionWatch {")
	fmt.Println("    groups:        repeated ExtensionMirrorGroup")
	fmt.Println("      ExtensionMirrorGroup {")
	fmt.Println("        title:   string                 (group title)")
	fmt.Println("        mirrors: repeated ExtensionMirror")
	fmt.Println("          ExtensionMirror {")
	fmt.Println("            name: string                (mirror name)")
	fmt.Println("            url:  string                (mirror URL)")
	fmt.Println("          }")
	fmt.Println("      }")
	fmt.Println("    default_group:  optional string     (default group name)")
	fmt.Println("    default_index:  optional int32      (default mirror index)")
	fmt.Println("  }")
}

func printBangumiWatchSchema() {
	fmt.Println("  ExtensionBangumiWatch {")
	fmt.Println("    type:      string                              (media type)")
	fmt.Println("    url:       string                              (video URL)")
	fmt.Println("    subtitles: repeated ExtensionBangumiWatchSubtitle")
	fmt.Println("      ExtensionBangumiWatchSubtitle {")
	fmt.Println("        language: optional string                  (language code)")
	fmt.Println("        title:    string                           (subtitle title)")
	fmt.Println("        url:      string                           (subtitle URL)")
	fmt.Println("      }")
	fmt.Println("    headers:    map<string, string>")
	fmt.Println("    audio_track: optional string                   (audio track)")
	fmt.Println("  }")
}

func printMangaWatchSchema() {
	fmt.Println("  ExtensionMangaWatch {")
	fmt.Println("    urls:    repeated string         (page image URLs)")
	fmt.Println("    headers: map<string, string>")
	fmt.Println("  }")
}

func printFikushonWatchSchema() {
	fmt.Println("  ExtensionFikushonWatch {")
	fmt.Println("    content:  repeated string       (text content paragraphs)")
	fmt.Println("    title:    string                 (chapter title)")
	fmt.Println("    subtitle: optional string        (subtitle)")
	fmt.Println("  }")
}

func printMirrorSchema() {
	fmt.Println("  MirrorResponse {")
	fmt.Println("    oneof data {")
	fmt.Println("      bangumi  -> ExtensionBangumiWatch  (video streaming URL)")
	fmt.Println("      manga    -> ExtensionMangaWatch    (manga page URLs)")
	fmt.Println("      fikushon -> ExtensionFikushonWatch (novel content)")
	fmt.Println("      all      -> ExtensionAllWatch      (manga + fikushon + bangumi)")
	fmt.Println("    }")
	fmt.Println("  }")
}
