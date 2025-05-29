package main

import (
	"log"

	"mac-powermetrics-exporter/internal/config"
	"mac-powermetrics-exporter/internal/server"
)

func main() {
	// Load configuration
	cfg := config.New()

	// Create and start server
	srv := server.New(cfg)
	log.Fatal(srv.Start())
}
