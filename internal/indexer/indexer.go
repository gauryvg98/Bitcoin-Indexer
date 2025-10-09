package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"bitcoin-indexer/internal/bitcoin"
	"bitcoin-indexer/internal/database"
	"bitcoin-indexer/internal/models"
	"bitcoin-indexer/internal/parser"
	"bitcoin-indexer/internal/wallet"
	"bitcoin-indexer/internal/workers"
)

// Indexer manages the Bitcoin indexing process
type Indexer struct {
	rpcClient *bitcoin.RPCClient
	rpcPool   *bitcoin.RPCPool
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
	// Metrics collection
	metricsCollector *MetricsCollector
	// Backfill mode flag
	inBackfillMode bool
	// Worker pool for optimized processing
	workerPool *workers.WorkerPool
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

// BlockMetrics tracks performance metrics for a single block
type BlockMetrics struct {
	Height           int           `json:"height"`
	BlockHash        string        `json:"block_hash"`
	TotalTime        time.Duration `json:"total_time_ms"`
	RPCHashTime      time.Duration `json:"rpc_hash_time_ms"`
	RPCBlockTime     time.Duration `json:"rpc_block_time_ms"`
	StoreTXTime      time.Duration `json:"store_tx_time_ms"`
	ParseTime        time.Duration `json:"parse_time_ms"`
	ProcessUTXOTime  time.Duration `json:"process_utxo_time_ms"`
	ProgressTime     time.Duration `json:"progress_time_ms"`
	TransactionCount int           `json:"transaction_count"`
	InputCount       int           `json:"input_count"`
	OutputCount      int           `json:"output_count"`
	NewUTXOCount     int           `json:"new_utxo_count"`
	SpentUTXOCount   int           `json:"spent_utxo_count"`
	Timestamp        time.Time     `json:"timestamp"`
}

// RangeMetrics tracks aggregated metrics for a block range
type RangeMetrics struct {
	RangeStart             int           `json:"range_start"`
	RangeEnd               int           `json:"range_end"`
	BlockCount             int           `json:"block_count"`
	TotalTime              time.Duration `json:"total_time_ms"`
	AverageBlockTime       time.Duration `json:"avg_block_time_ms"`
	BlocksPerSecond        float64       `json:"blocks_per_second"`
	TotalTXs               int           `json:"total_transactions"`
	TotalInputs            int           `json:"total_inputs"`
	TotalOutputs           int           `json:"total_outputs"`
	TotalNewUTXOs          int           `json:"total_new_utxos"`
	TotalSpentUTXOs        int           `json:"total_spent_utxos"`
	AverageTXsPerBlock     float64       `json:"avg_txs_per_block"`
	AverageInputsPerBlock  float64       `json:"avg_inputs_per_block"`
	AverageOutputsPerBlock float64       `json:"avg_outputs_per_block"`
	// Accumulated timing for proper percentage calculation
	TotalRPCTime     time.Duration `json:"total_rpc_time_ms"`
	TotalDBTime      time.Duration `json:"total_db_time_ms"`
	TotalParseTime   time.Duration `json:"total_parse_time_ms"`
	RPCTimePercent   float64       `json:"rpc_time_percent"`
	DBTimePercent    float64       `json:"db_time_percent"`
	ParseTimePercent float64       `json:"parse_time_percent"`
	LastUpdated      time.Time     `json:"last_updated"`
}

// MetricsCollector manages performance metrics collection
type MetricsCollector struct {
	blockMetrics  []BlockMetrics
	rangeMetrics  map[int]*RangeMetrics // Key: range start (e.g., 0, 10000, 20000)
	mu            sync.RWMutex
	metricsFile   string
	rangeInterval int
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(metricsFile string, rangeInterval int) *MetricsCollector {
	if rangeInterval <= 0 {
		rangeInterval = 10000 // Default to 10k block ranges
	}

	return &MetricsCollector{
		blockMetrics:  make([]BlockMetrics, 0),
		rangeMetrics:  make(map[int]*RangeMetrics),
		metricsFile:   metricsFile,
		rangeInterval: rangeInterval,
	}
}

// NewIndexer creates a new Bitcoin indexer
func NewIndexer(rpcClient *bitcoin.RPCClient, db *database.DB, config *Config) *Indexer {
	// Use mainnet parameters
	hdWallet := wallet.NewHDWallet(&wallet.MainNetParams)

	// Dynamic RPC pool sizing based on worker count
	rpcPoolSize := getIntEnv("RPC_POOL_SIZE", config.BackfillWorkers/10) // 1 RPC client per 10 workers
	if rpcPoolSize < 5 {
		rpcPoolSize = 5 // Minimum pool size
	}
	if rpcPoolSize > 1000 {
		rpcPoolSize = 1000 // Maximum pool size to avoid overwhelming Bitcoin node
	}

	// Create RPC pool for parallel processing
	rpcPool := bitcoin.NewRPCPool(rpcClient.BaseURL(), rpcClient.Username(), rpcClient.Password(), rpcPoolSize)

	// Create metrics collector
	metricsCollector := NewMetricsCollector("indexer_metrics.json", 10000)

	// Create worker pool configuration
	workerConfig := workers.WorkerConfig{
		RPCWorkers:    10,   // 10 RPC workers for fetching blocks
		DBWorkers:     10,   // 10 DB workers for processing blocks
		QueueSize:     1000, // Queue size for work items
		WorkerTimeout: 30 * time.Second,
	}

	// Create worker pool
	workerPool := workers.NewWorkerPool(workerConfig)

	// Initialize worker pool with dependencies
	if err := workerPool.Initialize(rpcPool, db, parser.NewParser(db)); err != nil {
		log.Printf("⚠️  Failed to initialize worker pool: %v", err)
		workerPool = nil // Fallback to single-threaded processing
	}

	return &Indexer{
		rpcClient:           rpcClient,
		rpcPool:             rpcPool,
		db:                  db,
		parser:              parser.NewParser(db),
		hdWallet:            hdWallet,
		config:              config,
		stopChan:            make(chan struct{}),
		watchedScriptsCache: make(map[string]*models.WatchedScript),
		cacheExpiry:         time.Now().Add(-1 * time.Hour), // Force initial cache load
		metricsCollector:    metricsCollector,
		workerPool:          workerPool,
	}
}

// AddBlockMetrics adds metrics for a processed block (range metrics only)
func (mc *MetricsCollector) AddBlockMetrics(metrics BlockMetrics) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Skip storing individual block metrics - only update range metrics
	rangeStart := (metrics.Height / mc.rangeInterval) * mc.rangeInterval
	if mc.rangeMetrics[rangeStart] == nil {
		mc.rangeMetrics[rangeStart] = &RangeMetrics{
			RangeStart:      rangeStart,
			RangeEnd:        rangeStart + mc.rangeInterval - 1,
			BlockCount:      0,
			TotalTime:       0,
			TotalTXs:        0,
			TotalInputs:     0,
			TotalOutputs:    0,
			TotalNewUTXOs:   0,
			TotalSpentUTXOs: 0,
			TotalRPCTime:    0,
			TotalDBTime:     0,
			TotalParseTime:  0,
			LastUpdated:     time.Now(),
		}
	}

	rangeMetrics := mc.rangeMetrics[rangeStart]
	rangeMetrics.BlockCount++
	rangeMetrics.TotalTime += metrics.TotalTime
	rangeMetrics.TotalTXs += metrics.TransactionCount
	rangeMetrics.TotalInputs += metrics.InputCount
	rangeMetrics.TotalOutputs += metrics.OutputCount
	rangeMetrics.TotalNewUTXOs += metrics.NewUTXOCount
	rangeMetrics.TotalSpentUTXOs += metrics.SpentUTXOCount

	// Accumulate timing for proper percentage calculation
	rangeMetrics.TotalRPCTime += metrics.RPCHashTime + metrics.RPCBlockTime
	rangeMetrics.TotalDBTime += metrics.StoreTXTime + metrics.ProcessUTXOTime
	rangeMetrics.TotalParseTime += metrics.ParseTime

	rangeMetrics.LastUpdated = time.Now()

	// Recalculate averages
	if rangeMetrics.BlockCount > 0 {
		rangeMetrics.AverageBlockTime = rangeMetrics.TotalTime / time.Duration(rangeMetrics.BlockCount)
		rangeMetrics.BlocksPerSecond = float64(rangeMetrics.BlockCount) / rangeMetrics.TotalTime.Seconds()
		rangeMetrics.AverageTXsPerBlock = float64(rangeMetrics.TotalTXs) / float64(rangeMetrics.BlockCount)
		rangeMetrics.AverageInputsPerBlock = float64(rangeMetrics.TotalInputs) / float64(rangeMetrics.BlockCount)
		rangeMetrics.AverageOutputsPerBlock = float64(rangeMetrics.TotalOutputs) / float64(rangeMetrics.BlockCount)

		// Calculate time percentages using accumulated timing
		totalTime := float64(rangeMetrics.TotalTime.Nanoseconds())
		if totalTime > 0 {
			totalRPCTime := float64(rangeMetrics.TotalRPCTime.Nanoseconds())
			totalDBTime := float64(rangeMetrics.TotalDBTime.Nanoseconds())
			totalParseTime := float64(rangeMetrics.TotalParseTime.Nanoseconds())

			rangeMetrics.RPCTimePercent = (totalRPCTime / totalTime) * 100
			rangeMetrics.DBTimePercent = (totalDBTime / totalTime) * 100
			rangeMetrics.ParseTimePercent = (totalParseTime / totalTime) * 100
		}
	}

	// Log range metrics only at range boundaries
	if (metrics.Height+1)%mc.rangeInterval == 0 {
		mc.logRangeMetrics(rangeStart)
		// Save metrics asynchronously at range boundaries to avoid blocking
		go func() {
			if err := mc.SaveMetrics(); err != nil {
				log.Printf("Failed to save metrics at range boundary: %v", err)
			}
		}()
	}
}

// logRangeMetrics logs metrics for a specific range
func (mc *MetricsCollector) logRangeMetrics(rangeStart int) {
	rangeMetrics := mc.rangeMetrics[rangeStart]
	if rangeMetrics == nil {
		return
	}

	log.Printf("Range %d-%d: %.2f blocks/sec, avg %.1f TXs/block",
		rangeMetrics.RangeStart,
		rangeMetrics.RangeEnd,
		rangeMetrics.BlocksPerSecond,
		rangeMetrics.AverageTXsPerBlock)
}

// SaveMetrics saves metrics to file
func (mc *MetricsCollector) SaveMetrics() error {
	return mc.saveMetricsInternal(false)
}

// saveMetricsInternal saves metrics with optional lock control (range metrics only)
func (mc *MetricsCollector) saveMetricsInternal(lockHeld bool) error {
	if !lockHeld {
		mc.mu.RLock()
		defer mc.mu.RUnlock()
	}

	data := map[string]interface{}{
		"range_metrics":  mc.rangeMetrics,
		"generated_at":   time.Now(),
		"range_interval": mc.rangeInterval,
		"note":           "Individual block metrics disabled to reduce file size",
	}

	file, err := os.Create(mc.metricsFile)
	if err != nil {
		return fmt.Errorf("failed to create metrics file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// GetRangeMetrics returns metrics for a specific range
func (mc *MetricsCollector) GetRangeMetrics(rangeStart int) *RangeMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.rangeMetrics[rangeStart]
}

// GetAllRangeMetrics returns all range metrics
func (mc *MetricsCollector) GetAllRangeMetrics() map[int]*RangeMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[int]*RangeMetrics)
	for k, v := range mc.rangeMetrics {
		result[k] = v
	}
	return result
}

// PrintSummary prints a summary of all range metrics
func (mc *MetricsCollector) PrintSummary() {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	log.Println("📊 INDEXER PERFORMANCE SUMMARY (Range Metrics Only)")
	log.Println("==================================================")

	totalBlocks := 0
	totalTime := time.Duration(0)
	totalTXs := 0

	for rangeStart := 0; rangeStart < 1000000; rangeStart += mc.rangeInterval {
		rangeMetrics := mc.rangeMetrics[rangeStart]
		if rangeMetrics == nil {
			continue
		}

		totalBlocks += rangeMetrics.BlockCount
		totalTime += rangeMetrics.TotalTime
		totalTXs += rangeMetrics.TotalTXs

		log.Printf("Range %d-%d: %d blocks, %.2f blocks/sec, %.1fms avg, %.1f TXs/block",
			rangeMetrics.RangeStart,
			rangeMetrics.RangeEnd,
			rangeMetrics.BlockCount,
			rangeMetrics.BlocksPerSecond,
			float64(rangeMetrics.AverageBlockTime.Nanoseconds())/1e6,
			rangeMetrics.AverageTXsPerBlock)
	}

	if totalBlocks > 0 {
		avgBlocksPerSec := float64(totalBlocks) / totalTime.Seconds()
		avgTXsPerBlock := float64(totalTXs) / float64(totalBlocks)

		log.Printf("OVERALL: %d blocks, %.2f blocks/sec, %.1f TXs/block",
			totalBlocks, avgBlocksPerSec, avgTXsPerBlock)
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

	// // Start mempool polling
	// i.wg.Add(1)
	// go i.mempoolPoller(ctx)

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

	// Close RPC pool
	if i.rpcPool != nil {
		i.rpcPool.Close()
	}

	// Save metrics before stopping
	if i.metricsCollector != nil {
		if err := i.metricsCollector.SaveMetrics(); err != nil {
			log.Printf("Failed to save metrics: %v", err)
		} else {
			log.Println("Metrics saved to indexer_metrics.json")
		}
		i.metricsCollector.PrintSummary()
	}

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
	if blocksToProcess > 1000 && i.config.BackfillWorkers > 1 {
		// Use worker pool for backfill operations (1000+ blocks)
		log.Printf("🔄 Starting backfill processing for %d blocks using worker pool", blocksToProcess)
		return i.ProcessBlocksWithWorkerPool(progress.LastHeight+1, currentHeight)
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

// processBlockChunk processes a chunk of blocks one at a time
func (i *Indexer) processBlockChunk(startHeight, endHeight int) error {
	log.Printf("Processing block chunk %d-%d (one block at a time)", startHeight, endHeight)

	// Get watched scripts cache once for the entire chunk
	watchedScriptMap, err := i.getWatchedScriptsMap()
	if err != nil {
		return fmt.Errorf("failed to get watched scripts: %w", err)
	}

	// Process each block individually
	for height := startHeight; height <= endHeight; height++ {
		if err := i.processBlockWithCache(height, watchedScriptMap); err != nil {
			return fmt.Errorf("failed to process block at height %d: %w", height, err)
		}
	}

	return nil
}

// processBlocksBulk processes multiple blocks using true bulk operations for maximum performance
func (i *Indexer) processBlocksBulk(startHeight, endHeight int, watchedScriptMap map[string]*models.WatchedScript) error {
	// Collect all block data first
	var allBlocks []*bitcoin.Block
	var allBlockHashes []string
	blockHeightMap := make(map[string]int) // blockHash -> height

	// Fetch all block data in parallel
	for height := startHeight; height <= endHeight; height++ {
		blockHash, err := i.rpcPool.GetBlockHash(height)
		if err != nil {
			return fmt.Errorf("failed to get block hash for height %d: %w", height, err)
		}

		block, err := i.rpcPool.GetBlock(blockHash)
		if err != nil {
			return fmt.Errorf("failed to get block data for height %d: %w", height, err)
		}

		allBlocks = append(allBlocks, block)
		allBlockHashes = append(allBlockHashes, blockHash)
		blockHeightMap[blockHash] = height
	}

	// Process all blocks in true bulk operations
	return i.processBlocksBulkData(allBlocks, allBlockHashes, blockHeightMap, watchedScriptMap)
}

// processBlocksBulkData processes multiple blocks using true bulk database operations
func (i *Indexer) processBlocksBulkData(blocks []*bitcoin.Block, blockHashes []string, blockHeightMap map[string]int, watchedScriptMap map[string]*models.WatchedScript) error {
	startTime := time.Now()
	log.Printf("🔄 Starting bulk processing of %d blocks (heights %d-%d)",
		len(blocks), blockHeightMap[blockHashes[0]], blockHeightMap[blockHashes[len(blockHashes)-1]])

	// Collect all data from all blocks
	var allTransactions []*models.Transaction
	var allInputs []*models.TransactionInput
	var allOutputs []*models.TransactionOutput
	var allNewUTXOs []*models.UTXO
	var allSpentUTXOs []*models.UTXO
	var allTxReferences []*models.TransactionReference

	// Collect all UTXO references needed for bulk lookup
	var allUTXORefs []database.UTXORef
	utxoRefMap := make(map[string]int)

	// First pass: collect all UTXO references
	log.Printf("📊 Phase 1: Collecting UTXO references from %d blocks", len(blocks))
	for _, block := range blocks {
		for _, tx := range block.Transactions {
			for _, vin := range tx.Vin {
				if vin.Txid != "" && vin.Vout >= 0 {
					key := fmt.Sprintf("%s:%d", vin.Txid, vin.Vout)
					if _, exists := utxoRefMap[key]; !exists {
						utxoRefMap[key] = len(allUTXORefs)
						allUTXORefs = append(allUTXORefs, database.UTXORef{
							Txid: vin.Txid,
							Vout: vin.Vout,
						})
					}
				}
			}
		}
	}

	log.Printf("📊 Collected %d unique UTXO references for bulk lookup", len(allUTXORefs))

	// Bulk fetch all needed UTXOs in ONE query
	utxoFetchStart := time.Now()
	existingUTXOs, err := i.db.BatchGetUTXOs(allUTXORefs)
	if err != nil {
		return fmt.Errorf("failed to bulk get UTXOs: %w", err)
	}
	utxoFetchDuration := time.Since(utxoFetchStart)
	log.Printf("📊 Bulk UTXO fetch completed in %v - found %d existing UTXOs", utxoFetchDuration, len(existingUTXOs))

	// Second pass: process all blocks and collect all data
	log.Printf("📊 Phase 2: Processing blocks and collecting transaction data")
	totalTxs := 0
	totalInputs := 0
	totalOutputs := 0

	for blockIdx, block := range blocks {
		blockHash := blockHashes[blockIdx]
		height := blockHeightMap[blockHash]

		// Process transactions for this block
		txPointers := make([]*bitcoin.Tx, len(block.Transactions))
		for idx, tx := range block.Transactions {
			txPointers[idx] = &tx
		}

		// Prepare transaction data
		for _, tx := range block.Transactions {
			transaction := &models.Transaction{
				Txid:        tx.Txid,
				BlockHeight: &block.Height,
				BlockHash:   &block.Hash,
				BlockTime:   i.convertBlockTime(&block.Time),
				Size:        &tx.Size,
				Weight:      &tx.Weight,
				FeeSats:     i.calculateFee(&tx),
				IsCoinbase:  i.isCoinbase(&tx),
				CreatedAt:   time.Now(),
			}
			allTransactions = append(allTransactions, transaction)

			// Prepare inputs
			for i, vin := range tx.Vin {
				input := &models.TransactionInput{
					Txid:      tx.Txid,
					Vout:      i,
					PrevTxid:  vin.Txid,
					PrevVout:  vin.Vout,
					ScriptSig: &vin.ScriptSig.Hex,
					Sequence:  &vin.Sequence,
					Witness:   vin.Txinwitness,
				}
				allInputs = append(allInputs, input)
			}

			// Prepare outputs
			for _, vout := range tx.Vout {
				address := i.extractAddressFromScriptPubKey(vout.ScriptPubKey)
				scriptType, _ := i.getScriptType(vout.ScriptPubKey.Hex)

				output := &models.TransactionOutput{
					Txid:       tx.Txid,
					Vout:       vout.N,
					Address:    address,
					ValueSats:  int64(vout.Value * 100000000),
					ScriptType: &scriptType,
					ScriptHex:  &vout.ScriptPubKey.Hex,
					ScriptAsm:  &vout.ScriptPubKey.Asm,
				}
				allOutputs = append(allOutputs, output)
			}

			totalTxs++
			totalInputs += len(tx.Vin)
			totalOutputs += len(tx.Vout)
		}

		// Parse UTXOs and transaction references for this block
		newUTXOs, spentUTXOs, txReferences, err := i.parseBlockUTXOs(block, existingUTXOs, watchedScriptMap)
		if err != nil {
			return fmt.Errorf("failed to parse UTXOs for block %d: %w", height, err)
		}

		allNewUTXOs = append(allNewUTXOs, newUTXOs...)
		allSpentUTXOs = append(allSpentUTXOs, spentUTXOs...)
		allTxReferences = append(allTxReferences, txReferences...)
	}

	// Log comprehensive statistics
	log.Printf("📊 Bulk processing statistics:")
	log.Printf("   📦 Blocks: %d", len(blocks))
	log.Printf("   💳 Transactions: %d", totalTxs)
	log.Printf("   📥 Inputs: %d", totalInputs)
	log.Printf("   📤 Outputs: %d", totalOutputs)
	log.Printf("   🆕 New UTXOs: %d", len(allNewUTXOs))
	log.Printf("   💸 Spent UTXOs: %d", len(allSpentUTXOs))
	log.Printf("   🔗 Transaction References: %d", len(allTxReferences))
	log.Printf("   👀 Watched Scripts: %d", len(watchedScriptMap))

	// Execute all bulk operations in a single transaction
	dbStart := time.Now()
	err = i.executeBulkOperations(allTransactions, allInputs, allOutputs, allNewUTXOs, allSpentUTXOs, allTxReferences, blockHashes, blockHeightMap)
	dbDuration := time.Since(dbStart)
	totalDuration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("bulk operations failed after %v: %w", totalDuration, err)
	}

	// Log performance metrics
	log.Printf("✅ Bulk processing completed successfully!")
	log.Printf("   ⏱️  Total time: %v", totalDuration)
	log.Printf("   🗄️  Database time: %v (%.1f%% of total)", dbDuration, float64(dbDuration.Nanoseconds())/float64(totalDuration.Nanoseconds())*100)
	log.Printf("   📊 Throughput: %.2f blocks/sec, %.2f txs/sec",
		float64(len(blocks))/totalDuration.Seconds(),
		float64(totalTxs)/totalDuration.Seconds())

	return nil
}

// executeBulkOperations executes all database operations in a single transaction for maximum performance
func (i *Indexer) executeBulkOperations(transactions []*models.Transaction, inputs []*models.TransactionInput, outputs []*models.TransactionOutput, newUTXOs []*models.UTXO, spentUTXOs []*models.UTXO, txReferences []*models.TransactionReference, blockHashes []string, blockHeightMap map[string]int) error {
	return i.db.ExecuteBulkOperations(transactions, inputs, outputs, newUTXOs, spentUTXOs, txReferences, blockHashes, blockHeightMap)
}

// processBlockWithCache processes a single block using cached watched scripts with panic recovery
func (i *Indexer) processBlockWithCache(height int, watchedScriptMap map[string]*models.WatchedScript) (err error) {
	startTime := time.Now()

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

	// Get block hash using RPC pool
	blockHash, err := i.rpcPool.GetBlockHash(height)
	if err != nil {
		return fmt.Errorf("failed to get block hash for height %d: %w", height, err)
	}

	// Check for reorg
	if err := i.checkReorg(height, blockHash); err != nil {
		return fmt.Errorf("failed to check reorg: %w", err)
	}

	// Get block data using RPC pool
	block, err := i.rpcPool.GetBlock(blockHash)
	if err != nil {
		return fmt.Errorf("failed to get block data: %w", err)
	}

	// Count transactions and I/O for logging
	totalInputs := 0
	totalOutputs := 0
	for _, tx := range block.Transactions {
		totalInputs += len(tx.Vin)
		totalOutputs += len(tx.Vout)
	}

	log.Printf("📦 Processing block %d: %d txs, %d inputs, %d outputs",
		height, len(block.Transactions), totalInputs, totalOutputs)

	// OPTIMIZATION: Process everything in a single database transaction
	// This reduces database overhead from 3+ transactions to 1 transaction per block
	return i.processBlockInSingleTransaction(height, blockHash, block, watchedScriptMap, startTime)
}

// processBlockInSingleTransaction processes a block in a single database transaction for better performance
func (i *Indexer) processBlockInSingleTransaction(height int, blockHash string, block *bitcoin.Block, watchedScriptMap map[string]*models.WatchedScript, startTime time.Time) error {
	// Convert []bitcoin.Tx to []*bitcoin.Tx
	txPointers := make([]*bitcoin.Tx, len(block.Transactions))
	for idx, tx := range block.Transactions {
		txPointers[idx] = &tx
	}

	// Parse block transactions for UTXOs and transaction references (for watched addresses)
	parseStart := time.Now()
	newUTXOs, spentUTXOs, txReferences, err := i.parser.ParseBlockWithCache(block, watchedScriptMap)
	if err != nil {
		return fmt.Errorf("failed to parse block: %w", err)
	}
	parseDuration := time.Since(parseStart)

	// OPTIMIZATION: Process everything in a single database transaction
	// This includes: transactions, inputs, outputs, UTXOs, and progress update
	utxoStart := time.Now()
	if err := i.processBlockDataInSingleTransaction(txPointers, &block.Height, &block.Hash, &block.Time, newUTXOs, spentUTXOs, txReferences, height, blockHash); err != nil {
		return fmt.Errorf("failed to process block data in single transaction: %w", err)
	}
	utxoDuration := time.Since(utxoStart)

	totalDuration := time.Since(startTime)

	// Log block processing details
	log.Printf("✅ Block %d completed: %d txs, %d new UTXOs, %d spent UTXOs, %d tx refs in %v",
		height, len(block.Transactions), len(newUTXOs), len(spentUTXOs), len(txReferences), totalDuration)
	log.Printf("   ⏱️  Times: Parse=%v, DB=%v",
		parseDuration, utxoDuration)

	return nil
}

// processBlockDataInSingleTransaction processes all block data in a single database transaction
func (i *Indexer) processBlockDataInSingleTransaction(transactions []*bitcoin.Tx, blockHeight *int, blockHash *string, blockTime *int64, newUTXOs, spentUTXOs []*models.UTXO, txReferences []*models.TransactionReference, height int, blockHashStr string) error {
	// Start a single database transaction for all operations
	tx, err := i.db.Conn().Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Set transaction isolation level for better performance
	_, err = tx.Exec("SET TRANSACTION ISOLATION LEVEL READ COMMITTED")
	if err != nil {
		return fmt.Errorf("failed to set transaction isolation level: %w", err)
	}

	// 1. Process transactions, inputs, and outputs in batch
	if len(transactions) > 0 {
		if err := i.parser.ProcessTransactionsBatchInTx(tx, transactions, blockHeight, blockHash, blockTime); err != nil {
			return fmt.Errorf("failed to process transactions batch: %w", err)
		}
	}

	// 2. Process UTXOs in batch
	if len(newUTXOs) > 0 {
		if err := i.db.BatchUpsertUTXOsInTx(tx, newUTXOs); err != nil {
			return fmt.Errorf("failed to batch upsert UTXOs: %w", err)
		}
	}

	if len(spentUTXOs) > 0 {
		if err := i.db.BatchMarkUTXOsSpentInTx(tx, spentUTXOs); err != nil {
			return fmt.Errorf("failed to batch mark UTXOs as spent: %w", err)
		}
	}

	// 3. Process transaction references in batch
	if len(txReferences) > 0 {
		if err := i.db.BatchUpsertTransactionReferencesInTx(tx, txReferences); err != nil {
			return fmt.Errorf("failed to batch upsert transaction references: %w", err)
		}
	}

	// 4. Update index progress
	if err := i.db.UpdateIndexProgressInTx(tx, height, blockHashStr); err != nil {
		return fmt.Errorf("failed to update index progress: %w", err)
	}

	// Commit the single transaction
	return tx.Commit()
}

// processBlocksParallel processes blocks in parallel one at a time
func (i *Indexer) processBlocksParallel(startHeight, endHeight int) error {
	startTime := time.Now()
	totalBlocks := endHeight - startHeight + 1

	log.Printf("🚀 Starting parallel single-block processing:")
	log.Printf("   📦 Total blocks: %d (heights %d-%d)", totalBlocks, startHeight, endHeight)
	log.Printf("   👥 Workers: %d", i.config.BackfillWorkers)

	// Create channels for work distribution
	blockChan := make(chan int, i.config.BackfillWorkers*2)
	errorChan := make(chan error, i.config.BackfillWorkers)

	// Track progress
	var processed int32
	var mu sync.Mutex
	errors := make([]error, 0)

	// Start workers - they process individual blocks
	var wg sync.WaitGroup
	for w := 0; w < i.config.BackfillWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			blocksProcessed := 0

			for height := range blockChan {
				blockStart := time.Now()
				if err := i.processBlockWithRPCPool(height); err != nil {
					// Log the failed block to file
					i.logFailedBlock(height, err, false)
					errorChan <- fmt.Errorf("worker %d failed block %d: %w", workerID, height, err)
					continue
				}

				blocksProcessed++
				blockDuration := time.Since(blockStart)

				// Update progress
				current := atomic.AddInt32(&processed, 1)
				if current%100 == 0 || current == int32(totalBlocks) {
					remaining := totalBlocks - int(current)
					elapsed := time.Since(startTime)
					rate := float64(current) / elapsed.Seconds()
					eta := time.Duration(float64(remaining)/rate) * time.Second

					log.Printf("📊 Progress: %d/%d blocks (%.1f%%) - %.2f blocks/sec - ETA: %v",
						current, totalBlocks, float64(current)/float64(totalBlocks)*100, rate, eta)
				}

				// Log block completion for workers
				if blocksProcessed%50 == 0 {
					log.Printf("👷 Worker %d: completed %d blocks, last block (%d) in %v",
						workerID, blocksProcessed, height, blockDuration)
				}
			}

			log.Printf("👷 Worker %d completed: processed %d blocks", workerID, blocksProcessed)
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

	// Send individual blocks to workers
	go func() {
		defer close(blockChan)
		for height := startHeight; height <= endHeight; height++ {
			select {
			case blockChan <- height:
			case <-i.stopChan:
				log.Printf("🛑 Stopping block dispatch due to stop signal")
				return
			}
		}
		log.Printf("📤 All blocks dispatched to workers")
	}()

	// Wait for all workers to complete
	wg.Wait()
	close(errorChan)

	totalDuration := time.Since(startTime)

	// Check for errors
	mu.Lock()
	defer mu.Unlock()
	if len(errors) > 0 {
		log.Printf("❌ Parallel processing completed with %d errors out of %d blocks", len(errors), totalBlocks)
		log.Printf("   ⏱️  Total time: %v", totalDuration)
		return fmt.Errorf("parallel processing had %d errors (first: %v)", len(errors), errors[0])
	}

	// Log final statistics
	successfulBlocks := totalBlocks - len(errors)
	rate := float64(successfulBlocks) / totalDuration.Seconds()

	log.Printf("✅ Parallel single-block processing completed successfully!")
	log.Printf("   📦 Blocks processed: %d", successfulBlocks)
	log.Printf("   ⏱️  Total time: %v", totalDuration)
	log.Printf("   📊 Average rate: %.2f blocks/sec", rate)
	log.Printf("   👥 Workers used: %d", i.config.BackfillWorkers)

	return nil
}

// processBlockChunkBulk processes a chunk of blocks using true bulk operations
func (i *Indexer) processBlockChunkBulk(heights []int) error {
	if len(heights) == 0 {
		return nil
	}

	// Get watched scripts cache once for the entire chunk
	watchedScriptMap, err := i.getWatchedScriptsMap()
	if err != nil {
		return fmt.Errorf("failed to get watched scripts: %w", err)
	}

	// Use bulk processing for the chunk
	return i.processBlocksBulk(heights[0], heights[len(heights)-1], watchedScriptMap)
}

// processBlockIndependent processes a single block completely independently with panic recovery
func (i *Indexer) processBlockIndependent(height int) (err error) {
	// Performance timing for metrics collection
	start := time.Now()

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

	// Get block hash using RPC pool
	hashStart := time.Now()
	blockHash, err := i.rpcPool.GetBlockHash(height)
	if err != nil {
		return fmt.Errorf("failed to get block hash for height %d: %w", height, err)
	}
	hashDuration := time.Since(hashStart)

	// Get block data using RPC pool
	blockStart := time.Now()
	block, err := i.rpcPool.GetBlock(blockHash)
	if err != nil {
		return fmt.Errorf("failed to get block data: %w", err)
	}
	blockDuration := time.Since(blockStart)

	// RPC performance tracked for metrics (no console logging)

	// Store comprehensive transaction data for ALL transactions
	txStart := time.Now()
	txPointers := make([]*bitcoin.Tx, len(block.Transactions))
	for idx, tx := range block.Transactions {
		txPointers[idx] = &tx
	}

	if err := i.parser.ProcessTransactionsBatch(txPointers, &block.Height, &block.Hash, &block.Time); err != nil {
		return fmt.Errorf("failed to process transactions batch: %w", err)
	}
	txDuration := time.Since(txStart)

	// Parse block transactions for UTXOs and transaction references
	parseStart := time.Now()
	watchedScriptMap, err := i.getWatchedScriptsMap()
	if err != nil {
		return fmt.Errorf("failed to get watched scripts: %w", err)
	}

	newUTXOs, spentUTXOs, txReferences, err := i.parser.ParseBlockWithCache(block, watchedScriptMap)
	if err != nil {
		return fmt.Errorf("failed to parse block: %w", err)
	}
	parseDuration := time.Since(parseStart)

	// Process UTXOs and transaction references
	utxoStart := time.Now()
	if err := i.parser.ProcessUTXOs(newUTXOs, spentUTXOs, txReferences); err != nil {
		return fmt.Errorf("failed to process UTXOs: %w", err)
	}
	utxoDuration := time.Since(utxoStart)

	// Update index progress (this can happen in any order for historical data)
	progressStart := time.Now()
	if err := i.db.UpdateIndexProgress(height, blockHash); err != nil {
		return fmt.Errorf("failed to update index progress: %w", err)
	}
	progressDuration := time.Since(progressStart)

	// Collect metrics for JSON file (always enabled for testing)
	if i.metricsCollector != nil {
		metrics := &BlockMetrics{
			Height:           height,
			Timestamp:        start,
			BlockHash:        blockHash,
			RPCHashTime:      hashDuration,
			RPCBlockTime:     blockDuration,
			StoreTXTime:      txDuration,
			ParseTime:        parseDuration,
			ProcessUTXOTime:  utxoDuration,
			ProgressTime:     progressDuration,
			TotalTime:        time.Since(start),
			TransactionCount: len(block.Transactions),
			InputCount:       countInputs(block.Transactions),
			OutputCount:      countOutputs(block.Transactions),
		}
		i.metricsCollector.AddBlockMetrics(*metrics)
	}

	return nil
}

// countInputs counts total inputs across all transactions in a block
func countInputs(transactions []bitcoin.Tx) int {
	total := 0
	for _, tx := range transactions {
		total += len(tx.Vin)
	}
	return total
}

// countOutputs counts total outputs across all transactions in a block
func countOutputs(transactions []bitcoin.Tx) int {
	total := 0
	for _, tx := range transactions {
		total += len(tx.Vout)
	}
	return total
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

	// Block processed (no logging for sequential processing)

	return nil
}

// ProcessBlocksParallel processes multiple blocks in parallel one at a time
// ProcessBlocksWithWorkerPool processes blocks using the optimized worker pool architecture
func (i *Indexer) ProcessBlocksWithWorkerPool(startHeight, endHeight int) error {
	if i.workerPool == nil {
		log.Printf("⚠️  Worker pool not available, falling back to parallel processing")
		// Create heights array for fallback
		heights := make([]int, endHeight-startHeight+1)
		for i := 0; i < len(heights); i++ {
			heights[i] = startHeight + i
		}
		return i.ProcessBlocksParallel(heights, 20) // Use 20 workers as fallback
	}

	log.Printf("🏭 Starting worker pool processing for blocks %d-%d", startHeight, endHeight)

	// Start the worker pool
	if err := i.workerPool.Start(); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}
	defer i.workerPool.Stop()

	// Process blocks using worker pool
	return i.workerPool.ProcessBlocks(startHeight, endHeight)
}

func (i *Indexer) ProcessBlocksParallel(heights []int, maxWorkers int) error {
	if len(heights) == 0 {
		return nil
	}

	if maxWorkers <= 0 {
		maxWorkers = 10 // Default to 10 parallel workers
	}

	startTime := time.Now()
	totalBlocks := len(heights)

	log.Printf("🚀 Starting ProcessBlocksParallel with single-block processing:")
	log.Printf("   📦 Total blocks: %d", totalBlocks)
	log.Printf("   👥 Workers: %d", maxWorkers)
	log.Printf("   📍 Block range: %d to %d", heights[0], heights[len(heights)-1])

	// Set backfill mode to disable reorg detection
	i.mu.Lock()
	i.inBackfillMode = true
	i.mu.Unlock()
	defer func() {
		i.mu.Lock()
		i.inBackfillMode = false
		i.mu.Unlock()
	}()

	// Create channels for work distribution
	blockChan := make(chan int, maxWorkers*2)
	errorChan := make(chan error, maxWorkers)

	// Track progress
	var processed int32
	var mu sync.Mutex
	errors := make([]error, 0)

	// Start workers - they process individual blocks
	var wg sync.WaitGroup
	for w := 0; w < maxWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			blocksProcessed := 0

			for height := range blockChan {
				blockStart := time.Now()
				if err := i.processBlockWithRPCPool(height); err != nil {
					// Log the failed block to file
					i.logFailedBlock(height, err, false)
					errorChan <- fmt.Errorf("worker %d failed block %d: %w", workerID, height, err)
					continue
				}

				blocksProcessed++
				blockDuration := time.Since(blockStart)

				// Update progress
				current := atomic.AddInt32(&processed, 1)
				if current%100 == 0 || current == int32(totalBlocks) {
					remaining := totalBlocks - int(current)
					elapsed := time.Since(startTime)
					rate := float64(current) / elapsed.Seconds()
					eta := time.Duration(float64(remaining)/rate) * time.Second

					log.Printf("📊 Progress: %d/%d blocks (%.1f%%) - %.2f blocks/sec - ETA: %v",
						current, totalBlocks, float64(current)/float64(totalBlocks)*100, rate, eta)
				}

				// Log block completion for workers
				if blocksProcessed%50 == 0 {
					log.Printf("👷 Worker %d: completed %d blocks, last block (%d) in %v",
						workerID, blocksProcessed, height, blockDuration)
				}
			}

			log.Printf("👷 Worker %d completed: processed %d blocks", workerID, blocksProcessed)
		}(w)
	}

	// Send individual blocks to workers
	go func() {
		defer close(blockChan)
		for _, height := range heights {
			select {
			case blockChan <- height:
			case <-i.stopChan:
				log.Printf("🛑 Stopping block dispatch due to stop signal")
				return
			}
		}
		log.Printf("📤 All blocks dispatched to workers")
	}()

	// Collect errors
	go func() {
		defer close(errorChan)
		for err := range errorChan {
			mu.Lock()
			errors = append(errors, err)
			mu.Unlock()
		}
	}()

	// Wait for all workers to complete
	wg.Wait()

	totalDuration := time.Since(startTime)

	// Log summary
	if len(errors) > 0 {
		log.Printf("❌ ProcessBlocksParallel completed with %d failed blocks out of %d total blocks", len(errors), totalBlocks)
		for _, err := range errors {
			log.Printf("   ❌ %v", err)
		}
	}

	successfulBlocks := totalBlocks - len(errors)
	rate := float64(successfulBlocks) / totalDuration.Seconds()

	if successfulBlocks >= 10 {
		log.Printf("✅ ProcessBlocksParallel completed successfully!")
		log.Printf("   📦 Blocks processed: %d", successfulBlocks)
		log.Printf("   ⏱️  Total time: %v", totalDuration)
		log.Printf("   📊 Average rate: %.2f blocks/sec", rate)
		log.Printf("   👥 Workers used: %d", maxWorkers)
	}

	if len(errors) > 0 {
		return fmt.Errorf("%d blocks failed to process", len(errors))
	}

	return nil
}

// processBlockWithRPCPool processes a single block using the RPC pool
func (i *Indexer) processBlockWithRPCPool(height int) error {
	// Initialize metrics collection only for backfill operations
	start := time.Now()
	var metrics *BlockMetrics

	// Only collect metrics during backfill operations
	i.mu.RLock()
	inBackfill := i.inBackfillMode
	i.mu.RUnlock()

	if inBackfill {
		metrics = &BlockMetrics{
			Height:    height,
			Timestamp: start,
		}
	}

	// Get block hash using RPC pool
	hashStart := time.Now()
	blockHash, err := i.rpcPool.GetBlockHash(height)
	if err != nil {
		return fmt.Errorf("failed to get block hash for height %d: %w", height, err)
	}
	if metrics != nil {
		metrics.RPCHashTime = time.Since(hashStart)
		metrics.BlockHash = blockHash
	}

	// Skip reorg detection during parallel processing (backfill mode)
	// Reorg detection is only needed for sequential real-time processing

	// Get block data using RPC pool
	blockStart := time.Now()
	block, err := i.rpcPool.GetBlock(blockHash)
	if err != nil {
		return fmt.Errorf("failed to get block data: %w", err)
	}
	if metrics != nil {
		metrics.RPCBlockTime = time.Since(blockStart)

		// Count transactions and I/O
		metrics.TransactionCount = len(block.Transactions)
		metrics.InputCount = 0
		metrics.OutputCount = 0
		for _, tx := range block.Transactions {
			metrics.InputCount += len(tx.Vin)
			metrics.OutputCount += len(tx.Vout)
		}
	}

	// Store comprehensive transaction data for ALL transactions
	txStart := time.Now()
	txPointers := make([]*bitcoin.Tx, len(block.Transactions))
	for idx, tx := range block.Transactions {
		txPointers[idx] = &tx
	}

	if err := i.parser.ProcessTransactionsBatch(txPointers, &block.Height, &block.Hash, &block.Time); err != nil {
		return fmt.Errorf("failed to process transactions batch: %w", err)
	}
	if metrics != nil {
		metrics.StoreTXTime = time.Since(txStart)
	}

	// Parse block transactions for UTXOs and transaction references
	parseStart := time.Now()
	watchedScriptMap, err := i.getWatchedScriptsMap()
	if err != nil {
		return fmt.Errorf("failed to get watched scripts: %w", err)
	}

	newUTXOs, spentUTXOs, txReferences, err := i.parser.ParseBlockWithCache(block, watchedScriptMap)
	if err != nil {
		return fmt.Errorf("failed to parse block: %w", err)
	}
	if metrics != nil {
		metrics.ParseTime = time.Since(parseStart)
		metrics.NewUTXOCount = len(newUTXOs)
		metrics.SpentUTXOCount = len(spentUTXOs)
	}

	// Process UTXOs and transaction references
	utxoStart := time.Now()
	if err := i.parser.ProcessUTXOs(newUTXOs, spentUTXOs, txReferences); err != nil {
		return fmt.Errorf("failed to process UTXOs: %w", err)
	}
	if metrics != nil {
		metrics.ProcessUTXOTime = time.Since(utxoStart)
	}

	// Update index progress
	progressStart := time.Now()
	if err := i.db.UpdateIndexProgress(height, blockHash); err != nil {
		return fmt.Errorf("failed to update index progress: %w", err)
	}
	if metrics != nil {
		metrics.ProgressTime = time.Since(progressStart)
		// Calculate total time
		metrics.TotalTime = time.Since(start)
	}

	// Add metrics to collector (only during backfill)
	if i.metricsCollector != nil && metrics != nil {
		i.metricsCollector.AddBlockMetrics(*metrics)
	}

	// Log individual block completion for parallel processing
	duration := time.Since(start)
	log.Printf("🔄 Block %d processed: %d txs, %d new UTXOs, %d spent UTXOs in %v",
		height, len(block.Transactions), len(newUTXOs), len(spentUTXOs), duration)

	return nil
}

// MetricsCollector returns the metrics collector for external access
func (i *Indexer) MetricsCollector() *MetricsCollector {
	return i.metricsCollector
}

// SaveMetricsNow manually saves current metrics to file
func (i *Indexer) SaveMetricsNow() error {
	if i.metricsCollector != nil {
		return i.metricsCollector.SaveMetrics()
	}
	return fmt.Errorf("metrics collector not initialized")
}

// BlockProcessResult represents the result of processing a block
type BlockProcessResult struct {
	Height   int
	Duration time.Duration
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

// checkReorg checks for chain reorganizations (only for real-time processing)
func (i *Indexer) checkReorg(height int, blockHash string) error {
	// Skip reorg detection during backfill operations
	i.mu.RLock()
	inBackfill := i.inBackfillMode
	i.mu.RUnlock()

	if inBackfill {
		// Backfill mode - skip reorg detection entirely
		return nil
	}

	progress, err := i.db.GetIndexProgress()
	if err != nil {
		return fmt.Errorf("failed to get index progress: %w", err)
	}

	// Skip reorg detection for historical blocks
	if height <= progress.LastHeight {
		// This is a backfill operation - skip reorg detection
		return nil
	}

	// If this is the first block or height is 0, no reorg check needed
	if progress.LastHeight == 0 || height == 0 {
		return nil
	}

	// Only check for reorgs when processing the next sequential block
	if height == progress.LastHeight+1 {
		// Verify the block hash matches what we expect
		if blockHash != progress.LastBlockHash {
			log.Printf("Reorg detected at height %d. Expected: %s, Got: %s",
				height, progress.LastBlockHash, blockHash)

			if err := i.handleReorg(height); err != nil {
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

// getIntEnv gets an integer environment variable with a default value
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// Helper methods for bulk processing

// convertBlockTime converts block timestamp to time.Time
func (i *Indexer) convertBlockTime(blockTime *int64) *time.Time {
	if blockTime == nil {
		return nil
	}
	t := time.Unix(*blockTime, 0)
	return &t
}

// calculateFee calculates transaction fee
func (i *Indexer) calculateFee(tx *bitcoin.Tx) *int64 {
	// Simple fee calculation: sum of inputs - sum of outputs
	// This is a simplified approach; in practice, you'd need to fetch input values
	// For now, return nil to indicate fee calculation is not implemented
	return nil
}

// isCoinbase checks if transaction is coinbase
func (i *Indexer) isCoinbase(tx *bitcoin.Tx) bool {
	return len(tx.Vin) == 1 && tx.Vin[0].Txid == "" && tx.Vin[0].Vout == 0
}

// extractAddressFromScriptPubKey extracts address from script pub key
func (i *Indexer) extractAddressFromScriptPubKey(scriptPubKey bitcoin.ScriptPubKey) string {
	// Extract address with priority: address field > addresses[0] > derived identifier
	if scriptPubKey.Address != "" {
		return scriptPubKey.Address
	}
	if len(scriptPubKey.Addresses) > 0 {
		return scriptPubKey.Addresses[0]
	}
	return ""
}

// getScriptType determines the script type from script hex
func (i *Indexer) getScriptType(scriptHex string) (string, error) {
	// Import the parser package function
	return parser.GetScriptType(scriptHex)
}

// parseBlockUTXOs parses UTXOs for a single block using pre-fetched data
func (i *Indexer) parseBlockUTXOs(block *bitcoin.Block, existingUTXOs map[string]*models.UTXO, watchedScriptMap map[string]*models.WatchedScript) ([]*models.UTXO, []*models.UTXO, []*models.TransactionReference, error) {
	var newUTXOs []*models.UTXO
	var spentUTXOs []*models.UTXO
	var txReferences []*models.TransactionReference

	// Process transactions with pre-fetched UTXOs
	for _, tx := range block.Transactions {
		// Parse outputs (ALL new UTXOs, not just watched ones)
		for _, vout := range tx.Vout {
			// Extract address with priority: address field > addresses[0] > derived identifier
			address := i.extractAddressFromScriptPubKey(vout.ScriptPubKey)

			// Store ALL UTXOs
			utxo := &models.UTXO{
				Txid:        tx.Txid,
				Vout:        vout.N,
				Address:     address,
				ScriptHex:   vout.ScriptPubKey.Hex,
				ValueSats:   int64(vout.Value * 100000000), // Convert BTC to satoshis
				Status:      "confirmed",
				BlockHeight: &block.Height,
				BlockHash:   &block.Hash,
				FirstSeenAt: time.Unix(block.Time, 0),
			}
			newUTXOs = append(newUTXOs, utxo)

			// Check if this output is for a watched address using cache
			if _, isWatched := watchedScriptMap[vout.ScriptPubKey.Hex]; isWatched {
				// Compute sender address for incoming transaction
				senderAddress := i.computeSenderAddressFromUTXOs(&tx, existingUTXOs)

				blockTimestamp := int(block.Time)
				txRef := &models.TransactionReference{
					Txid:           tx.Txid,
					Address:        address,
					Direction:      "in",
					ValueSats:      int64(vout.Value * 100000000),
					SenderAddress:  senderAddress,
					BlockHeight:    &block.Height,
					BlockTimestamp: &blockTimestamp,
					CreatedAt:      time.Now(),
				}
				txReferences = append(txReferences, txRef)
			}
		}

		// Parse inputs (ALL spent UTXOs) - use pre-fetched data
		for _, vin := range tx.Vin {
			if vin.Txid != "" && vin.Vout >= 0 { // Skip coinbase inputs
				key := fmt.Sprintf("%s:%d", vin.Txid, vin.Vout)
				existingUTXO, found := existingUTXOs[key]

				if found && existingUTXO != nil {
					// Mark as spent
					spentUTXO := *existingUTXO
					spentUTXO.Status = "spent"
					spentAt := time.Unix(block.Time, 0)
					spentUTXO.SpentAt = &spentAt
					spentUTXO.SpentByTxid = &tx.Txid
					spentUTXOs = append(spentUTXOs, &spentUTXO)

					// Check if the spent UTXO was for a watched address using cache
					if _, isWatched := watchedScriptMap[existingUTXO.ScriptHex]; isWatched {
						// Compute receiver address for outgoing transaction
						receiverAddress := i.computeReceiverAddress(&tx, block.Height)

						blockTimestamp := int(block.Time)
						txRef := &models.TransactionReference{
							Txid:            tx.Txid,
							Address:         existingUTXO.Address,
							Direction:       "out",
							ValueSats:       existingUTXO.ValueSats,
							ReceiverAddress: receiverAddress,
							BlockHeight:     &block.Height,
							BlockTimestamp:  &blockTimestamp,
							CreatedAt:       time.Now(),
						}
						txReferences = append(txReferences, txRef)
					}
				}
			}
		}
	}

	return newUTXOs, spentUTXOs, txReferences, nil
}

// computeSenderAddressFromUTXOs computes sender address from pre-fetched UTXOs map
func (i *Indexer) computeSenderAddressFromUTXOs(tx *bitcoin.Tx, utxoMap map[string]*models.UTXO) *string {
	// For simplicity, we'll use the first input's previous output address as sender
	for _, vin := range tx.Vin {
		if vin.Txid != "" && vin.Vout >= 0 {
			key := fmt.Sprintf("%s:%d", vin.Txid, vin.Vout)
			if prevUTXO, found := utxoMap[key]; found && prevUTXO != nil {
				return &prevUTXO.Address
			}
		}
	}
	return nil
}

// computeReceiverAddress computes the receiver address from transaction outputs
func (i *Indexer) computeReceiverAddress(tx *bitcoin.Tx, blockHeight int) *string {
	// For simplicity, we'll use the first output address as receiver
	// In a more sophisticated implementation, you'd analyze all outputs
	for _, vout := range tx.Vout {
		address := i.extractAddressFromScriptPubKey(vout.ScriptPubKey)
		if address != "" {
			return &address
		}
	}
	return nil
}
