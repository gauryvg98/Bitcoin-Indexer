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

// WorkerPool manages the RPC and DB worker pools
type WorkerPool struct {
	config     WorkerConfig
	rpcWorkers []*RPCWorker
	dbWorkers  []*DBWorker
	workQueue  chan *WorkItem
	resultChan chan *WorkResult
	heightChan chan int
	stopChan   chan struct{}
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	stats      map[int]*WorkStats
	mu         sync.RWMutex
}

// NewWorkerPool creates a new worker pool with the specified configuration
func NewWorkerPool(config WorkerConfig) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		config:     config,
		workQueue:  make(chan *WorkItem, config.QueueSize),
		resultChan: make(chan *WorkResult, config.QueueSize),
		heightChan: make(chan int, config.QueueSize),
		stopChan:   make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
		stats:      make(map[int]*WorkStats),
	}
}

// Initialize sets up the worker pools with the required dependencies
func (wp *WorkerPool) Initialize(rpcPool *bitcoin.RPCPool, db *database.DB, parser *parser.Parser) error {
	// Create RPC workers
	wp.rpcWorkers = make([]*RPCWorker, wp.config.RPCWorkers)
	for i := 0; i < wp.config.RPCWorkers; i++ {
		wp.rpcWorkers[i] = NewRPCWorker(i+1, rpcPool, parser, wp.workQueue, &wp.wg)
	}

	// Create DB workers
	wp.dbWorkers = make([]*DBWorker, wp.config.DBWorkers)
	for i := 0; i < wp.config.DBWorkers; i++ {
		wp.dbWorkers[i] = NewDBWorker(i+1, db, parser, wp.workQueue, wp.resultChan, &wp.wg)
	}

	log.Printf("🏭 Worker Pool initialized: %d RPC workers, %d DB workers, queue size: %d",
		wp.config.RPCWorkers, wp.config.DBWorkers, wp.config.QueueSize)

	return nil
}

// Start begins all workers
func (wp *WorkerPool) Start() error {
	// Start RPC workers
	for _, worker := range wp.rpcWorkers {
		wp.wg.Add(1)
		go worker.Start(wp.ctx, wp.heightChan)
	}

	// Start DB workers
	for _, worker := range wp.dbWorkers {
		wp.wg.Add(1)
		go worker.Start(wp.ctx)
	}

	// Start result processor
	wp.wg.Add(1)
	go wp.processResults()

	log.Printf("🚀 Worker Pool started with %d RPC workers and %d DB workers",
		len(wp.rpcWorkers), len(wp.dbWorkers))

	return nil
}

// Stop gracefully stops all workers
func (wp *WorkerPool) Stop() {
	log.Printf("🛑 Stopping Worker Pool...")

	// Cancel context to stop all workers
	wp.cancel()

	// Close channels
	close(wp.heightChan)
	close(wp.workQueue)
	close(wp.resultChan)

	// Wait for all workers to finish
	wp.wg.Wait()

	log.Printf("✅ Worker Pool stopped")
}

// ProcessBlocks processes a range of blocks using the worker pool
func (wp *WorkerPool) ProcessBlocks(startHeight, endHeight int) error {
	log.Printf("📦 Processing blocks %d-%d using worker pool", startHeight, endHeight)

	startTime := time.Now()
	totalBlocks := endHeight - startHeight + 1
	processedBlocks := 0
	failedBlocks := 0

	// Send all block heights to RPC workers
	go func() {
		defer close(wp.heightChan)
		for height := startHeight; height <= endHeight; height++ {
			select {
			case wp.heightChan <- height:
				// Successfully queued
			case <-wp.ctx.Done():
				log.Printf("⚠️  Context cancelled while queuing blocks")
				return
			}
		}
	}()

	// Process results
	for result := range wp.resultChan {
		processedBlocks++

		if result.Success {
			// Update stats
			wp.mu.Lock()
			wp.stats[result.DBWorkerID] = &WorkStats{
				DBWorkerID:      result.DBWorkerID,
				BlocksProcessed: 1,
				TotalTime:       result.ProcessTime,
				AverageTime:     result.ProcessTime,
				LastProcessed:   result.CompletedAt,
			}
			wp.mu.Unlock()
		} else {
			failedBlocks++
			log.Printf("❌ Block %d failed: %v", result.Height, result.Error)
		}

		// Log progress every 100 blocks
		if processedBlocks%100 == 0 || processedBlocks == totalBlocks {
			elapsed := time.Since(startTime)
			rate := float64(processedBlocks) / elapsed.Seconds()
			eta := time.Duration(float64(totalBlocks-processedBlocks)/rate) * time.Second

			log.Printf("📊 Progress: %d/%d blocks (%.1f%%) - %.2f blocks/sec - ETA: %v - Failed: %d",
				processedBlocks, totalBlocks, float64(processedBlocks)/float64(totalBlocks)*100, rate, eta, failedBlocks)
		}
	}

	totalTime := time.Since(startTime)
	rate := float64(processedBlocks) / totalTime.Seconds()

	log.Printf("✅ Worker Pool processing completed!")
	log.Printf("   📦 Blocks processed: %d/%d", processedBlocks, totalBlocks)
	log.Printf("   ⏱️  Total time: %v", totalTime)
	log.Printf("   📊 Average rate: %.2f blocks/sec", rate)
	log.Printf("   ❌ Failed blocks: %d", failedBlocks)

	if failedBlocks > 0 {
		return fmt.Errorf("failed to process %d blocks", failedBlocks)
	}

	return nil
}

// processResults processes results from DB workers
func (wp *WorkerPool) processResults() {
	defer wp.wg.Done()

	for {
		select {
		case result := <-wp.resultChan:
			if result == nil {
				// Channel closed
				return
			}

			// Update statistics
			wp.mu.Lock()
			if stats, exists := wp.stats[result.DBWorkerID]; exists {
				stats.BlocksProcessed++
				stats.TotalTime += result.ProcessTime
				stats.AverageTime = stats.TotalTime / time.Duration(stats.BlocksProcessed)
				stats.LastProcessed = result.CompletedAt
			} else {
				wp.stats[result.DBWorkerID] = &WorkStats{
					DBWorkerID:      result.DBWorkerID,
					BlocksProcessed: 1,
					TotalTime:       result.ProcessTime,
					AverageTime:     result.ProcessTime,
					LastProcessed:   result.CompletedAt,
				}
			}
			wp.mu.Unlock()

		case <-wp.ctx.Done():
			return
		}
	}
}

// GetStats returns statistics for all workers
func (wp *WorkerPool) GetStats() map[int]*WorkStats {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	// Create a copy of stats
	statsCopy := make(map[int]*WorkStats)
	for id, stats := range wp.stats {
		statsCopy[id] = &WorkStats{
			DBWorkerID:      stats.DBWorkerID,
			BlocksProcessed: stats.BlocksProcessed,
			TotalTime:       stats.TotalTime,
			AverageTime:     stats.AverageTime,
			LastProcessed:   stats.LastProcessed,
		}
	}

	return statsCopy
}

// GetRPCWorkerStats returns statistics for RPC workers
func (wp *WorkerPool) GetRPCWorkerStats() []WorkStats {
	var stats []WorkStats
	for _, worker := range wp.rpcWorkers {
		stats = append(stats, worker.GetStats())
	}
	return stats
}

// GetDBWorkerStats returns statistics for DB workers
func (wp *WorkerPool) GetDBWorkerStats() []WorkStats {
	var stats []WorkStats
	for _, worker := range wp.dbWorkers {
		stats = append(stats, worker.GetStats())
	}
	return stats
}
