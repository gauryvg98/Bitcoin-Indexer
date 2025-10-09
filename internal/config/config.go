package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the Bitcoin indexer
type Config struct {
	// Bitcoin RPC
	BitcoinRPCURL      string
	BitcoinRPCUsername string
	BitcoinRPCPassword string

	// Database
	DatabaseURL string

	// Indexer Settings
	BlockPollInterval   time.Duration
	MempoolPollInterval time.Duration
	BackfillWorkers     int
	MaxReorgDepth       int

	// API Server
	ServerPort string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	config := &Config{
		// Bitcoin RPC (required)
		BitcoinRPCURL:      getEnv("BTC_RPC_URL", ""),
		BitcoinRPCUsername: getEnv("BTC_RPC_USERNAME", ""),
		BitcoinRPCPassword: getEnv("BTC_RPC_PASSWORD", ""),

		// Database (required)
		DatabaseURL: getEnv("DATABASE_URL", ""),

		// Indexer Settings
		BlockPollInterval:   getDurationEnv("BLOCK_POLL_INTERVAL", "10s"),
		MempoolPollInterval: getDurationEnv("MEMPOOL_POLL_INTERVAL", "5s"),
		BackfillWorkers:     getIntEnv("BACKFILL_WORKERS", 10), // Reduced from 20 to 10 for better stability
		MaxReorgDepth:       getIntEnv("MAX_REORG_DEPTH", 10),

		// API Server
		ServerPort: getEnv("SERVER_PORT", "8080"),
	}

	// Validate required configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.BitcoinRPCURL == "" {
		return fmt.Errorf("BTC_RPC_URL is required")
	}
	if c.BitcoinRPCUsername == "" {
		return fmt.Errorf("BTC_RPC_USERNAME is required")
	}
	if c.BitcoinRPCPassword == "" {
		return fmt.Errorf("BTC_RPC_PASSWORD is required")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.BackfillWorkers < 1 {
		return fmt.Errorf("BACKFILL_WORKERS must be at least 1")
	}
	return nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getIntEnv gets an integer environment variable with a default value
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getDurationEnv gets a duration environment variable with a default value
func getDurationEnv(key, defaultValue string) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}

	// Parse default value
	duration, err := time.ParseDuration(defaultValue)
	if err != nil {
		panic(fmt.Sprintf("invalid default duration %s: %v", defaultValue, err))
	}
	return duration
}
