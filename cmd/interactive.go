package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/miru-project/miru-core/config"
	pb "github.com/miru-project/miru-core/proto/generate/proto"
)

// runInteractive starts the interactive CLI mode
func runInteractive(keyword string) {
	// Try to connect to the running instance first
	client, coreClient, _, conn, err := checkInstance()
	if err != nil {
		// If not running, show instructions to start miru_core
		fmt.Println("miru_core is not running. Please start it first:")
		fmt.Println("  Run the miru_core server (e.g., the main miru-core binary)")
		os.Exit(1)
	}
	defer conn.Close()

	// Get available extensions
	var extensions []*pb.ExtensionMeta
	helloResp, err := coreClient.HelloMiru(context.Background(), &pb.HelloMiruRequest{})
	if err == nil {
		extensions = helloResp.ExtensionMeta
	} else {
		log.Printf("Warning: could not fetch extension list: %v", err)
	}

	// Main interactive flow
	pkg := selectExtension(extensions)

	if keyword == "" {
		keyword = promptInput("Enter search keyword: ")
	}

	if keyword == "" {
		fmt.Println("No search keyword provided. Exiting.")
		os.Exit(0)
	}

	// Search for items
	items := searchItems(client, pkg, keyword)
	if len(items) == 0 {
		fmt.Println("No results found.")
		os.Exit(0)
	}

	// Select an item
	selectedItem := selectItem(items)
	if selectedItem == nil {
		fmt.Println("No item selected. Exiting.")
		os.Exit(0)
	}

	// Get detail / episodes
	episodeGroups := getDetail(client, pkg, selectedItem.Url)
	if len(episodeGroups) == 0 {
		fmt.Printf("No episodes available for '%s'.\n", selectedItem.Title)
		os.Exit(0)
	}

	// Select an episode
	episode := selectEpisode(episodeGroups)
	if episode == nil {
		fmt.Println("No episode selected. Exiting.")
		os.Exit(0)
	}

	// Get watch URLs (mirrors)
	watchResp := getWatch(client, pkg, episode.Url)

	// Select a mirror (if multiple)
	videoURL := selectMirror(watchResp)
	if videoURL == "" {
		fmt.Println("No video URL available. Exiting.")
		os.Exit(0)
	}

	// Play with mpv
	playWithMpv(videoURL, selectedItem.Title, episode.Name)
}

func selectExtension(extensions []*pb.ExtensionMeta) string {
	if len(extensions) == 0 {
		// Fall back: prompt for package name
		return promptInput("Enter extension package name (e.g., animepahe.ru): ")
	}

	fmt.Println("\nAvailable extensions:")
	fmt.Println("  0. [All extensions]")
	for i, ext := range extensions {
		status := ""
		if ext.Error != "" {
			status = " [ERROR: " + ext.Error + "]"
		}
		fmt.Printf("  %d. %s (%s)%s\n", i+1, ext.Name, ext.Package, status)
	}
	fmt.Println()

	for {
		input := promptInput("Select extension (number): ")
		idx, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("Invalid input. Please enter a number.")
			continue
		}
		if idx == 0 {
			return "" // Empty string means search all
		}
		if idx >= 1 && idx <= len(extensions) {
			ext := extensions[idx-1]
			if ext.Error != "" {
				fmt.Printf("Extension '%s' has error: %s\n", ext.Name, ext.Error)
				continue
			}
			return ext.Package
		}
		fmt.Printf("Invalid selection. Choose 0-%d.\n", len(extensions))
	}
}

func searchItems(client pb.ExtensionServiceClient, pkg, keyword string) []*pb.ExtensionListItem {
	fmt.Printf("\nSearching for '%s'", keyword)
	if pkg != "" {
		fmt.Printf(" in '%s'", pkg)
	}
	fmt.Println("...")

	page := 1

	// Try with specific package first
	if pkg != "" {
		resp, err := client.Search(context.Background(), &pb.SearchRequest{
			Pkg:  pkg,
			Kw:   keyword,
			Page: int32(page),
		})
		if err == nil && len(resp.Items) > 0 {
			return resp.Items
		}
	}

	// If no results, try across all extensions (iterate through available ones)
	// For now, we'll just return what we got or try the general search
	resp, err := client.Search(context.Background(), &pb.SearchRequest{
		Pkg:  pkg,
		Kw:   keyword,
		Page: int32(page),
	})
	if err != nil {
		log.Printf("Search failed: %v", err)
		return nil
	}

	return resp.Items
}

func selectItem(items []*pb.ExtensionListItem) *pb.ExtensionListItem {
	fmt.Println("\nSearch results:")
	for i, item := range items {
		fmt.Printf("  %d. %s\n", i+1, item.Title)
		if item.Update != "" {
			fmt.Printf("     Update: %s\n", item.Update)
		}
	}
	fmt.Println()

	for {
		input := promptInput("Select item (number): ")
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(items) {
			fmt.Printf("Invalid input. Enter a number between 1 and %d.\n", len(items))
			continue
		}
		return items[idx-1]
	}
}

func getDetail(client pb.ExtensionServiceClient, pkg, url string) []*pb.ExtensionEpisodeGroup {
	fmt.Println("\nFetching details...")

	resp, err := client.Detail(context.Background(), &pb.DetailRequest{Pkg: pkg, Url: url})
	if err != nil {
		log.Printf("Detail RPC failed: %v", err)
		return nil
	}

	data := resp.Data
	if data.Title != nil {
		fmt.Printf("Title: %s\n", *data.Title)
	}
	if data.Desc != nil {
		desc := *data.Desc
		if len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		fmt.Printf("Description: %s\n", desc)
	}

	return data.Episodes
}

func selectEpisode(groups []*pb.ExtensionEpisodeGroup) *pb.ExtensionEpisode {
	fmt.Println("\nEpisodes:")

	var allEpisodes []*pb.ExtensionEpisode

	for _, group := range groups {
		if group.Title != "" {
			fmt.Printf("  [%s]\n", group.Title)
		}
		for _, ep := range group.Urls {
			label := ep.Name
			if ep.Description != nil && *ep.Description != "" {
				label = label + " - " + *ep.Description
			}
			allEpisodes = append(allEpisodes, ep)
			fmt.Printf("  %d. %s\n", len(allEpisodes), label)
		}
	}
	fmt.Println()

	if len(allEpisodes) == 0 {
		return nil
	}
	if len(allEpisodes) == 1 {
		fmt.Println("Only one episode available. Auto-selecting.")
		return allEpisodes[0]
	}

	for {
		input := promptInput("Select episode (number): ")
		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(allEpisodes) {
			fmt.Printf("Invalid input. Enter a number between 1 and %d.\n", len(allEpisodes))
			continue
		}
		return allEpisodes[idx-1]
	}
}

func getWatch(client pb.ExtensionServiceClient, pkg, url string) *pb.WatchResponse {
	fmt.Println("\nFetching watch URLs...")

	resp, err := client.Watch(context.Background(), &pb.WatchRequest{Pkg: pkg, Url: url})
	if err != nil {
		log.Printf("Watch RPC failed: %v", err)
		return nil
	}

	return resp
}

func selectMirror(watchResp *pb.WatchResponse) string {
	if watchResp == nil {
		return ""
	}

	switch data := watchResp.Data.(type) {
	case *pb.WatchResponse_Bangumi:
		return data.Bangumi.Url
	case *pb.WatchResponse_Manga:
		if len(data.Manga.Urls) > 0 {
			return data.Manga.Urls[0]
		}
	case *pb.WatchResponse_Fikushon:
		// Fikushon is text content, not playable
		fmt.Println("Fikushon content is not playable with mpv.")
		return ""
	case *pb.WatchResponse_Watch:
		groups := data.Watch.Groups
		if len(groups) == 0 {
			return ""
		}
		// If default group/index is specified, use that
		if data.Watch.DefaultGroup != nil && data.Watch.DefaultIndex != nil {
			for _, group := range groups {
				if group.Title == *data.Watch.DefaultGroup {
					idx := int(*data.Watch.DefaultIndex)
					if idx >= 0 && idx < len(group.Mirrors) {
						return group.Mirrors[idx].Url
					}
				}
			}
		}

		// Show mirrors and let user select
		fmt.Println("\nAvailable mirrors:")
		var allMirrors []*pb.ExtensionMirror
		for _, group := range groups {
			if len(group.Mirrors) == 0 {
				continue
			}
			if group.Title != "" {
				fmt.Printf("  [%s]\n", group.Title)
			}
			for _, mirror := range group.Mirrors {
				allMirrors = append(allMirrors, mirror)
				fmt.Printf("    %d. %s\n", len(allMirrors), mirror.Name)
			}
		}

		if len(allMirrors) == 0 {
			return ""
		}
		if len(allMirrors) == 1 {
			fmt.Printf("Auto-selecting: %s\n", allMirrors[0].Name)
			return allMirrors[0].Url
		}

		for {
			input := promptInput("Select mirror (number): ")
			idx, err := strconv.Atoi(input)
			if err != nil || idx < 1 || idx > len(allMirrors) {
				fmt.Printf("Invalid input. Enter a number between 1 and %d.\n", len(allMirrors))
				continue
			}
			return allMirrors[idx-1].Url
		}
	case *pb.WatchResponse_Raw:
		return data.Raw
	}

	return ""
}

func playWithMpv(url string, title string, episodeName string) {
	fmt.Printf("\nPlaying with mpv:\n")
	fmt.Printf("  Title: %s\n", title)
	if episodeName != "" {
		fmt.Printf("  Episode: %s\n", episodeName)
	}
	fmt.Printf("  URL: %s\n", url)

	// Check if mpv is installed
	if _, err := exec.LookPath("mpv"); err != nil {
		// Try mpv.net or other alternatives
		if _, err := exec.LookPath("mpv.net"); err != nil {
			fmt.Println("\nmpv player not found. Please install mpv to play videos.")
			fmt.Printf("You can manually open the URL in your browser:\n  %s\n", url)
			return
		}
	}

	fmt.Println("\nLaunching mpv...")

	// Build mpv command with useful flags
	args := []string{
		url,
		"--no-ytdl", // Don't use youtube-dl, use direct URL
		"--no-terminal",
		"--quiet",
		"--title=" + title + " - " + episodeName,
	}

	cmd := exec.Command("mpv", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("mpv exited with error: %v", err)
	}
}

func promptInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(input)
}

// init function to check for config file
func init() {
	if err := config.Load("config.json"); err != nil {
		// Config will be loaded again in checkInstance, so just ignore here
	}
}
