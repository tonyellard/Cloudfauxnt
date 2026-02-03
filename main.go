package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	log.Printf("Loading configuration from %s", *configPath)
	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("CloudFauxnt starting with %d origin(s)", len(config.Origins))
	for _, origin := range config.Origins {
		log.Printf("  - %s: %s (patterns: %v)", origin.Name, origin.URL, origin.PathPatterns)
	}

	// Initialize signature validator if signing is enabled
	var validator *SignatureValidator
	if config.Signing.Enabled {
		validator = NewSignatureValidator(config.Signing.PublicKey, config.Signing.KeyPairID)
		log.Printf("CloudFront signature validation enabled (Key Pair ID: %s)", config.Signing.KeyPairID)
	} else {
		log.Println("CloudFront signature validation disabled")
	}

	// Setup router
	router := SetupRouter(config, validator)

	// Configure HTTP server
	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  time.Duration(config.Server.TimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(config.Server.TimeoutSeconds) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server
	log.Printf("CloudFauxnt listening on %s", addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
