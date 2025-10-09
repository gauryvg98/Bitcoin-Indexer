package workers

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"bitcoin-indexer/internal/bitcoin"
	"bitcoin-indexer/internal/database"
	"bitcoin-indexer/internal/parser"
)

// DBWorker handles database operations for processed blocks
type DBWorker struct {
	ID         int
	db         *database.DB
	parser     *parser.Parser
	workQueue  <-chan *WorkItem
	resultChan chan<- *WorkResult
	stopChan   chan struct{}
	wg         *sync.WaitGroup
	stats      *WorkStats
	mu         sync.RWMutex
}

// NewDBWorker creates a new database worker
func NewDBWorker(id int, db *database.DB, parser *parser.Parser, workQueue <-chan *WorkItem, resultChan chan<- *WorkResult, wg *sync.WaitGroup) *DBWorker {
	return &DBWorker{
		ID:         id,
		db:         db,
		parser:     parser,
		workQueue:  workQueue,
		resultChan: resultChan,
		stopChan:   make(chan struct{}),
		wg:         wg,
		stats:      &WorkStats{DBWorkerID: id},
	}
}

// Start begins the database worker
func (w *DBWorker) Start(ctx context.Context) {
	defer w.wg.Done()

	log.Printf("🗄️  DB Worker %d started", w.ID)

	for {
		select {
		case workItem := <-w.workQueue:
			if workItem == nil {
				// Channel closed
				log.Printf("🛑 DB Worker %d: Work queue closed", w.ID)
				return
			}

			result := w.processWorkItem(workItem)

			// Send result
			select {
			case w.resultChan <- result:
				// Successfully sent result
			case <-time.After(10 * time.Second):
				log.Printf("⚠️  DB Worker %d: Timeout sending result for block %d", w.ID, workItem.Height)
			}

		case <-w.stopChan:
			log.Printf("🛑 DB Worker %d stopped", w.ID)
			return
		case <-ctx.Done():
			log.Printf("🛑 DB Worker %d stopped (context cancelled)", w.ID)
			return
		}
	}
}

// Stop stops the database worker
func (w *DBWorker) Stop() {
	close(w.stopChan)
}

// processWorkItem processes a work item and returns the result
func (w *DBWorker) processWorkItem(workItem *WorkItem) *WorkResult {
	startTime := time.Now()

	result := &WorkResult{
		Height:      workItem.Height,
		BlockHash:   workItem.BlockHash,
		DBWorkerID:  w.ID,
		CompletedAt: time.Now(),
	}

	// Process the work item
	if err := w.processBlockData(workItem); err != nil {
		result.Success = false
		result.Error = err
		log.Printf("❌ DB Worker %d: Failed to process block %d: %v", w.ID, workItem.Height, err)
	} else {
		result.Success = true
	}

	result.ProcessTime = time.Since(startTime)

	// Update stats
	w.mu.Lock()
	w.stats.BlocksProcessed++
	w.stats.TotalTime += result.ProcessTime
	w.stats.AverageTime = w.stats.TotalTime / time.Duration(w.stats.BlocksProcessed)
	w.stats.LastProcessed = time.Now()
	w.mu.Unlock()

	// Log progress for large blocks
	if len(workItem.Block.Transactions) > 100 {
		log.Printf("✅ DB Worker %d: Processed block %d (%d txs, %d new UTXOs, %d spent UTXOs) in %v",
			w.ID, workItem.Height, len(workItem.Block.Transactions), len(workItem.NewUTXOs), len(workItem.SpentUTXOs), result.ProcessTime)
	}

	return result
}

// processBlockData processes all block data in a single database transaction
func (w *DBWorker) processBlockData(workItem *WorkItem) error {
	// Start a single database transaction for all operations
	tx, err := w.db.Conn().Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Set transaction isolation level for better performance
	_, err = tx.Exec("SET TRANSACTION ISOLATION LEVEL READ COMMITTED")
	if err != nil {
		return fmt.Errorf("failed to set transaction isolation level: %w", err)
	}

	// Convert []bitcoin.Tx to []*bitcoin.Tx
	txPointers := make([]*bitcoin.Tx, len(workItem.Block.Transactions))
	for idx, tx := range workItem.Block.Transactions {
		txPointers[idx] = &tx
	}

	// 1. Process transactions, inputs, and outputs in batch
	if len(txPointers) > 0 {
		start := time.Now()
		if err := w.parser.ProcessTransactionsBatchInTx(tx, txPointers, &workItem.Block.Height, &workItem.Block.Hash, &workItem.Block.Time); err != nil {
			return fmt.Errorf("failed to process transactions batch: %w", err)
		}
		log.Printf("   📊 DB Worker %d: Transactions processed in %v (%d txs)", w.ID, time.Since(start), len(txPointers))
	}

	// 2. Process UTXOs in batch
	if len(workItem.NewUTXOs) > 0 {
		start := time.Now()
		if err := w.db.BatchUpsertUTXOsInTx(tx, workItem.NewUTXOs); err != nil {
			return fmt.Errorf("failed to batch upsert UTXOs: %w", err)
		}
		log.Printf("   📊 DB Worker %d: New UTXOs processed in %v (%d UTXOs)", w.ID, time.Since(start), len(workItem.NewUTXOs))
	}

	if len(workItem.SpentUTXOs) > 0 {
		start := time.Now()
		if err := w.db.BatchMarkUTXOsSpentInTx(tx, workItem.SpentUTXOs); err != nil {
			return fmt.Errorf("failed to batch mark UTXOs as spent: %w", err)
		}
		log.Printf("   📊 DB Worker %d: Spent UTXOs processed in %v (%d UTXOs)", w.ID, time.Since(start), len(workItem.SpentUTXOs))
	}

	// 3. Process transaction references in batch
	if len(workItem.TxReferences) > 0 {
		start := time.Now()
		if err := w.db.BatchUpsertTransactionReferencesInTx(tx, workItem.TxReferences); err != nil {
			return fmt.Errorf("failed to batch upsert transaction references: %w", err)
		}
		log.Printf("   📊 DB Worker %d: Transaction references processed in %v (%d refs)", w.ID, time.Since(start), len(workItem.TxReferences))
	}

	// 4. Update index progress
	start := time.Now()
	if err := w.db.UpdateIndexProgressInTx(tx, workItem.Height, workItem.BlockHash); err != nil {
		return fmt.Errorf("failed to update index progress: %w", err)
	}
	log.Printf("   📊 DB Worker %d: Index progress updated in %v", w.ID, time.Since(start))

	// Commit the single transaction
	commitStart := time.Now()
	err = tx.Commit()
	commitDuration := time.Since(commitStart)
	log.Printf("   📊 DB Worker %d: Transaction committed in %v", w.ID, commitDuration)

	return err
}

// GetStats returns the worker statistics
func (w *DBWorker) GetStats() WorkStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return *w.stats
}
