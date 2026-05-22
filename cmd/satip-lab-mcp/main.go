package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/e12media/satip-lab/internal/mcp"
)

func main() {
	httpURL := flag.String("http-url", "http://127.0.0.1:8875", "satip-lab HTTP base URL")
	flag.Parse()

	server := mcp.NewServer(*httpURL)
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "satip-lab MCP server stopped: %v\n", err)
		os.Exit(1)
	}
}
