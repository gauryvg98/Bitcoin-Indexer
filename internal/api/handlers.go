package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"bitcoin-indexer/internal/database"
	"bitcoin-indexer/internal/models"

	"github.com/gorilla/mux"
)

// APIHandler handles HTTP API requests
type APIHandler struct {
	db *database.DB
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(db *database.DB) *APIHandler {
	return &APIHandler{db: db}
}

// GetBalance returns the balance for a specific address
func (h *APIHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]

	if address == "" {
		http.Error(w, "address parameter is required", http.StatusBadRequest)
		return
	}

	// Get balance with default confirmations required (3)
	balance, err := h.db.GetBalance(address, 3)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get balance: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(balance)
}

// GetUTXOs returns UTXOs for a specific address
func (h *APIHandler) GetUTXOs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]

	if address == "" {
		http.Error(w, "address parameter is required", http.StatusBadRequest)
		return
	}

	// Get UTXOs for the address
	utxos, err := h.db.GetUTXOsForAddress(address)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get UTXOs: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to API response format
	var utxoInfos []models.UTXOInfo
	for _, utxo := range utxos {
		utxoInfo := models.UTXOInfo{
			Txid:        utxo.Txid,
			Vout:        utxo.Vout,
			ValueSats:   utxo.ValueSats,
			Status:      utxo.Status,
			BlockHeight: utxo.BlockHeight,
			BlockHash:   utxo.BlockHash,
			FirstSeenAt: utxo.FirstSeenAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		if utxo.SpentAt != nil {
			spentAt := utxo.SpentAt.Format("2006-01-02T15:04:05Z07:00")
			utxoInfo.SpentAt = &spentAt
		}

		// Calculate confirmations
		if utxo.BlockHeight != nil {
			// Get current height from index progress
			progress, err := h.db.GetIndexProgress()
			if err == nil {
				utxoInfo.Confirmations = progress.LastHeight - *utxo.BlockHeight + 1
			}
		}

		utxoInfos = append(utxoInfos, utxoInfo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(utxoInfos)
}

// GetTransaction returns transaction information if it's relevant to our addresses
func (h *APIHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	txid := vars["txid"]

	if txid == "" {
		http.Error(w, "txid parameter is required", http.StatusBadRequest)
		return
	}

	// For now, return a simple response
	// In a full implementation, you'd query the database for transaction details
	response := map[string]interface{}{
		"txid":    txid,
		"message": "Transaction details not implemented yet",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AddWatchTarget adds a new watch target (address or xpub)
func (h *APIHandler) AddWatchTarget(w http.ResponseWriter, r *http.Request) {
	var target models.WatchTarget
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if target.Kind == "" {
		http.Error(w, "kind is required", http.StatusBadRequest)
		return
	}

	if target.Kind != "address" && target.Kind != "xpub" {
		http.Error(w, "kind must be 'address' or 'xpub'", http.StatusBadRequest)
		return
	}

	if target.Kind == "address" && (target.Address == nil || *target.Address == "") {
		http.Error(w, "address is required for kind 'address'", http.StatusBadRequest)
		return
	}

	if target.Kind == "xpub" && (target.Xpub == nil || *target.Xpub == "") {
		http.Error(w, "xpub is required for kind 'xpub'", http.StatusBadRequest)
		return
	}

	// Set default gap limit if not provided
	if target.GapLimit == 0 {
		target.GapLimit = 20
	}

	// Add the watch target
	if err := h.db.AddWatchTarget(&target); err != nil {
		http.Error(w, fmt.Sprintf("failed to add watch target: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(target)
}

// GetWatchTargets returns all watch targets
func (h *APIHandler) GetWatchTargets(w http.ResponseWriter, r *http.Request) {
	targets, err := h.db.GetWatchTargets()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get watch targets: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(targets)
}

// GetIndexStatus returns the current indexing status
func (h *APIHandler) GetIndexStatus(w http.ResponseWriter, r *http.Request) {
	progress, err := h.db.GetIndexProgress()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get index progress: %v", err), http.StatusInternalServerError)
		return
	}

	status := map[string]interface{}{
		"last_height":     progress.LastHeight,
		"last_block_hash": progress.LastBlockHash,
		"updated_at":      progress.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// HealthCheck returns the health status of the service
func (h *APIHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// Simple health check - in production you'd check database connectivity, etc.
	response := map[string]interface{}{
		"status":  "healthy",
		"service": "bitcoin-indexer",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetTransactionHistory returns transaction history for a specific address
func (h *APIHandler) GetTransactionHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["address"]

	if address == "" {
		http.Error(w, "address parameter is required", http.StatusBadRequest)
		return
	}

	// Get query parameters
	limit := 50 // default limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || parsedLimit != 1 {
			http.Error(w, "invalid limit parameter", http.StatusBadRequest)
			return
		}
	}

	// Get transaction history
	txHistory, err := h.db.GetTransactionHistory(address, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get transaction history: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(txHistory)
}

// ConnectWallet instantly connects a wallet by adding it to watchlist
func (h *APIHandler) ConnectWallet(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Address string `json:"address"`
		Label   string `json:"label,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if request.Address == "" {
		http.Error(w, "address is required", http.StatusBadRequest)
		return
	}

	// Create watch target
	target := models.WatchTarget{
		Kind:     "address",
		Address:  &request.Address,
		GapLimit: 20,
	}

	// Add the watch target
	if err := h.db.AddWatchTarget(&target); err != nil {
		http.Error(w, fmt.Sprintf("failed to connect wallet: %v", err), http.StatusInternalServerError)
		return
	}

	// Get transaction history for the newly connected wallet
	txHistory, err := h.db.GetTransactionHistory(request.Address, 50)
	if err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: failed to get transaction history for %s: %v\n", request.Address, err)
	}

	response := map[string]interface{}{
		"status":              "connected",
		"address":             request.Address,
		"target_id":           target.ID,
		"transaction_history": txHistory,
		"message":             "Wallet connected successfully. Historical data is available immediately.",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
