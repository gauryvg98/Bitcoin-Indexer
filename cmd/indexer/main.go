package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bitcoin-indexer/internal/api"
	"bitcoin-indexer/internal/bitcoin"
	"bitcoin-indexer/internal/config"
	"bitcoin-indexer/internal/database"
	"bitcoin-indexer/internal/indexer"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	db, err := database.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize Bitcoin RPC client
	rpcClient := bitcoin.NewRPCClient(
		cfg.BitcoinRPCURL,
		cfg.BitcoinRPCUsername,
		cfg.BitcoinRPCPassword,
	)

	// Initialize indexer
	indexerConfig := &indexer.Config{
		BlockPollInterval:   cfg.BlockPollInterval,
		MempoolPollInterval: cfg.MempoolPollInterval,
		BackfillWorkers:     cfg.BackfillWorkers,
		MaxReorgDepth:       cfg.MaxReorgDepth,
	}

	btcIndexer := indexer.NewIndexer(rpcClient, db, indexerConfig)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start indexer
	if err := btcIndexer.Start(ctx); err != nil {
		log.Fatalf("Failed to start indexer: %v", err)
	}

	// Setup API routes
	router := api.SetupRoutes(db)

	// Start HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting HTTP server on port %s", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop indexer
	if err := btcIndexer.Stop(); err != nil {
		log.Printf("Error stopping indexer: %v", err)
	}

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
	}

	log.Println("Server stopped")
}
