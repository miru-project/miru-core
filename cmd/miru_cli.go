package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"

	"github.com/miru-project/miru-core/config"
	pb "github.com/miru-project/miru-core/proto/generate/proto"
)

// checkInstance tries to establish a gRPC connection to the miru_core service.
func checkInstance() (pb.ExtensionServiceClient, pb.MiruCoreServiceClient, pb.EventServiceClient, *grpc.ClientConn, error) {
	// Load configuration (default path config.json)
	if err := config.Load("config.json"); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	addr := fmt.Sprintf("%s:%s", config.Global.Address, config.Global.GRPCPort)
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not connect to miru_core at %s: %w", addr, err)
	}
	extClient := pb.NewExtensionServiceClient(conn)
	coreClient := pb.NewMiruCoreServiceClient(conn)
	eventClient := pb.NewEventServiceClient(conn)
	return extClient, coreClient, eventClient, conn, nil
}

func main() {
	// Define flags
	attachFlag := flag.Bool("attach", false, "Attach to existing running instance (for extension testing)")
	helpFlag := flag.Bool("help", false, "Show help")
	flag.Usage = printUsage
	flag.Parse()

	if *helpFlag {
		printUsage()
		os.Exit(0)
	}

	// Mode 1: Attach mode - connects to existing instance and runs subcommands
	if *attachFlag {
		args := flag.Args()
		if len(args) < 1 {
			fmt.Println("error: attach mode requires a subcommand (search, latest, detail, watch, mirror)")
			printAttachUsage()
			os.Exit(1)
		}
		runAttach(args)
		return
	}

	// Mode 2: Interactive mode - no attach flag
	args := flag.Args()
	keyword := strings.Join(args, " ")
	runInteractive(keyword)
}

func printUsage() {
	fmt.Println("miru-cli - Miru Core CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  miru-cli [keyword]              Interactive mode (search, select, watch)")
	fmt.Println("  miru-cli --attach <command>      Attach mode (connect to running instance)")
	fmt.Println("  miru-cli --help                  Show this help")
	fmt.Println()
	fmt.Println("Interactive Mode (default):")
	fmt.Println("  Search for content, select episodes, and watch with mpv player.")
	fmt.Println("  Provide a keyword to search directly, or run without arguments for interactive prompts.")
	fmt.Println()
	fmt.Println("Attach Mode (--attach):")
	fmt.Println("  Attach to a running miru_core instance and execute commands.")
	fmt.Println("  Use for extension testing and scripting.")
	fmt.Println()
	fmt.Println("  Subcommands:")
	fmt.Println("    search <pkg> <kw> [page] [filter]   Search for items")
	fmt.Println("    latest <pkg> [page]                 Fetch latest items")
	fmt.Println("    detail <pkg> <url>                  Get detail info")
	fmt.Println("    watch <pkg> <url>                   Get watch URLs")
	fmt.Println("    mirror <pkg> <url>                  Get mirror URLs")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  miru-cli \"attack on titan\"         Interactive search")
	fmt.Println("  miru-cli                            Interactive mode (prompt)")
	fmt.Println("  miru-cli --attach search animepahe naruto")
	fmt.Println("  miru-cli --attach watch animepahe /episode/123")
}

func printAttachUsage() {
	fmt.Println("Attach mode usage: miru-cli --attach <subcommand> [arguments]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  search <pkg> <kw> [page] [filter]   Search for items")
	fmt.Println("  latest <pkg> [page]                 Fetch latest items")
	fmt.Println("  detail <pkg> <url>                  Get detail info")
	fmt.Println("  watch <pkg> <url>                   Get watch URLs")
	fmt.Println("  mirror <pkg> <url>                  Get mirror URLs")
	fmt.Println("  events                              Stream events (logs, network, downloads)")
}
