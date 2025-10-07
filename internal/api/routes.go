package api

import (
	"bitcoin-indexer/internal/database"

	"github.com/gorilla/mux"
)

// SetupRoutes sets up the API routes
func SetupRoutes(db *database.DB) *mux.Router {
	handler := NewAPIHandler(db)

	router := mux.NewRouter()

	// API v1 routes
	v1 := router.PathPrefix("/v1/btc").Subrouter()

	// Balance endpoints
	v1.HandleFunc("/balance/{address}", handler.GetBalance).Methods("GET")

	// UTXO endpoints
	v1.HandleFunc("/utxos/{address}", handler.GetUTXOs).Methods("GET")

	// Transaction endpoints
	v1.HandleFunc("/tx/{txid}", handler.GetTransaction).Methods("GET")

	// Watchlist management
	v1.HandleFunc("/watch", handler.AddWatchTarget).Methods("POST")
	v1.HandleFunc("/watch", handler.GetWatchTargets).Methods("GET")

	// Transaction history
	v1.HandleFunc("/history/{address}", handler.GetTransactionHistory).Methods("GET")

	// Wallet connection
	v1.HandleFunc("/connect", handler.ConnectWallet).Methods("POST")

	// Status endpoints
	v1.HandleFunc("/status", handler.GetIndexStatus).Methods("GET")

	// Health check
	router.HandleFunc("/health", handler.HealthCheck).Methods("GET")

	return router
}
