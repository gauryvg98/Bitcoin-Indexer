package bitcoin

import (
	"log"
	"strings"
	"sync"
	"time"
)

// RPCPool manages a pool of RPC clients for concurrent requests
type RPCPool struct {
	clients   []*RPCClient
	available chan *RPCClient
	mu        sync.RWMutex
	baseURL   string
	username  string
	password  string
	poolSize  int
	created   time.Time
	stats     *PoolStats
}

// PoolStats tracks pool performance metrics
type PoolStats struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	AverageWaitTime    time.Duration
	AverageProcessTime time.Duration
	mu                 sync.RWMutex
}

// NewRPCPool creates a new RPC connection pool
func NewRPCPool(baseURL, username, password string, poolSize int) *RPCPool {
	if poolSize <= 0 {
		poolSize = 5 // Default pool size
	}

	pool := &RPCPool{
		clients:   make([]*RPCClient, poolSize),
		available: make(chan *RPCClient, poolSize),
		baseURL:   baseURL,
		username:  username,
		password:  password,
		poolSize:  poolSize,
		created:   time.Now(),
		stats:     &PoolStats{},
	}

	// Initialize all clients
	for i := 0; i < poolSize; i++ {
		client := NewRPCClient(baseURL, username, password)
		pool.clients[i] = client
		pool.available <- client
	}

	// RPC pool initialized with concurrent clients
	return pool
}

// GetClient acquires an RPC client from the pool with timeout
func (p *RPCPool) GetClient() *RPCClient {
	start := time.Now()

	// Add timeout to prevent indefinite blocking
	select {
	case client := <-p.available:
		// Update stats
		p.stats.mu.Lock()
		p.stats.TotalRequests++
		waitTime := time.Since(start)
		p.stats.AverageWaitTime = (p.stats.AverageWaitTime + waitTime) / 2
		p.stats.mu.Unlock()
		return client
	case <-time.After(30 * time.Second):
		// Timeout - create a temporary client
		log.Printf("WARNING: RPC pool timeout, creating temporary client")
		return NewRPCClient(p.baseURL, p.username, p.password)
	}
}

// ReturnClient returns an RPC client to the pool
func (p *RPCPool) ReturnClient(client *RPCClient) {
	select {
	case p.available <- client:
		// Client returned successfully
	default:
		log.Printf("WARNING: RPC pool is full, client not returned")
	}
}

// GetBlockHash gets block hash using pool with retry logic
func (p *RPCPool) GetBlockHash(height int) (string, error) {
	client := p.GetClient()
	defer p.ReturnClient(client)

	start := time.Now()
	hash, err := client.GetBlockHash(height)

	// Retry logic for RPC errors
	if err != nil && isRetryableRPCError(err) {
		// Wait a bit and retry once
		time.Sleep(100 * time.Millisecond)
		hash, err = client.GetBlockHash(height)
	}

	// Update stats
	p.stats.mu.Lock()
	if err != nil {
		p.stats.FailedRequests++
	} else {
		p.stats.SuccessfulRequests++
	}
	processTime := time.Since(start)
	p.stats.AverageProcessTime = (p.stats.AverageProcessTime + processTime) / 2
	p.stats.mu.Unlock()

	return hash, err
}

// GetBlock gets block data using pool with retry logic
func (p *RPCPool) GetBlock(blockHash string) (*Block, error) {
	client := p.GetClient()
	defer p.ReturnClient(client)

	start := time.Now()
	block, err := client.GetBlock(blockHash)

	// Retry logic for RPC errors
	if err != nil && isRetryableRPCError(err) {
		// Wait a bit and retry once
		time.Sleep(100 * time.Millisecond)
		block, err = client.GetBlock(blockHash)
	}

	// Update stats
	p.stats.mu.Lock()
	if err != nil {
		p.stats.FailedRequests++
	} else {
		p.stats.SuccessfulRequests++
	}
	processTime := time.Since(start)
	p.stats.AverageProcessTime = (p.stats.AverageProcessTime + processTime) / 2
	p.stats.mu.Unlock()

	return block, err
}

// GetBlockHashAsync gets block hash asynchronously
func (p *RPCPool) GetBlockHashAsync(height int) <-chan BlockHashResult {
	result := make(chan BlockHashResult, 1)

	go func() {
		hash, err := p.GetBlockHash(height)
		result <- BlockHashResult{Height: height, Hash: hash, Error: err}
		close(result)
	}()

	return result
}

// GetBlockAsync gets block data asynchronously
func (p *RPCPool) GetBlockAsync(blockHash string) <-chan BlockResult {
	result := make(chan BlockResult, 1)

	go func() {
		block, err := p.GetBlock(blockHash)
		result <- BlockResult{Hash: blockHash, Block: block, Error: err}
		close(result)
	}()

	return result
}

// GetBlockHashAndBlockAsync gets both hash and block data asynchronously
func (p *RPCPool) GetBlockHashAndBlockAsync(height int) <-chan BlockHashAndBlockResult {
	result := make(chan BlockHashAndBlockResult, 1)

	go func() {
		// Get hash first
		hash, err := p.GetBlockHash(height)
		if err != nil {
			result <- BlockHashAndBlockResult{Height: height, Error: err}
			close(result)
			return
		}

		// Get block data
		block, err := p.GetBlock(hash)
		result <- BlockHashAndBlockResult{
			Height: height,
			Hash:   hash,
			Block:  block,
			Error:  err,
		}
		close(result)
	}()

	return result
}

// GetStats returns pool statistics
func (p *RPCPool) GetStats() PoolStats {
	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()

	return PoolStats{
		TotalRequests:      p.stats.TotalRequests,
		SuccessfulRequests: p.stats.SuccessfulRequests,
		FailedRequests:     p.stats.FailedRequests,
		AverageWaitTime:    p.stats.AverageWaitTime,
		AverageProcessTime: p.stats.AverageProcessTime,
	}
}

// Close closes the RPC pool
func (p *RPCPool) Close() {
	close(p.available)
	// RPC pool closed
}

// Result types for async operations
type BlockHashResult struct {
	Height int
	Hash   string
	Error  error
}

type BlockResult struct {
	Hash  string
	Block *Block
	Error error
}

type BlockHashAndBlockResult struct {
	Height int
	Hash   string
	Block  *Block
	Error  error
}

// isRetryableRPCError checks if an RPC error is retryable
func isRetryableRPCError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "EOF")
}
