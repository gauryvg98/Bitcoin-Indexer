package workers

import (
	"time"

	"bitcoin-indexer/internal/bitcoin"
	"bitcoin-indexer/internal/models"
)

// WorkItem represents a unit of work to be processed by DB workers
type WorkItem struct {
	Height           int                              `json:"height"`
	BlockHash        string                           `json:"block_hash"`
	Block            *bitcoin.Block                   `json:"block"`
	NewUTXOs         []*models.UTXO                   `json:"new_utxos"`
	SpentUTXOs       []*models.UTXO                   `json:"spent_utxos"`
	TxReferences     []*models.TransactionReference   `json:"tx_references"`
	WatchedScriptMap map[string]*models.WatchedScript `json:"watched_script_map"`
	CreatedAt        time.Time                        `json:"created_at"`
	RPCWorkerID      int                              `json:"rpc_worker_id"`
}

// WorkResult represents the result of processing a work item
type WorkResult struct {
	Height      int           `json:"height"`
	BlockHash   string        `json:"block_hash"`
	Success     bool          `json:"success"`
	Error       error         `json:"error,omitempty"`
	ProcessTime time.Duration `json:"process_time"`
	DBWorkerID  int           `json:"db_worker_id"`
	CompletedAt time.Time     `json:"completed_at"`
}

// WorkStats tracks statistics for worker performance
type WorkStats struct {
	RPCWorkerID     int           `json:"rpc_worker_id"`
	DBWorkerID      int           `json:"db_worker_id"`
	BlocksProcessed int           `json:"blocks_processed"`
	TotalTime       time.Duration `json:"total_time"`
	AverageTime     time.Duration `json:"average_time"`
	LastProcessed   time.Time     `json:"last_processed"`
}

// WorkerConfig holds configuration for the worker pools
type WorkerConfig struct {
	RPCWorkers    int           `json:"rpc_workers"`
	DBWorkers     int           `json:"db_workers"`
	QueueSize     int           `json:"queue_size"`
	WorkerTimeout time.Duration `json:"worker_timeout"`
}
