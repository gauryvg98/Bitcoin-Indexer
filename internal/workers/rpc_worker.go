package workers

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"bitcoin-indexer/internal/bitcoin"
	"bitcoin-indexer/internal/models"
	"bitcoin-indexer/internal/parser"
)

// RPCWorker handles fetching blocks and preparing work items for DB workers
type RPCWorker struct {
	ID                   int
	rpcPool              *bitcoin.RPCPool
	parser               *parser.Parser
	workQueue            chan<- *WorkItem
	stopChan             chan struct{}
	wg                   *sync.WaitGroup
	stats                *WorkStats
	mu                   sync.RWMutex
	watchedScripts       map[string]*models.WatchedScript
	lastScriptUpdate     time.Time
	scriptUpdateInterval time.Duration
}

// NewRPCWorker creates a new RPC worker
func NewRPCWorker(id int, rpcPool *bitcoin.RPCPool, parser *parser.Parser, workQueue chan<- *WorkItem, wg *sync.WaitGroup) *RPCWorker {
	return &RPCWorker{
		ID:                   id,
		rpcPool:              rpcPool,
		parser:               parser,
		workQueue:            workQueue,
		stopChan:             make(chan struct{}),
		wg:                   wg,
		stats:                &WorkStats{RPCWorkerID: id},
		watchedScripts:       make(map[string]*models.WatchedScript),
		scriptUpdateInterval: 5 * time.Minute, // Update watched scripts every 5 minutes
	}
}

// Start begins the RPC worker
func (w *RPCWorker) Start(ctx context.Context, heightChan <-chan int) {
	defer w.wg.Done()

	log.Printf("🚀 RPC Worker %d started", w.ID)

	// Initial watched scripts load
	if err := w.updateWatchedScripts(); err != nil {
		log.Printf("⚠️  RPC Worker %d: Failed to load initial watched scripts: %v", w.ID, err)
	}

	for {
		select {
		case height := <-heightChan:
			if err := w.processBlock(height); err != nil {
				log.Printf("❌ RPC Worker %d: Failed to process block %d: %v", w.ID, height, err)
				// Continue processing other blocks even if one fails
			}
		case <-w.stopChan:
			log.Printf("🛑 RPC Worker %d stopped", w.ID)
			return
		case <-ctx.Done():
			log.Printf("🛑 RPC Worker %d stopped (context cancelled)", w.ID)
			return
		}
	}
}

// Stop stops the RPC worker
func (w *RPCWorker) Stop() {
	close(w.stopChan)
}

// processBlock fetches a block and prepares it for DB processing
func (w *RPCWorker) processBlock(height int) error {
	startTime := time.Now()

	// Update watched scripts periodically
	if time.Since(w.lastScriptUpdate) > w.scriptUpdateInterval {
		if err := w.updateWatchedScripts(); err != nil {
			log.Printf("⚠️  RPC Worker %d: Failed to update watched scripts: %v", w.ID, err)
		}
	}

	// Get block hash
	blockHash, err := w.rpcPool.GetBlockHash(height)
	if err != nil {
		return fmt.Errorf("failed to get block hash for height %d: %w", height, err)
	}

	// Get block data
	block, err := w.rpcPool.GetBlock(blockHash)
	if err != nil {
		return fmt.Errorf("failed to get block data: %w", err)
	}

	// Parse block for UTXOs and transaction references
	newUTXOs, spentUTXOs, txReferences, err := w.parser.ParseBlockWithCache(block, w.watchedScripts)
	if err != nil {
		return fmt.Errorf("failed to parse block: %w", err)
	}

	// Create work item
	workItem := &WorkItem{
		Height:           height,
		BlockHash:        blockHash,
		Block:            block,
		NewUTXOs:         newUTXOs,
		SpentUTXOs:       spentUTXOs,
		TxReferences:     txReferences,
		WatchedScriptMap: w.watchedScripts,
		CreatedAt:        time.Now(),
		RPCWorkerID:      w.ID,
	}

	// Send work item to queue
	select {
	case w.workQueue <- workItem:
		// Successfully queued
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout sending work item to queue")
	}

	// Update stats
	w.mu.Lock()
	w.stats.BlocksProcessed++
	w.stats.TotalTime += time.Since(startTime)
	w.stats.AverageTime = w.stats.TotalTime / time.Duration(w.stats.BlocksProcessed)
	w.stats.LastProcessed = time.Now()
	w.mu.Unlock()

	// Log progress for large blocks
	if len(block.Transactions) > 100 {
		log.Printf("📦 RPC Worker %d: Prepared block %d (%d txs, %d new UTXOs, %d spent UTXOs) in %v",
			w.ID, height, len(block.Transactions), len(newUTXOs), len(spentUTXOs), time.Since(startTime))
	}

	return nil
}

// updateWatchedScripts updates the watched scripts cache
func (w *RPCWorker) updateWatchedScripts() error {
	// This would typically fetch from database, but for now we'll use empty map
	// In a real implementation, you'd call something like:
	// scripts, err := w.db.GetWatchedScriptsMap()
	// if err != nil {
	//     return err
	// }
	// w.watchedScripts = scripts

	w.lastScriptUpdate = time.Now()
	return nil
}

// GetStats returns the worker statistics
func (w *RPCWorker) GetStats() WorkStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return *w.stats
}
