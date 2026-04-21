package main

import (
	"fmt"
	"log"
	"os"

	"zotero_cli/internal/config"
	"zotero_cli/internal/server"
)

func main() {
	cfg, path, err := config.Load()
	if err != nil {
		if err == config.ErrNotFound {
			fmt.Fprintf(os.Stderr, "Config not found at %s. Run 'zot init' first.\n", path)
			os.Exit(1)
		}
		log.Fatalf("Failed to load config: %v", err)
	}

	shutdown, err := server.ServeFromConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer shutdown()

	log.Println("Server running. Press Ctrl+C to stop.")

	// Block forever
	select {}
}
