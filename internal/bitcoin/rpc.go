package bitcoin

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"time"
)

// RPCClient handles Bitcoin Core RPC communication
type RPCClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	// Retry configuration
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

// NewRPCClient creates a new Bitcoin RPC client
func NewRPCClient(baseURL, username, password string) *RPCClient {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		// Connection pooling
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		// Enable HTTP/2 for better multiplexing
		ForceAttemptHTTP2: true,
	}

	return &RPCClient{
		baseURL:  baseURL,
		username: username,
		password: password,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   120 * time.Second, // Increased from 30s to 120s for large blocks
		},
		// Default retry configuration
		maxRetries: 5,
		baseDelay:  100 * time.Millisecond,
		maxDelay:   60 * time.Second, // Increased from 30s to 60s
	}
}

// BaseURL returns the base URL of the RPC client
func (c *RPCClient) BaseURL() string {
	return c.baseURL
}

// Username returns the username of the RPC client
func (c *RPCClient) Username() string {
	return c.username
}

// Password returns the password of the RPC client
func (c *RPCClient) Password() string {
	return c.password
}

// RPCRequest represents a JSON-RPC request
type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// RPCResponse represents a JSON-RPC response
type RPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents an RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return e.Message
}

// Block represents a Bitcoin block
type Block struct {
	Hash          string `json:"hash"`
	Height        int    `json:"height"`
	PreviousHash  string `json:"previousblockhash"`
	Time          int64  `json:"time"`
	Transactions  []Tx   `json:"tx"`
	Confirmations int    `json:"confirmations"`
}

// Tx represents a Bitcoin transaction
type Tx struct {
	Txid     string `json:"txid"`
	Hash     string `json:"hash"`
	Version  int    `json:"version"`
	Size     int    `json:"size"`
	Vsize    int    `json:"vsize"`
	Weight   int    `json:"weight"`
	LockTime int    `json:"locktime"`
	Vin      []Vin  `json:"vin"`
	Vout     []Vout `json:"vout"`
	Hex      string `json:"hex,omitempty"`
}

// Vin represents a transaction input
type Vin struct {
	Txid        string    `json:"txid"`
	Vout        int       `json:"vout"`
	ScriptSig   ScriptSig `json:"scriptSig"`
	Sequence    int64     `json:"sequence"`
	Txinwitness []string  `json:"txinwitness,omitempty"`
}

// Vout represents a transaction output
type Vout struct {
	Value        float64      `json:"value"`
	N            int          `json:"n"`
	ScriptPubKey ScriptPubKey `json:"scriptPubKey"`
}

// ScriptSig represents input script signature
type ScriptSig struct {
	Asm string `json:"asm"`
	Hex string `json:"hex"`
}

// ScriptPubKey represents output script
type ScriptPubKey struct {
	Asm       string   `json:"asm"`
	Hex       string   `json:"hex"`
	Type      string   `json:"type"`
	Address   string   `json:"address,omitempty"`   // Singular (newer Bitcoin Core versions)
	Addresses []string `json:"addresses,omitempty"` // Plural (older Bitcoin Core versions)
}

// isRetryableError checks if an error is retryable
func (c *RPCClient) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network errors that are typically retryable
	if netErr, ok := err.(net.Error); ok {
		return netErr.Temporary() || netErr.Timeout()
	}

	// Check for HTTP errors that are retryable
	if httpErr, ok := err.(*HTTPError); ok {
		// Retry on server errors (5xx) and some client errors (429, 502, 503, 504)
		status := httpErr.StatusCode
		return status >= 500 || status == 429 || status == 502 || status == 503 || status == 504
	}

	// Check for timeout errors
	if err.Error() == "context deadline exceeded" ||
		err.Error() == "i/o timeout" ||
		err.Error() == "connection timeout" {
		return true
	}

	return false
}

// calculateBackoffDelay calculates the delay for exponential backoff
func (c *RPCClient) calculateBackoffDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	delay := time.Duration(float64(c.baseDelay) * math.Pow(2, float64(attempt)))

	// Add jitter to prevent thundering herd
	jitter := time.Duration(float64(delay) * 0.1 * (0.5 + math.Sin(float64(time.Now().UnixNano()))))
	delay += jitter

	// Cap at maxDelay
	if delay > c.maxDelay {
		delay = c.maxDelay
	}

	return delay
}

// callWithRetry makes an RPC call with exponential backoff retry logic
//
// Retry Logic:
// - Uses exponential backoff: delay = baseDelay * 2^attempt
// - Adds jitter to prevent thundering herd problems
// - Caps delay at maxDelay to prevent excessive waits
// - Retries on network errors, timeouts, and HTTP 5xx/429/502/503/504 errors
// - Returns immediately on success or non-retryable errors
func (c *RPCClient) callWithRetry(method string, params []interface{}) (*RPCResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.call(method, params)

		// If successful, return immediately
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// If not retryable or this is the last attempt, return the error
		if !c.isRetryableError(err) || attempt == c.maxRetries {
			break
		}

		// Calculate delay for next attempt
		delay := c.calculateBackoffDelay(attempt)

		// Log retry attempt (you might want to use a proper logger in production)
		fmt.Printf("RPC call failed (attempt %d/%d): %v. Retrying in %v...\n",
			attempt+1, c.maxRetries+1, err, delay)

		// Wait before retrying
		time.Sleep(delay)
	}

	return nil, fmt.Errorf("RPC call failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

// call makes an RPC call to Bitcoin Core
func (c *RPCClient) call(method string, params []interface{}) (*RPCResponse, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Create a custom error that includes status code for retry logic
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("RPC call failed with status %d: %s", resp.StatusCode, string(body)),
		}
	}

	var rpcResp RPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s", rpcResp.Error.Message)
	}

	return &rpcResp, nil
}

// GetBlockCount returns the current block height
func (c *RPCClient) GetBlockCount() (int, error) {
	resp, err := c.callWithRetry("getblockcount", nil)
	if err != nil {
		return 0, err
	}

	height, ok := resp.Result.(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected response type for getblockcount")
	}

	return int(height), nil
}

// GetBlockHash returns the hash of a block at the given height
func (c *RPCClient) GetBlockHash(height int) (string, error) {
	resp, err := c.callWithRetry("getblockhash", []interface{}{height})
	if err != nil {
		return "", err
	}

	hash, ok := resp.Result.(string)
	if !ok {
		return "", fmt.Errorf("unexpected response type for getblockhash")
	}

	return hash, nil
}

// GetBlock returns block information with transactions
func (c *RPCClient) GetBlock(hash string) (*Block, error) {
	resp, err := c.callWithRetry("getblock", []interface{}{hash, 2})
	if err != nil {
		return nil, err
	}

	// Parse the block data
	blockData, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response type for getblock")
	}

	// Convert to Block struct
	blockJSON, err := json.Marshal(blockData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal block data: %w", err)
	}

	var block Block
	if err := json.Unmarshal(blockJSON, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return &block, nil
}

// GetRawMempool returns mempool transaction IDs
func (c *RPCClient) GetRawMempool() ([]string, error) {
	resp, err := c.callWithRetry("getrawmempool", []interface{}{false})
	if err != nil {
		return nil, err
	}

	// Parse array of transaction IDs
	txids, ok := resp.Result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response type for getrawmempool")
	}

	result := make([]string, len(txids))
	for i, txid := range txids {
		if txidStr, ok := txid.(string); ok {
			result[i] = txidStr
		} else {
			return nil, fmt.Errorf("unexpected txid type in mempool")
		}
	}

	return result, nil
}

// GetRawTransaction returns transaction details
func (c *RPCClient) GetRawTransaction(txid string) (*Tx, error) {
	resp, err := c.callWithRetry("getrawtransaction", []interface{}{txid, true})
	if err != nil {
		return nil, err
	}

	// Parse transaction data
	txData, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response type for getrawtransaction")
	}

	// Convert to Tx struct
	txJSON, err := json.Marshal(txData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction data: %w", err)
	}

	var tx Tx
	if err := json.Unmarshal(txJSON, &tx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	return &tx, nil
}

// GetTxOut returns unspent transaction output information
func (c *RPCClient) GetTxOut(txid string, vout int) (map[string]interface{}, error) {
	resp, err := c.callWithRetry("gettxout", []interface{}{txid, vout})
	if err != nil {
		return nil, err
	}

	if resp.Result == nil {
		return nil, nil // UTXO is spent
	}

	txout, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response type for gettxout")
	}

	return txout, nil
}

// SetRetryConfig configures retry settings for the RPC client
func (c *RPCClient) SetRetryConfig(maxRetries int, baseDelay, maxDelay time.Duration) {
	c.maxRetries = maxRetries
	c.baseDelay = baseDelay
	c.maxDelay = maxDelay
}

// NewRPCClientWithRetry creates a new Bitcoin RPC client with custom retry configuration
func NewRPCClientWithRetry(baseURL, username, password string, maxRetries int, baseDelay, maxDelay time.Duration) *RPCClient {
	client := NewRPCClient(baseURL, username, password)
	client.SetRetryConfig(maxRetries, baseDelay, maxDelay)
	return client
}
