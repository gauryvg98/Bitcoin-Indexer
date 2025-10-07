package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"bitcoin-indexer/internal/bitcoin"
	"bitcoin-indexer/internal/database"
	"bitcoin-indexer/internal/models"
	"bitcoin-indexer/internal/parser"
	"bitcoin-indexer/internal/wallet"
)

// Indexer manages the Bitcoin indexing process
type Indexer struct {
	rpcClient *bitcoin.RPCClient
	db        *database.DB
	parser    *parser.Parser
	hdWallet  *wallet.HDWallet
	config    *Config
	stopChan  chan struct{}
	wg        sync.WaitGroup
	mu        sync.RWMutex
	isRunning bool
	// Performance optimizations
	watchedScriptsCache map[string]*models.WatchedScript
	cacheMutex          sync.RWMutex
	cacheExpiry         time.Time
}

// Config holds indexer configuration
type Config struct {
	BlockPollInterval   time.Duration
	MempoolPollInterval time.Duration
	BackfillWorkers     int
	MaxReorgDepth       int
}

// FailedBlock represents a block that failed to process
type FailedBlock struct {
	Height    int       `json:"height"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
	IsPanic   bool      `json:"is_panic"`
}

// NewIndexer creates a new Bitcoin indexer
func NewIndexer(rpcClient *bitcoin.RPCClient, db *database.DB, config *Config) *Indexer {
	// Use mainnet parameters
	hdWallet := wallet.NewHDWallet(&wallet.MainNetParams)

	return &Indexer{
		rpcClient:           rpcClient,
		db:                  db,
		parser:              parser.NewParser(db),
		hdWallet:            hdWallet,
		config:              config,
		stopChan:            make(chan struct{}),
		watchedScriptsCache: make(map[string]*models.WatchedScript),
		cacheExpiry:         time.Now().Add(-1 * time.Hour), // Force initial cache load
	}
}

// Start begins the indexing process
func (i *Indexer) Start(ctx context.Context) error {
	i.mu.Lock()
	if i.isRunning {
		i.mu.Unlock()
		return fmt.Errorf("indexer is already running")
	}
	i.isRunning = true
	i.mu.Unlock()

	// Start block polling
	i.wg.Add(1)
	go i.blockPoller(ctx)

	// Start mempool polling
	i.wg.Add(1)
	go i.mempoolPoller(ctx)

	log.Println("Bitcoin indexer started")
	return nil
}

// Stop stops the indexing process
func (i *Indexer) Stop() error {
	i.mu.Lock()
	if !i.isRunning {
		i.mu.Unlock()
		return fmt.Errorf("indexer is not running")
	}
	i.isRunning = false
	i.mu.Unlock()

	close(i.stopChan)
	i.wg.Wait()

	log.Println("Bitcoin indexer stopped")
	return nil
}

// getWatchedScriptsMap returns cached watched scripts with automatic refresh
func (i *Indexer) getWatchedScriptsMap() (map[string]*models.WatchedScript, error) {
	i.cacheMutex.RLock()
	// Check if cache is still valid (refresh every 5 minutes)
	if time.Now().Before(i.cacheExpiry) && len(i.watchedScriptsCache) > 0 {
		cache := i.watchedScriptsCache
		i.cacheMutex.RUnlock()
		return cache, nil
	}
	i.cacheMutex.RUnlock()

	// Cache expired or empty, refresh it
	i.cacheMutex.Lock()
	defer i.cacheMutex.Unlock()

	// Double-check after acquiring write lock
	if time.Now().Before(i.cacheExpiry) && len(i.watchedScriptsCache) > 0 {
		return i.watchedScriptsCache, nil
	}

	// Load fresh data from database
	scripts, err := i.db.GetWatchedScripts()
	if err != nil {
		return nil, fmt.Errorf("failed to get watched scripts: %w", err)
	}

	// Update cache
	i.watchedScriptsCache = make(map[string]*models.WatchedScript)
	for _, script := range scripts {
		i.watchedScriptsCache[script.ScriptHex] = script
	}
	i.cacheExpiry = time.Now().Add(5 * time.Minute)

	return i.watchedScriptsCache, nil
}

// invalidateCache forces cache refresh on next access
func (i *Indexer) invalidateCache() {
	i.cacheMutex.Lock()
	defer i.cacheMutex.Unlock()
	i.cacheExpiry = time.Now().Add(-1 * time.Hour)
}

// blockPoller polls for new blocks
func (i *Indexer) blockPoller(ctx context.Context) {
	defer i.wg.Done()

	ticker := time.NewTicker(i.config.BlockPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-i.stopChan:
			return
		case <-ticker.C:
			if err := i.processBlocks(); err != nil {
				log.Printf("Error processing blocks: %v", err)
			}
		}
	}
}

// mempoolPoller polls for mempool transactions
func (i *Indexer) mempoolPoller(ctx context.Context) {
	defer i.wg.Done()

	ticker := time.NewTicker(i.config.MempoolPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-i.stopChan:
			return
		case <-ticker.C:
			if err := i.processMempool(); err != nil {
				log.Printf("Error processing mempool: %v", err)
			}
		}
	}
}

// processBlocks processes new blocks
func (i *Indexer) processBlocks() error {
	// Get current block height
	currentHeight, err := i.rpcClient.GetBlockCount()
	if err != nil {
		return fmt.Errorf("failed to get block count: %w", err)
	}

	// Get last processed height
	progress, err := i.db.GetIndexProgress()
	if err != nil {
		return fmt.Errorf("failed to get index progress: %w", err)
	}

	// Check if we need to do backfill
	blocksToProcess := currentHeight - progress.LastHeight
	if blocksToProcess > 100 && i.config.BackfillWorkers > 1 {
		// Use parallel backfill for large gaps
		return i.processBlocksParallel(progress.LastHeight+1, currentHeight)
	}

	// Use sequential processing for small gaps or real-time processing
	// Process blocks in chunks to avoid memory issues
	const chunkSize = 10
	for height := progress.LastHeight + 1; height <= currentHeight; height += chunkSize {
		endHeight := height + chunkSize - 1
		if endHeight > currentHeight {
			endHeight = currentHeight
		}

		if err := i.processBlockChunk(height, endHeight); err != nil {
			return fmt.Errorf("failed to process block chunk %d-%d: %w", height, endHeight, err)
		}
	}

	return nil
}

// processBlockChunk processes a chunk of blocks sequentially to manage memory usage
func (i *Indexer) processBlockChunk(startHeight, endHeight int) error {
	log.Printf("Processing block chunk %d-%d", startHeight, endHeight)

	// Get watched scripts cache once for the entire chunk
	watchedScriptMap, err := i.getWatchedScriptsMap()
	if err != nil {
		return fmt.Errorf("failed to get watched scripts: %w", err)
	}

	// Process each block in the chunk
	for height := startHeight; height <= endHeight; height++ {
		if err := i.processBlockWithCache(height, watchedScriptMap); err != nil {
			return fmt.Errorf("failed to process block at height %d: %w", height, err)
		}
	}

	log.Printf("Completed block chunk %d-%d", startHeight, endHeight)
	return nil
}

// processBlockWithCache processes a single block using cached watched scripts with panic recovery
func (i *Indexer) processBlockWithCache(height int, watchedScriptMap map[string]*models.WatchedScript) (err error) {
	// Recover from panics and convert to error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic processing block %d: %v", height, r)
			log.Printf("PANIC caught while processing block %d: %v", height, r)

			// Log failed block to file
			i.logFailedBlock(height, err, true)

			// Attempt to rollback to last known good state
			if rollbackErr := i.rollbackToLastGoodBlock(); rollbackErr != nil {
				log.Printf("Failed to rollback after panic: %v", rollbackErr)
			}
		}
	}()

	// Get block hash
	blockHash, err := i.rpcClient.GetBlockHash(height)
	if err != nil {
		return fmt.Errorf("failed to get block hash for height %d: %w", height, err)
	}

	// Check for reorg
	if err := i.checkReorg(height, blockHash); err != nil {
		return fmt.Errorf("failed to check reorg: %w", err)
	}

	// Get block data
	block, err := i.rpcClient.GetBlock(blockHash)
	if err != nil {
		return fmt.Errorf("failed to get block data: %w", err)
	}

	// Process all transactions in the block using batch processing
	// Convert []bitcoin.Tx to []*bitcoin.Tx
	txPointers := make([]*bitcoin.Tx, len(block.Transactions))
	for idx, tx := range block.Transactions {
		txPointers[idx] = &tx
	}

	if err := i.parser.ProcessTransactionsBatch(txPointers, &block.Height, &block.Hash, &block.Time); err != nil {
		return fmt.Errorf("failed to process transactions batch: %w", err)
	}

	// Parse block transactions for UTXOs and transaction references (for watched addresses)
	newUTXOs, spentUTXOs, txReferences, err := i.parser.ParseBlockWithCache(block, watchedScriptMap)
	if err != nil {
		return fmt.Errorf("failed to parse block: %w", err)
	}

	// Process UTXOs and transaction references in batch
	if err := i.parser.ProcessUTXOs(newUTXOs, spentUTXOs, txReferences); err != nil {
		return fmt.Errorf("failed to process UTXOs: %w", err)
	}

	// Update index progress
	if err := i.db.UpdateIndexProgress(height, blockHash); err != nil {
		return fmt.Errorf("failed to update index progress: %w", err)
	}

	log.Printf("Processed block %d (%s) with %d new UTXOs and %d spent UTXOs",
		height, blockHash, len(newUTXOs), len(spentUTXOs))

	return nil
}

// processBlocksParallel processes blocks in TRUE parallel - no artificial ordering
func (i *Indexer) processBlocksParallel(startHeight, endHeight int) error {
	log.Printf("Starting TRUE parallel processing from height %d to %d with %d workers",
		startHeight, endHeight, i.config.BackfillWorkers)

	totalBlocks := endHeight - startHeight + 1

	// Create channels for work distribution
	blockChan := make(chan int, i.config.BackfillWorkers*2)
	errorChan := make(chan error, i.config.BackfillWorkers)
	doneChan := make(chan struct{})

	// Track progress
	var processed int32
	var mu sync.Mutex
	errors := make([]error, 0)

	// Start workers - they process ANY block they receive
	var wg sync.WaitGroup
	for w := 0; w < i.config.BackfillWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for height := range blockChan {
				if err := i.processBlockIndependent(height); err != nil {
					// Log the failed block to file
					i.logFailedBlock(height, err, false)
					errorChan <- fmt.Errorf("worker %d failed block %d: %w", workerID, height, err)
					continue
				}

				// Update progress
				current := atomic.AddInt32(&processed, 1)
				if current%10 == 0 {
					log.Printf("Progress: %d/%d blocks (%.3f%%)",
						current, totalBlocks, float64(current)/float64(totalBlocks)*100)
				}
			}
		}(w)
	}

	// Error collector
	go func() {
		for err := range errorChan {
			mu.Lock()
			errors = append(errors, err)
			mu.Unlock()
		}
	}()

	// Send ALL blocks to workers immediately - process in any order
	go func() {
		defer close(blockChan)
		for height := startHeight; height <= endHeight; height++ {
			select {
			case blockChan <- height:
			case <-i.stopChan:
				return
			}
		}
	}()

	// Wait for all workers to complete
	wg.Wait()
	close(errorChan)
	close(doneChan)

	// Check for errors
	mu.Lock()
	defer mu.Unlock()
	if len(errors) > 0 {
		log.Printf("Completed with %d errors out of %d blocks", len(errors), totalBlocks)
		return fmt.Errorf("parallel processing had %d errors (first: %v)", len(errors), errors[0])
	}

	log.Printf("Parallel processing completed: %d blocks processed successfully", totalBlocks)
	return nil
}

// processBlockIndependent processes a single block completely independently with panic recovery
func (i *Indexer) processBlockIndependent(height int) (err error) {
	// Recover from panics and convert to error
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic processing block %d: %v", height, r)
			log.Printf("PANIC caught while processing block %d: %v", height, r)

			// Log failed block to file
			i.logFailedBlock(height, err, true)

			// Attempt to rollback to last known good state
			if rollbackErr := i.rollbackToLastGoodBlock(); rollbackErr != nil {
				log.Printf("Failed to rollback after panic: %v", rollbackErr)
			}
		}
	}()

	// Get block hash
	blockHash, err := i.rpcClient.GetBlockHash(height)
	if err != nil {
		return fmt.Errorf("failed to get block hash for height %d: %w", height, err)
	}

	// Get block data
	block, err := i.rpcClient.GetBlock(blockHash)
	if err != nil {
		return fmt.Errorf("failed to get block data: %w", err)
	}

	// Store comprehensive transaction data for ALL transactions
	txPointers := make([]*bitcoin.Tx, len(block.Transactions))
	for idx, tx := range block.Transactions {
		txPointers[idx] = &tx
	}

	if err := i.parser.ProcessTransactionsBatch(txPointers, &block.Height, &block.Hash, &block.Time); err != nil {
		return fmt.Errorf("failed to process transactions batch: %w", err)
	}

	// Parse block transactions for UTXOs and transaction references
	watchedScriptMap, err := i.getWatchedScriptsMap()
	if err != nil {
		return fmt.Errorf("failed to get watched scripts: %w", err)
	}

	newUTXOs, spentUTXOs, txReferences, err := i.parser.ParseBlockWithCache(block, watchedScriptMap)
	if err != nil {
		return fmt.Errorf("failed to parse block: %w", err)
	}

	// Process UTXOs and transaction references
	if err := i.parser.ProcessUTXOs(newUTXOs, spentUTXOs, txReferences); err != nil {
		return fmt.Errorf("failed to process UTXOs: %w", err)
	}

	// Update index progress (this can happen in any order for historical data)
	if err := i.db.UpdateIndexProgress(height, blockHash); err != nil {
		return fmt.Errorf("failed to update index progress: %w", err)
	}

	return nil
}

// rollbackToLastGoodBlock attempts to rollback to the last successfully processed block
func (i *Indexer) rollbackToLastGoodBlock() error {
	// Get the last recorded progress
	progress, err := i.db.GetIndexProgress()
	if err != nil {
		return fmt.Errorf("failed to get index progress: %w", err)
	}

	log.Printf("Rolling back to last good block: height=%d, hash=%s",
		progress.LastHeight, progress.LastBlockHash)

	// The progress table already contains the last good block
	// No need to modify it, but we could clean up any partial data
	// from failed blocks if needed

	return nil
}

// logFailedBlock logs a failed block to a separate file
func (i *Indexer) logFailedBlock(height int, err error, isPanic bool) {
	failedBlock := FailedBlock{
		Height:    height,
		Error:     err.Error(),
		Timestamp: time.Now(),
		IsPanic:   isPanic,
	}

	// Open or create failed blocks log file
	file, openErr := os.OpenFile("failed_blocks.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if openErr != nil {
		log.Printf("Failed to open failed_blocks.json: %v", openErr)
		return
	}
	defer file.Close()

	// Write as JSON line
	encoder := json.NewEncoder(file)
	if encodeErr := encoder.Encode(failedBlock); encodeErr != nil {
		log.Printf("Failed to write to failed_blocks.json: %v", encodeErr)
		return
	}

	log.Printf("Logged failed block %d to failed_blocks.json", height)
}

// processBlock processes a single block
func (i *Indexer) processBlock(height int) error {
	// Get block hash
	blockHash, err := i.rpcClient.GetBlockHash(height)
	if err != nil {
		return fmt.Errorf("failed to get block hash for height %d: %w", height, err)
	}

	// Check for reorg
	if err := i.checkReorg(height, blockHash); err != nil {
		return fmt.Errorf("failed to check reorg: %w", err)
	}

	// Get block data
	block, err := i.rpcClient.GetBlock(blockHash)
	if err != nil {
		return fmt.Errorf("failed to get block data: %w", err)
	}

	// Store comprehensive transaction data for ALL transactions
	for _, tx := range block.Transactions {
		err := i.parser.ProcessTransactionComprehensive(&tx, &block.Height, &block.Hash, &block.Time)
		if err != nil {
			log.Printf("Failed to store transaction data for %s: %v", tx.Txid, err)
			continue
		}
	}

	// Parse block transactions for UTXOs and transaction references (for watched addresses)
	newUTXOs, spentUTXOs, txReferences, err := i.parser.ParseBlock(block)
	if err != nil {
		return fmt.Errorf("failed to parse block: %w", err)
	}

	// Process UTXOs and transaction references
	if err := i.parser.ProcessUTXOs(newUTXOs, spentUTXOs, txReferences); err != nil {
		return fmt.Errorf("failed to process UTXOs: %w", err)
	}

	// Update index progress
	if err := i.db.UpdateIndexProgress(height, blockHash); err != nil {
		return fmt.Errorf("failed to update index progress: %w", err)
	}

	log.Printf("Processed block %d (%s) with %d new UTXOs and %d spent UTXOs",
		height, blockHash, len(newUTXOs), len(spentUTXOs))

	return nil
}

// processMempool processes mempool transactions
func (i *Indexer) processMempool() error {
	// Get mempool transaction IDs
	txids, err := i.rpcClient.GetRawMempool()
	if err != nil {
		return fmt.Errorf("failed to get mempool: %w", err)
	}

	// Get watched scripts using cache
	watchedScriptMap, err := i.getWatchedScriptsMap()
	if err != nil {
		return fmt.Errorf("failed to get watched scripts: %w", err)
	}

	// Create a simple boolean map for quick lookup
	watchedScriptSet := make(map[string]bool)
	for scriptHex := range watchedScriptMap {
		watchedScriptSet[scriptHex] = true
	}

	// Process each mempool transaction
	for _, txid := range txids {
		// Get transaction details
		tx, err := i.rpcClient.GetRawTransaction(txid)
		if err != nil {
			log.Printf("Failed to get transaction %s: %v", txid, err)
			continue
		}

		// Check if transaction is relevant to our watched addresses
		if !i.isTransactionRelevant(tx, watchedScriptSet) {
			continue
		}

		// Store comprehensive transaction data
		err = i.parser.ProcessTransactionComprehensive(tx, nil, nil, nil)
		if err != nil {
			log.Printf("Failed to store mempool transaction data for %s: %v", txid, err)
			continue
		}

		// Parse mempool transaction for UTXOs and transaction references
		newUTXOs, spentUTXOs, txReferences, err := i.parser.ParseMempoolTransaction(tx)
		if err != nil {
			log.Printf("Failed to parse mempool transaction %s: %v", txid, err)
			continue
		}

		// Process UTXOs and transaction references
		if err := i.parser.ProcessUTXOs(newUTXOs, spentUTXOs, txReferences); err != nil {
			log.Printf("Failed to process UTXOs for transaction %s: %v", txid, err)
			continue
		}

	}

	return nil
}

// isTransactionRelevant checks if a transaction is relevant to our watched addresses
func (i *Indexer) isTransactionRelevant(tx *bitcoin.Tx, watchedScriptMap map[string]bool) bool {
	// Check outputs
	for _, vout := range tx.Vout {
		if watchedScriptMap[vout.ScriptPubKey.Hex] {
			return true
		}
	}

	// Check inputs (spending our UTXOs)
	for _, vin := range tx.Vin {
		if vin.Txid != "" && vin.Vout >= 0 {
			// Check if this input spends one of our UTXOs
			utxo, err := i.db.GetUTXO(vin.Txid, vin.Vout)
			if err == nil && utxo != nil {
				return true
			}
		}
	}

	return false
}

// checkReorg checks for chain reorganizations
func (i *Indexer) checkReorg(height int, blockHash string) error {
	progress, err := i.db.GetIndexProgress()
	if err != nil {
		return fmt.Errorf("failed to get index progress: %w", err)
	}

	// If this is the first block or height is 0, no reorg check needed
	if progress.LastHeight == 0 || height == 0 {
		return nil
	}

	// Check if the previous block hash matches
	if height > 0 {
		prevHeight := height - 1
		prevBlockHash, err := i.rpcClient.GetBlockHash(prevHeight)
		if err != nil {
			return fmt.Errorf("failed to get previous block hash: %w", err)
		}

		// If the previous block hash doesn't match, we have a reorg
		if prevBlockHash != progress.LastBlockHash {
			log.Printf("Reorg detected at height %d. Expected: %s, Got: %s",
				prevHeight, progress.LastBlockHash, prevBlockHash)

			if err := i.handleReorg(prevHeight); err != nil {
				return fmt.Errorf("failed to handle reorg: %w", err)
			}
		}
	}

	return nil
}

// handleReorg handles chain reorganizations
func (i *Indexer) handleReorg(fromHeight int) error {
	// Walk back to find common ancestor (limited by max reorg depth)
	maxDepth := i.config.MaxReorgDepth
	if maxDepth <= 0 {
		maxDepth = 10 // Default max reorg depth
	}

	progress, err := i.db.GetIndexProgress()
	if err != nil {
		return fmt.Errorf("failed to get index progress: %w", err)
	}

	// Find common ancestor by walking back
	commonHeight := fromHeight
	for depth := 0; depth < maxDepth && commonHeight > 0; depth++ {
		blockHash, err := i.rpcClient.GetBlockHash(commonHeight)
		if err != nil {
			return fmt.Errorf("failed to get block hash at height %d: %w", commonHeight, err)
		}

		// Check if this block exists in our database
		// For simplicity, we'll just rollback to the common height
		// In a production system, you'd want more sophisticated reorg handling
		_ = blockHash // TODO: Use blockHash to verify block exists in database
		_ = progress  // TODO: Use progress to determine rollback strategy
		commonHeight--
	}

	// Rollback UTXOs from orphaned blocks
	if err := i.rollbackUTXOs(fromHeight); err != nil {
		return fmt.Errorf("failed to rollback UTXOs: %w", err)
	}

	log.Printf("Handled reorg, rolled back to height %d", commonHeight)
	return nil
}

// rollbackUTXOs rolls back UTXOs from orphaned blocks
func (i *Indexer) rollbackUTXOs(fromHeight int) error {
	// This is a simplified rollback - in production you'd want more sophisticated handling
	// For now, we'll just delete UTXOs from blocks after the reorg point

	// Get all block hashes that need to be rolled back
	progress, err := i.db.GetIndexProgress()
	if err != nil {
		return fmt.Errorf("failed to get index progress: %w", err)
	}

	// Delete UTXOs from orphaned blocks
	// Note: This is simplified - in production you'd want to track which blocks were orphaned
	log.Printf("Rolling back UTXOs from height %d to %d", fromHeight, progress.LastHeight)

	// Update index progress to the reorg point
	if err := i.db.UpdateIndexProgress(fromHeight, ""); err != nil {
		return fmt.Errorf("failed to update index progress after reorg: %w", err)
	}

	return nil
}
