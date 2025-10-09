package database

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"bitcoin-indexer/internal/models"

	"github.com/lib/pq"
)

// isRetryableDBError checks if a database error is retryable
func isRetryableDBError(err error) bool {
	if err == nil {
		return false
	}

	// Check for PostgreSQL connection errors and deadlocks
	errStr := err.Error()
	return strings.Contains(errStr, "too many clients already") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection timeout") ||
		strings.Contains(errStr, "server closed the connection") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "deadlock detected") ||
		strings.Contains(errStr, "serialization failure")
}

// executeWithRetry executes a database operation with retry logic
func executeWithRetry(operation func() error, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// If not retryable or this is the last attempt, return the error
		if !isRetryableDBError(err) || attempt == maxRetries {
			return err
		}

		// Calculate exponential backoff delay with jitter for deadlocks
		baseDelay := time.Duration(1<<uint(attempt)) * time.Millisecond * 100 // Start with 100ms
		if baseDelay > 5*time.Second {
			baseDelay = 5 * time.Second
		}

		// Add jitter to prevent thundering herd
		jitter := time.Duration(rand.Intn(100)) * time.Millisecond
		delay := baseDelay + jitter

		// Log retry attempt with more detail for deadlocks
		if strings.Contains(err.Error(), "deadlock") {
			fmt.Printf("Deadlock detected (attempt %d/%d): %v. Retrying in %v with jitter...\n",
				attempt+1, maxRetries+1, err, delay)
		} else {
			fmt.Printf("Database operation failed (attempt %d/%d): %v. Retrying in %v...\n",
				attempt+1, maxRetries+1, err, delay)
		}

		time.Sleep(delay)
	}

	return lastErr
}

// DB wraps the database connection and provides methods for data access
type DB struct {
	conn *sql.DB
	// Prepared statements for better performance
	stmtCache map[string]*sql.Stmt
	// Connection pool settings
	maxOpenConns int
	maxIdleConns int
}

// NewDB creates a new database connection
func NewDB(databaseURL string) (*DB, error) {
	conn, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Dynamic connection pool sizing based on environment
	maxOpenConns := getIntEnv("DB_MAX_OPEN_CONNS", 500)               // Increased for 250 workers
	maxIdleConns := getIntEnv("DB_MAX_IDLE_CONNS", 250)               // Increased proportionally
	connMaxLifetime := getDurationEnv("DB_CONN_MAX_LIFETIME", "30m")  // Increased
	connMaxIdleTime := getDurationEnv("DB_CONN_MAX_IDLE_TIME", "15m") // Increased

	conn.SetMaxOpenConns(maxOpenConns)
	conn.SetMaxIdleConns(maxIdleConns)
	conn.SetConnMaxLifetime(connMaxLifetime)
	conn.SetConnMaxIdleTime(connMaxIdleTime)

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{
		conn:         conn,
		stmtCache:    make(map[string]*sql.Stmt),
		maxOpenConns: maxOpenConns,
		maxIdleConns: maxIdleConns,
	}

	// Initialize schema
	if err := db.InitSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Prepare commonly used statements
	if err := db.prepareStatements(); err != nil {
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	return db, nil
}

// Conn returns the underlying database connection for transaction management
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Add helper functions for environment variables
func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getDurationEnv(key, defaultValue string) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}

	duration, err := time.ParseDuration(defaultValue)
	if err != nil {
		panic(fmt.Sprintf("invalid default duration %s: %v", defaultValue, err))
	}
	return duration
}

// InitSchema creates the database schema
func (db *DB) InitSchema() error {
	schema := `
	-- Watch targets (addresses and/or xpub sources)
	CREATE TABLE IF NOT EXISTS watch_target (
	  id BIGSERIAL PRIMARY KEY,
	  kind TEXT NOT NULL CHECK (kind IN ('address','xpub')),
	  address TEXT,            -- for kind='address'
	  xpub TEXT,               -- for kind='xpub'
	  derivation_scheme TEXT,  -- e.g., 'bip84'
	  account INT,
	  gap_limit INT DEFAULT 20,
	  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	  UNIQUE (address)
	);

	-- Derived scripts we actively monitor (from addresses or xpub derivations)
	CREATE TABLE IF NOT EXISTS watched_script (
	  id BIGSERIAL PRIMARY KEY,
	  address TEXT NOT NULL,
	  script_hex TEXT NOT NULL,
	  type TEXT,               -- p2wpkh, p2tr, etc.
	  target_id BIGINT REFERENCES watch_target(id) ON DELETE CASCADE,
	  UNIQUE (script_hex)
	);

	-- UTXO mirror for our addresses
	CREATE TABLE IF NOT EXISTS utxo (
	  txid TEXT NOT NULL,
	  vout INT NOT NULL,
	  address TEXT NOT NULL,
	  script_hex TEXT NOT NULL,
	  value_sats BIGINT NOT NULL,
	  status TEXT NOT NULL CHECK (status IN ('mempool','confirmed','spent')),
	  block_height INT,
	  block_hash TEXT,
	  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	  spent_at TIMESTAMPTZ,
	  PRIMARY KEY (txid, vout)
	);

	-- Indexing progress and tip continuity
	CREATE TABLE IF NOT EXISTS index_progress (
	  id BOOLEAN PRIMARY KEY DEFAULT TRUE,
	  last_height INT,
	  last_block_hash TEXT,
	  updated_at TIMESTAMPTZ DEFAULT now()
	);

	-- Webhook subscriptions and outbox
	CREATE TABLE IF NOT EXISTS webhook_subscription (
	  id BIGSERIAL PRIMARY KEY,
	  address TEXT NOT NULL,
	  callback_url TEXT NOT NULL,
	  secret TEXT NOT NULL,
	  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);

	CREATE TABLE IF NOT EXISTS webhook_outbox (
	  id BIGSERIAL PRIMARY KEY,
	  event_type TEXT NOT NULL,           -- incoming_tx, confirmations_changed, confirmed, spent
	  address TEXT NOT NULL,
	  txid TEXT,
	  vout INT,
	  payload JSONB NOT NULL,
	  attempt_count INT DEFAULT 0,
	  next_attempt_at TIMESTAMPTZ DEFAULT now(),
	  status TEXT NOT NULL DEFAULT 'pending',
	  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);

	-- Indexes (performance)
	-- Note: PRIMARY KEY (txid, vout) already creates an index, so idx_utxo_txid_vout is redundant
	CREATE INDEX IF NOT EXISTS idx_utxo_address        ON utxo (address);
	CREATE INDEX IF NOT EXISTS idx_utxo_blockhash      ON utxo (block_hash);
	CREATE INDEX IF NOT EXISTS idx_utxo_block_height   ON utxo (block_height);
	
	-- Partial indexes for better performance on status filtering
	CREATE INDEX IF NOT EXISTS idx_utxo_status_confirmed ON utxo (txid, vout) WHERE status = 'confirmed';
	CREATE INDEX IF NOT EXISTS idx_utxo_status_unspent  ON utxo (address, value_sats) WHERE status IN ('confirmed', 'mempool');
	
	-- Composite indexes for common query patterns
	CREATE INDEX IF NOT EXISTS idx_utxo_address_status  ON utxo (address, status, block_height);
	
	CREATE INDEX IF NOT EXISTS idx_webhook_outbox_sched ON webhook_outbox (status, next_attempt_at);
	CREATE INDEX IF NOT EXISTS idx_webhook_outbox_addr  ON webhook_outbox (address);
	CREATE INDEX IF NOT EXISTS idx_webhook_outbox_txid  ON webhook_outbox (txid);
	
	CREATE INDEX IF NOT EXISTS idx_watched_script_address ON watched_script (address);
	CREATE INDEX IF NOT EXISTS idx_watched_script_hex ON watched_script (script_hex);
	CREATE INDEX IF NOT EXISTS idx_watched_script_type  ON watched_script (type);
	CREATE INDEX IF NOT EXISTS idx_watched_script_target ON watched_script (target_id);
	
	-- Transaction indexes
	CREATE INDEX IF NOT EXISTS idx_transaction_txid    ON transaction (txid);
	CREATE INDEX IF NOT EXISTS idx_transaction_block   ON transaction (block_height);
	CREATE INDEX IF NOT EXISTS idx_transaction_hash    ON transaction (block_hash);
	CREATE INDEX IF NOT EXISTS idx_transaction_time    ON transaction (block_time);
	CREATE INDEX IF NOT EXISTS idx_transaction_coinbase ON transaction (is_coinbase);
	
	-- Transaction input indexes
	CREATE INDEX IF NOT EXISTS idx_tx_input_txid       ON transaction_input (txid);
	CREATE INDEX IF NOT EXISTS idx_tx_input_prev       ON transaction_input (prev_txid, prev_vout);
	CREATE INDEX IF NOT EXISTS idx_tx_input_vout       ON transaction_input (txid, vout);
	
	-- Transaction output indexes
	CREATE INDEX IF NOT EXISTS idx_tx_output_txid      ON transaction_output (txid);
	CREATE INDEX IF NOT EXISTS idx_tx_output_address  ON transaction_output (address);
	CREATE INDEX IF NOT EXISTS idx_tx_output_vout      ON transaction_output (txid, vout);
	CREATE INDEX IF NOT EXISTS idx_tx_output_script    ON transaction_output (script_hex);
	CREATE INDEX IF NOT EXISTS idx_tx_output_type      ON transaction_output (script_type);
	
	-- Transaction reference indexes
	CREATE INDEX IF NOT EXISTS idx_tx_ref_txid         ON tx_reference (txid);
	CREATE INDEX IF NOT EXISTS idx_tx_ref_address      ON tx_reference (address);
	CREATE INDEX IF NOT EXISTS idx_tx_ref_direction    ON tx_reference (direction);
	CREATE INDEX IF NOT EXISTS idx_tx_ref_block        ON tx_reference (block_height);
	CREATE INDEX IF NOT EXISTS idx_tx_ref_created      ON tx_reference (created_at);
	CREATE INDEX IF NOT EXISTS idx_tx_ref_composite    ON tx_reference (address, direction, block_height);
	
	-- Block info indexes
	CREATE INDEX IF NOT EXISTS idx_block_height        ON block_info (height);
	CREATE INDEX IF NOT EXISTS idx_block_hash          ON block_info (hash);
	CREATE INDEX IF NOT EXISTS idx_block_time          ON block_info (timestamp);
	CREATE INDEX IF NOT EXISTS idx_block_created       ON block_info (created_at);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// prepareStatements prepares commonly used SQL statements for better performance
func (db *DB) prepareStatements() error {
	statements := map[string]string{
		"upsert_utxo": `
			INSERT INTO utxo (txid, vout, address, script_hex, value_sats, status, block_height, block_hash, first_seen_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (txid, vout) 
			DO UPDATE SET 
				status = EXCLUDED.status,
				block_height = EXCLUDED.block_height,
				block_hash = EXCLUDED.block_hash
		`,
		"mark_utxo_spent": `
			UPDATE utxo 
			SET status = 'spent', spent_at = $3, spent_by_txid = $4
			WHERE txid = $1 AND vout = $2
		`,
		"get_utxo": `
			SELECT txid, vout, address, script_hex, value_sats, status, block_height, block_hash, first_seen_at, spent_at, spent_by_txid
			FROM utxo 
			WHERE txid = $1 AND vout = $2 AND status IN ('confirmed', 'mempool')
		`,
		"is_script_watched": `
			SELECT id, address, script_hex, type, target_id FROM watched_script WHERE script_hex = $1
		`,
		"transaction_exists": `
			SELECT 1 FROM transaction WHERE txid = $1 LIMIT 1
		`,
		"store_transaction": `
			INSERT INTO transaction (txid, block_height, block_hash, block_time, size, weight, fee_sats, is_coinbase, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (txid) DO UPDATE SET
				block_height = EXCLUDED.block_height,
				block_hash = EXCLUDED.block_hash,
				block_time = EXCLUDED.block_time,
				size = EXCLUDED.size,
				weight = EXCLUDED.weight,
				fee_sats = EXCLUDED.fee_sats,
				is_coinbase = EXCLUDED.is_coinbase,
				updated_at = EXCLUDED.updated_at
		`,
		"store_transaction_input": `
			INSERT INTO transaction_input (txid, vout, prev_txid, prev_vout, script_sig, sequence, witness)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (txid, vout) DO UPDATE SET
				prev_txid = EXCLUDED.prev_txid,
				prev_vout = EXCLUDED.prev_vout,
				script_sig = EXCLUDED.script_sig,
				sequence = EXCLUDED.sequence,
				witness = EXCLUDED.witness
		`,
		"store_transaction_output": `
			INSERT INTO transaction_output (txid, vout, address, value_sats, script_type, script_hex, script_asm)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (txid, vout) DO UPDATE SET
				address = EXCLUDED.address,
				value_sats = EXCLUDED.value_sats,
				script_type = EXCLUDED.script_type,
				script_hex = EXCLUDED.script_hex,
				script_asm = EXCLUDED.script_asm
		`,
		"upsert_tx_reference": `
			INSERT INTO tx_reference (txid, address, direction, value_sats, sender_address, receiver_address, block_height, block_timestamp, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (txid, address, direction) 
			DO UPDATE SET 
				value_sats = EXCLUDED.value_sats,
				sender_address = EXCLUDED.sender_address,
				receiver_address = EXCLUDED.receiver_address,
				block_height = EXCLUDED.block_height,
				block_timestamp = EXCLUDED.block_timestamp
		`,
	}

	for name, query := range statements {
		stmt, err := db.conn.Prepare(query)
		if err != nil {
			return fmt.Errorf("failed to prepare statement %s: %w", name, err)
		}
		db.stmtCache[name] = stmt
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	// Close all prepared statements
	for _, stmt := range db.stmtCache {
		stmt.Close()
	}
	return db.conn.Close()
}

// Watchlist Management

// AddWatchTarget adds a new watch target (address or xpub)
func (db *DB) AddWatchTarget(target *models.WatchTarget) error {
	query := `
		INSERT INTO watch_target (kind, address, xpub, derivation_scheme, account, gap_limit)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`

	err := db.conn.QueryRow(
		query,
		target.Kind,
		target.Address,
		target.Xpub,
		target.DerivationScheme,
		target.Account,
		target.GapLimit,
	).Scan(&target.ID, &target.CreatedAt)

	return err
}

// GetWatchTargets returns all watch targets
func (db *DB) GetWatchTargets() ([]*models.WatchTarget, error) {
	query := `SELECT id, kind, address, xpub, derivation_scheme, account, gap_limit, created_at FROM watch_target`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []*models.WatchTarget
	for rows.Next() {
		target := &models.WatchTarget{}
		err := rows.Scan(
			&target.ID,
			&target.Kind,
			&target.Address,
			&target.Xpub,
			&target.DerivationScheme,
			&target.Account,
			&target.GapLimit,
			&target.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}

	return targets, nil
}

// AddWatchedScript adds a script to monitor
func (db *DB) AddWatchedScript(script *models.WatchedScript) error {
	query := `
		INSERT INTO watched_script (address, script_hex, type, target_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (script_hex) DO NOTHING
		RETURNING id
	`

	err := db.conn.QueryRow(query, script.Address, script.ScriptHex, script.Type, script.TargetID).Scan(&script.ID)
	if err == sql.ErrNoRows {
		// Script already exists, get its ID
		query = `SELECT id FROM watched_script WHERE script_hex = $1`
		err = db.conn.QueryRow(query, script.ScriptHex).Scan(&script.ID)
	}

	return err
}

// GetWatchedScripts returns all watched scripts
func (db *DB) GetWatchedScripts() ([]*models.WatchedScript, error) {
	query := `SELECT id, address, script_hex, type, target_id FROM watched_script`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scripts []*models.WatchedScript
	for rows.Next() {
		script := &models.WatchedScript{}
		err := rows.Scan(&script.ID, &script.Address, &script.ScriptHex, &script.Type, &script.TargetID)
		if err != nil {
			return nil, err
		}
		scripts = append(scripts, script)
	}

	return scripts, nil
}

// IsScriptWatched checks if a script is being monitored
func (db *DB) IsScriptWatched(scriptHex string) (bool, *models.WatchedScript, error) {
	query := `SELECT id, address, script_hex, type, target_id FROM watched_script WHERE script_hex = $1`

	script := &models.WatchedScript{}
	err := db.conn.QueryRow(query, scriptHex).Scan(
		&script.ID,
		&script.Address,
		&script.ScriptHex,
		&script.Type,
		&script.TargetID,
	)

	if err == sql.ErrNoRows {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}

	return true, script, nil
}

// UTXO Management

// UpsertUTXO creates or updates a UTXO
func (db *DB) UpsertUTXO(utxo *models.UTXO) error {
	// First, ensure the transaction exists
	// This prevents foreign key constraint violations
	txExists, err := db.TransactionExists(utxo.Txid)
	if err != nil {
		return fmt.Errorf("failed to check if transaction exists: %w", err)
	}
	if !txExists {
		return fmt.Errorf("transaction %s does not exist, cannot create UTXO", utxo.Txid)
	}

	stmt := db.stmtCache["upsert_utxo"]
	_, err = stmt.Exec(
		utxo.Txid,
		utxo.Vout,
		utxo.Address,
		utxo.ScriptHex,
		utxo.ValueSats,
		utxo.Status,
		utxo.BlockHeight,
		utxo.BlockHash,
		utxo.FirstSeenAt,
	)

	return err
}

// MarkUTXOSpent marks a UTXO as spent
func (db *DB) MarkUTXOSpent(txid string, vout int, spentAt time.Time, spentByTxid *string) error {
	stmt := db.stmtCache["mark_utxo_spent"]
	_, err := stmt.Exec(txid, vout, spentAt, spentByTxid)
	return err
}

// GetUTXO returns a specific UTXO by txid and vout
func (db *DB) GetUTXO(txid string, vout int) (*models.UTXO, error) {
	stmt := db.stmtCache["get_utxo"]
	utxo := &models.UTXO{}
	err := stmt.QueryRow(txid, vout).Scan(
		&utxo.Txid,
		&utxo.Vout,
		&utxo.Address,
		&utxo.ScriptHex,
		&utxo.ValueSats,
		&utxo.Status,
		&utxo.BlockHeight,
		&utxo.BlockHash,
		&utxo.FirstSeenAt,
		&utxo.SpentAt,
		&utxo.SpentByTxid,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return utxo, nil
}

// UTXORef represents a reference to a UTXO (txid + vout)
type UTXORef struct {
	Txid string
	Vout int
}

// BatchGetUTXOs retrieves multiple UTXOs in a single query
func (db *DB) BatchGetUTXOs(refs []UTXORef) (map[string]*models.UTXO, error) {
	if len(refs) == 0 {
		return make(map[string]*models.UTXO), nil
	}

	// Build arrays for the query
	txids := make([]string, len(refs))
	vouts := make([]int, len(refs))

	for i, ref := range refs {
		txids[i] = ref.Txid
		vouts[i] = ref.Vout
	}

	// Use UNNEST to query multiple UTXOs efficiently
	query := `
		SELECT txid, vout, address, script_hex, value_sats, status, 
		       block_height, block_hash, first_seen_at, spent_at, spent_by_txid
		FROM utxo
		WHERE (txid, vout) IN (
			SELECT UNNEST($1::text[]), UNNEST($2::int[])
		)
		AND status IN ('confirmed', 'mempool')
	`

	rows, err := db.conn.Query(query, pq.Array(txids), pq.Array(vouts))
	if err != nil {
		return nil, fmt.Errorf("failed to batch get UTXOs: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*models.UTXO)
	found := 0
	for rows.Next() {
		utxo := &models.UTXO{}
		err := rows.Scan(
			&utxo.Txid,
			&utxo.Vout,
			&utxo.Address,
			&utxo.ScriptHex,
			&utxo.ValueSats,
			&utxo.Status,
			&utxo.BlockHeight,
			&utxo.BlockHash,
			&utxo.FirstSeenAt,
			&utxo.SpentAt,
			&utxo.SpentByTxid,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan UTXO: %w", err)
		}

		// Use txid:vout as key for fast lookup
		key := fmt.Sprintf("%s:%d", utxo.Txid, utxo.Vout)
		result[key] = utxo
		found++
	}

	// Bulk UTXO query completed (metrics collected internally)

	return result, rows.Err()
}

// GetUTXOsForAddress returns UTXOs for a specific address
func (db *DB) GetUTXOsForAddress(address string) ([]*models.UTXO, error) {
	query := `
		SELECT txid, vout, address, script_hex, value_sats, status, block_height, block_hash, first_seen_at, spent_at, spent_by_txid
		FROM utxo 
		WHERE address = $1
		ORDER BY first_seen_at DESC
	`

	rows, err := db.conn.Query(query, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var utxos []*models.UTXO
	for rows.Next() {
		utxo := &models.UTXO{}
		err := rows.Scan(
			&utxo.Txid,
			&utxo.Vout,
			&utxo.Address,
			&utxo.ScriptHex,
			&utxo.ValueSats,
			&utxo.Status,
			&utxo.BlockHeight,
			&utxo.BlockHash,
			&utxo.FirstSeenAt,
			&utxo.SpentAt,
			&utxo.SpentByTxid,
		)
		if err != nil {
			return nil, err
		}
		utxos = append(utxos, utxo)
	}

	return utxos, nil
}

// GetBalance returns balance information for an address
func (db *DB) GetBalance(address string, confirmationsRequired int) (*models.Balance, error) {
	// Get current tip height for confirmation calculation
	var tipHeight int
	err := db.conn.QueryRow("SELECT last_height FROM index_progress WHERE id = true").Scan(&tipHeight)
	if err != nil {
		return nil, err
	}

	// Calculate pending and confirmed balances
	query := `
		SELECT 
			SUM(CASE WHEN status = 'mempool' OR (status = 'confirmed' AND block_height > $2) THEN value_sats ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'confirmed' AND block_height <= $2 THEN value_sats ELSE 0 END) as confirmed
		FROM utxo 
		WHERE address = $1 AND status IN ('mempool', 'confirmed')
	`

	var pending, confirmed sql.NullInt64
	err = db.conn.QueryRow(query, address, tipHeight-confirmationsRequired+1).Scan(&pending, &confirmed)
	if err != nil {
		return nil, err
	}

	balance := &models.Balance{
		Address:           address,
		Pending:           pending.Int64,
		Confirmed:         confirmed.Int64,
		Total:             pending.Int64 + confirmed.Int64,
		LastIndexedHeight: tipHeight,
	}

	return balance, nil
}

// Index Progress Management

// UpdateIndexProgress updates the indexing progress
func (db *DB) UpdateIndexProgress(height int, blockHash string) error {
	query := `
		INSERT INTO index_progress (id, last_height, last_block_hash, updated_at)
		VALUES (true, $1, $2, $3)
		ON CONFLICT (id) 
		DO UPDATE SET 
			last_height = EXCLUDED.last_height,
			last_block_hash = EXCLUDED.last_block_hash,
			updated_at = EXCLUDED.updated_at
	`

	_, err := db.conn.Exec(query, height, blockHash, time.Now())
	return err
}

// UpdateIndexProgressInTx updates the indexing progress within a transaction
func (db *DB) UpdateIndexProgressInTx(tx *sql.Tx, height int, blockHash string) error {
	query := `
		INSERT INTO index_progress (id, last_height, last_block_hash, updated_at)
		VALUES (true, $1, $2, $3)
		ON CONFLICT (id) 
		DO UPDATE SET 
			last_height = EXCLUDED.last_height,
			last_block_hash = EXCLUDED.last_block_hash,
			updated_at = EXCLUDED.updated_at
	`

	_, err := tx.Exec(query, height, blockHash, time.Now())
	return err
}

// GetIndexProgress returns the current indexing progress
func (db *DB) GetIndexProgress() (*models.IndexProgress, error) {
	query := `SELECT id, last_height, last_block_hash, updated_at FROM index_progress WHERE id = true`

	progress := &models.IndexProgress{}
	err := db.conn.QueryRow(query).Scan(
		&progress.ID,
		&progress.LastHeight,
		&progress.LastBlockHash,
		&progress.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// No progress recorded yet
		return &models.IndexProgress{
			ID:            true,
			LastHeight:    0,
			LastBlockHash: "",
			UpdatedAt:     time.Now(),
		}, nil
	}

	return progress, err
}

// Reorg Handling

// DeleteUTXOsByBlockHash removes UTXOs from a specific block (for reorg handling)
func (db *DB) DeleteUTXOsByBlockHash(blockHash string) error {
	query := `DELETE FROM utxo WHERE block_hash = $1`
	_, err := db.conn.Exec(query, blockHash)
	return err
}

// UpsertTransactionReference creates or updates a transaction reference
func (db *DB) UpsertTransactionReference(txRef *models.TransactionReference) error {
	query := `
		INSERT INTO tx_reference (txid, address, direction, value_sats, sender_address, receiver_address, block_height, block_timestamp, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (txid, address, direction) 
		DO UPDATE SET 
			value_sats = EXCLUDED.value_sats,
			sender_address = EXCLUDED.sender_address,
			receiver_address = EXCLUDED.receiver_address,
			block_height = EXCLUDED.block_height,
			block_timestamp = EXCLUDED.block_timestamp
	`

	_, err := db.conn.Exec(
		query,
		txRef.Txid,
		txRef.Address,
		txRef.Direction,
		txRef.ValueSats,
		txRef.SenderAddress,
		txRef.ReceiverAddress,
		txRef.BlockHeight,
		txRef.BlockTimestamp,
		txRef.CreatedAt,
	)

	return err
}

// TransactionExists checks if a transaction exists in the database
func (db *DB) TransactionExists(txid string) (bool, error) {
	query := `SELECT 1 FROM transaction WHERE txid = $1 LIMIT 1`

	var exists int
	err := db.conn.QueryRow(query, txid).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetTransactionReferences returns transaction references for an address
func (db *DB) GetTransactionReferences(address string, limit int) ([]*models.TransactionReference, error) {
	query := `
		SELECT id, txid, address, direction, value_sats, sender_address, receiver_address, block_height, block_timestamp, created_at
		FROM tx_reference 
		WHERE address = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := db.conn.Query(query, address, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txRefs []*models.TransactionReference
	for rows.Next() {
		txRef := &models.TransactionReference{}
		err := rows.Scan(
			&txRef.ID,
			&txRef.Txid,
			&txRef.Address,
			&txRef.Direction,
			&txRef.ValueSats,
			&txRef.SenderAddress,
			&txRef.ReceiverAddress,
			&txRef.BlockHeight,
			&txRef.BlockTimestamp,
			&txRef.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		txRefs = append(txRefs, txRef)
	}

	return txRefs, nil
}

// GetTransactionHistory returns complete transaction history for an address
func (db *DB) GetTransactionHistory(address string, limit int) ([]*models.TransactionReference, error) {
	// This would return both incoming and outgoing transactions
	return db.GetTransactionReferences(address, limit)
}

// Comprehensive Transaction Storage Methods

// StoreTransaction stores a complete transaction
func (db *DB) StoreTransaction(tx *models.Transaction) error {
	query := `
		INSERT INTO transaction (txid, block_height, block_hash, block_time, size, weight, fee_sats, is_coinbase, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (txid) DO UPDATE SET
			block_height = EXCLUDED.block_height,
			block_hash = EXCLUDED.block_hash,
			block_time = EXCLUDED.block_time,
			size = EXCLUDED.size,
			weight = EXCLUDED.weight,
			fee_sats = EXCLUDED.fee_sats,
			is_coinbase = EXCLUDED.is_coinbase,
			updated_at = EXCLUDED.updated_at
	`

	_, err := db.conn.Exec(
		query,
		tx.Txid,
		tx.BlockHeight,
		tx.BlockHash,
		tx.BlockTime,
		tx.Size,
		tx.Weight,
		tx.FeeSats,
		tx.IsCoinbase,
		tx.CreatedAt,
		time.Now(),
	)

	return err
}

// StoreTransactionInput stores a transaction input
func (db *DB) StoreTransactionInput(input *models.TransactionInput) error {
	query := `
		INSERT INTO transaction_input (txid, vout, prev_txid, prev_vout, script_sig, sequence, witness)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (txid, vout) DO UPDATE SET
			prev_txid = EXCLUDED.prev_txid,
			prev_vout = EXCLUDED.prev_vout,
			script_sig = EXCLUDED.script_sig,
			sequence = EXCLUDED.sequence,
			witness = EXCLUDED.witness
	`

	// Handle witness field - convert nil to empty array for PostgreSQL
	witness := input.Witness
	if witness == nil {
		witness = []string{}
	}

	_, err := db.conn.Exec(
		query,
		input.Txid,
		input.Vout,
		input.PrevTxid,
		input.PrevVout,
		input.ScriptSig,
		input.Sequence,
		pq.Array(witness),
	)

	return err
}

// StoreTransactionOutput stores a transaction output
func (db *DB) StoreTransactionOutput(output *models.TransactionOutput) error {
	query := `
		INSERT INTO transaction_output (txid, vout, address, value_sats, script_type, script_hex, script_asm)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (txid, vout) DO UPDATE SET
			address = EXCLUDED.address,
			value_sats = EXCLUDED.value_sats,
			script_type = EXCLUDED.script_type,
			script_hex = EXCLUDED.script_hex,
			script_asm = EXCLUDED.script_asm
	`

	_, err := db.conn.Exec(
		query,
		output.Txid,
		output.Vout,
		output.Address,
		output.ValueSats,
		output.ScriptType,
		output.ScriptHex,
		output.ScriptAsm,
	)

	return err
}

// StoreBlockInfo stores block information
func (db *DB) StoreBlockInfo(block *models.BlockInfo) error {
	query := `
		INSERT INTO block_info (height, hash, previous_hash, timestamp, size, weight, transaction_count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (height) DO UPDATE SET
			hash = EXCLUDED.hash,
			previous_hash = EXCLUDED.previous_hash,
			timestamp = EXCLUDED.timestamp,
			size = EXCLUDED.size,
			weight = EXCLUDED.weight,
			transaction_count = EXCLUDED.transaction_count
	`

	_, err := db.conn.Exec(
		query,
		block.Height,
		block.Hash,
		block.PreviousHash,
		block.Timestamp,
		block.Size,
		block.Weight,
		block.TransactionCount,
		block.CreatedAt,
	)

	return err
}

// GetTransactionDetails returns complete transaction details
func (db *DB) GetTransactionDetails(txid string) (*models.Transaction, []*models.TransactionInput, []*models.TransactionOutput, error) {
	// Get transaction
	tx := &models.Transaction{}
	err := db.conn.QueryRow(`
		SELECT txid, block_height, block_hash, block_time, size, weight, fee_sats, is_coinbase, created_at, updated_at
		FROM transaction WHERE txid = $1
	`, txid).Scan(
		&tx.Txid, &tx.BlockHeight, &tx.BlockHash, &tx.BlockTime, &tx.Size, &tx.Weight, &tx.FeeSats, &tx.IsCoinbase, &tx.CreatedAt, &tx.UpdatedAt,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	// Get inputs
	inputs, err := db.getTransactionInputs(txid)
	if err != nil {
		return nil, nil, nil, err
	}

	// Get outputs
	outputs, err := db.getTransactionOutputs(txid)
	if err != nil {
		return nil, nil, nil, err
	}

	return tx, inputs, outputs, nil
}

// getTransactionInputs gets all inputs for a transaction
func (db *DB) getTransactionInputs(txid string) ([]*models.TransactionInput, error) {
	query := `
		SELECT txid, vout, prev_txid, prev_vout, script_sig, sequence, witness
		FROM transaction_input WHERE txid = $1 ORDER BY vout
	`

	rows, err := db.conn.Query(query, txid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var inputs []*models.TransactionInput
	for rows.Next() {
		input := &models.TransactionInput{}
		var witness pq.StringArray
		err := rows.Scan(
			&input.Txid, &input.Vout, &input.PrevTxid, &input.PrevVout, &input.ScriptSig, &input.Sequence, &witness,
		)
		if err != nil {
			return nil, err
		}
		input.Witness = []string(witness)
		inputs = append(inputs, input)
	}

	return inputs, nil
}

// getTransactionOutputs gets all outputs for a transaction
func (db *DB) getTransactionOutputs(txid string) ([]*models.TransactionOutput, error) {
	query := `
		SELECT txid, vout, address, value_sats, script_type, script_hex, script_asm
		FROM transaction_output WHERE txid = $1 ORDER BY vout
	`

	rows, err := db.conn.Query(query, txid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var outputs []*models.TransactionOutput
	for rows.Next() {
		output := &models.TransactionOutput{}
		err := rows.Scan(
			&output.Txid, &output.Vout, &output.Address, &output.ValueSats, &output.ScriptType, &output.ScriptHex, &output.ScriptAsm,
		)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}

	return outputs, nil
}

// GetAddressTransactions returns all transactions for an address
func (db *DB) GetAddressTransactions(address string, limit int) ([]*models.Transaction, error) {
	query := `
		SELECT DISTINCT t.txid, t.block_height, t.block_hash, t.block_time, t.size, t.weight, t.fee_sats, t.is_coinbase, t.created_at, t.updated_at
		FROM transaction t
		JOIN transaction_output o ON t.txid = o.txid
		WHERE o.address = $1
		ORDER BY t.block_time DESC, t.created_at DESC
		LIMIT $2
	`

	rows, err := db.conn.Query(query, address, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []*models.Transaction
	for rows.Next() {
		tx := &models.Transaction{}
		err := rows.Scan(
			&tx.Txid, &tx.BlockHeight, &tx.BlockHash, &tx.BlockTime, &tx.Size, &tx.Weight, &tx.FeeSats, &tx.IsCoinbase, &tx.CreatedAt, &tx.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// GetTransactionSenders returns sender information for a transaction
func (db *DB) GetTransactionSenders(txid string) ([]*SenderInfo, error) {
	query := `
		SELECT 
			i.prev_txid,
			i.prev_vout,
			o.address as sender_address,
			o.value_sats as sender_amount,
			o.script_type
		FROM transaction_input i
		JOIN transaction_output o ON i.prev_txid = o.txid AND i.prev_vout = o.vout
		WHERE i.txid = $1
		ORDER BY i.vout
	`

	rows, err := db.conn.Query(query, txid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var senders []*SenderInfo
	for rows.Next() {
		sender := &SenderInfo{}
		err := rows.Scan(
			&sender.PrevTxid, &sender.PrevVout, &sender.Address, &sender.ValueSats, &sender.ScriptType,
		)
		if err != nil {
			return nil, err
		}
		senders = append(senders, sender)
	}

	return senders, nil
}

// SenderInfo represents sender information
type SenderInfo struct {
	PrevTxid   string `json:"prev_txid" db:"prev_txid"`
	PrevVout   int    `json:"prev_vout" db:"prev_vout"`
	Address    string `json:"address" db:"sender_address"`
	ValueSats  int64  `json:"value_sats" db:"sender_amount"`
	ScriptType string `json:"script_type" db:"script_type"`
}

// Batch Processing Methods for Performance Optimization

// sortUTXOsByTxidVout sorts UTXOs by (txid, vout) to ensure consistent lock ordering and prevent deadlocks
func sortUTXOsByTxidVout(utxos []*models.UTXO) {
	sort.Slice(utxos, func(i, j int) bool {
		if utxos[i].Txid != utxos[j].Txid {
			return utxos[i].Txid < utxos[j].Txid
		}
		return utxos[i].Vout < utxos[j].Vout
	})
}

// BatchUpsertUTXOs processes multiple UTXOs in a single transaction with deadlock prevention
func (db *DB) BatchUpsertUTXOs(utxos []*models.UTXO) error {
	if len(utxos) == 0 {
		return nil
	}

	// Sort UTXOs by (txid, vout) to ensure consistent lock ordering and prevent deadlocks
	sortedUTXOs := make([]*models.UTXO, len(utxos))
	copy(sortedUTXOs, utxos)
	sortUTXOsByTxidVout(sortedUTXOs)

	return executeWithRetry(func() error {
		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback()

		stmt := tx.Stmt(db.stmtCache["upsert_utxo"])
		defer stmt.Close()

		// Process UTXOs in sorted order to prevent deadlocks
		for _, utxo := range sortedUTXOs {
			_, err := stmt.Exec(
				utxo.Txid,
				utxo.Vout,
				utxo.Address,
				utxo.ScriptHex,
				utxo.ValueSats,
				utxo.Status,
				utxo.BlockHeight,
				utxo.BlockHash,
				utxo.FirstSeenAt,
			)
			if err != nil {
				return fmt.Errorf("failed to upsert UTXO %s:%d: %w", utxo.Txid, utxo.Vout, err)
			}
		}

		return tx.Commit()
	}, 5) // Increased retries for deadlock scenarios
}

// BatchMarkUTXOsSpent marks multiple UTXOs as spent in a single transaction with deadlock prevention
func (db *DB) BatchMarkUTXOsSpent(spentUTXOs []*models.UTXO) error {
	if len(spentUTXOs) == 0 {
		return nil
	}

	// Sort UTXOs by (txid, vout) to ensure consistent lock ordering and prevent deadlocks
	sortedUTXOs := make([]*models.UTXO, len(spentUTXOs))
	copy(sortedUTXOs, spentUTXOs)
	sortUTXOsByTxidVout(sortedUTXOs)

	return executeWithRetry(func() error {
		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback()

		// Set transaction isolation level to prevent deadlocks
		_, err = tx.Exec("SET TRANSACTION ISOLATION LEVEL READ COMMITTED")
		if err != nil {
			return fmt.Errorf("failed to set transaction isolation level: %w", err)
		}

		stmt := tx.Stmt(db.stmtCache["mark_utxo_spent"])
		defer stmt.Close()

		// Process UTXOs in sorted order to prevent deadlocks
		for _, utxo := range sortedUTXOs {
			_, err := stmt.Exec(utxo.Txid, utxo.Vout, utxo.SpentAt, utxo.SpentByTxid)
			if err != nil {
				return fmt.Errorf("failed to mark UTXO as spent %s:%d: %w", utxo.Txid, utxo.Vout, err)
			}
		}

		return tx.Commit()
	}, 5) // Increased retries for deadlock scenarios
}

// BatchStoreTransactions stores multiple transactions with their inputs and outputs
func (db *DB) BatchStoreTransactions(transactions []*models.Transaction, inputs []*models.TransactionInput, outputs []*models.TransactionOutput) error {
	if len(transactions) == 0 {
		return nil
	}

	return executeWithRetry(func() error {
		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback()

		// Set transaction isolation level to prevent deadlocks
		_, err = tx.Exec("SET TRANSACTION ISOLATION LEVEL READ COMMITTED")
		if err != nil {
			return fmt.Errorf("failed to set transaction isolation level: %w", err)
		}

		// Store transactions using bulk VALUES insert with batching
		if len(transactions) > 0 {
			// PostgreSQL has a limit of 65535 parameters, so we need to batch
			// Each transaction uses 10 parameters, so max batch size is 6553 transactions
			batchSize := 6000 // Conservative batch size

			for i := 0; i < len(transactions); i += batchSize {
				end := i + batchSize
				if end > len(transactions) {
					end = len(transactions)
				}

				batch := transactions[i:end]
				query := `
					INSERT INTO transaction (txid, block_height, block_hash, block_time, size, weight, fee_sats, is_coinbase, created_at, updated_at)
					VALUES `

				values := make([]string, len(batch))
				args := make([]interface{}, 0, len(batch)*10)

				for j, transaction := range batch {
					values[j] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
						j*10+1, j*10+2, j*10+3, j*10+4, j*10+5, j*10+6, j*10+7, j*10+8, j*10+9, j*10+10)

					args = append(args,
						transaction.Txid,
						transaction.BlockHeight,
						transaction.BlockHash,
						transaction.BlockTime,
						transaction.Size,
						transaction.Weight,
						transaction.FeeSats,
						transaction.IsCoinbase,
						transaction.CreatedAt,
						time.Now(),
					)
				}

				query += strings.Join(values, ", ")
				query += ` ON CONFLICT (txid) DO UPDATE SET
					block_height = EXCLUDED.block_height,
					block_hash = EXCLUDED.block_hash,
					block_time = EXCLUDED.block_time,
					size = EXCLUDED.size,
					weight = EXCLUDED.weight,
					fee_sats = EXCLUDED.fee_sats,
					is_coinbase = EXCLUDED.is_coinbase,
					updated_at = EXCLUDED.updated_at`

				_, err = tx.Exec(query, args...)
				if err != nil {
					return fmt.Errorf("failed to bulk store transactions batch %d-%d: %w", i, end, err)
				}
			}
		}

		// Store inputs using bulk VALUES insert with batching
		if len(inputs) > 0 {
			// Each input uses 7 parameters, so max batch size is 9360 inputs
			batchSize := 8000 // Conservative batch size

			for i := 0; i < len(inputs); i += batchSize {
				end := i + batchSize
				if end > len(inputs) {
					end = len(inputs)
				}

				batch := inputs[i:end]
				query := `
					INSERT INTO transaction_input (txid, vout, prev_txid, prev_vout, script_sig, sequence, witness)
					VALUES `

				values := make([]string, len(batch))
				args := make([]interface{}, 0, len(batch)*7)

				for j, input := range batch {
					values[j] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
						j*7+1, j*7+2, j*7+3, j*7+4, j*7+5, j*7+6, j*7+7)

					// Handle witness field - convert nil to empty array for PostgreSQL
					witness := input.Witness
					if witness == nil {
						witness = []string{}
					}

					args = append(args,
						input.Txid,
						input.Vout,
						input.PrevTxid,
						input.PrevVout,
						input.ScriptSig,
						input.Sequence,
						pq.Array(witness),
					)
				}

				query += strings.Join(values, ", ")
				query += ` ON CONFLICT (txid, vout) DO UPDATE SET
					prev_txid = EXCLUDED.prev_txid,
					prev_vout = EXCLUDED.prev_vout,
					script_sig = EXCLUDED.script_sig,
					sequence = EXCLUDED.sequence,
					witness = EXCLUDED.witness`

				_, err = tx.Exec(query, args...)
				if err != nil {
					return fmt.Errorf("failed to bulk store transaction inputs batch %d-%d: %w", i, end, err)
				}
			}
		}

		// Store outputs using bulk VALUES insert with batching
		if len(outputs) > 0 {
			// Each output uses 7 parameters, so max batch size is 9360 outputs
			batchSize := 8000 // Conservative batch size

			for i := 0; i < len(outputs); i += batchSize {
				end := i + batchSize
				if end > len(outputs) {
					end = len(outputs)
				}

				batch := outputs[i:end]
				query := `
					INSERT INTO transaction_output (txid, vout, address, value_sats, script_type, script_hex, script_asm)
					VALUES `

				values := make([]string, len(batch))
				args := make([]interface{}, 0, len(batch)*7)

				for j, output := range batch {
					values[j] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
						j*7+1, j*7+2, j*7+3, j*7+4, j*7+5, j*7+6, j*7+7)

					args = append(args,
						output.Txid,
						output.Vout,
						output.Address,
						output.ValueSats,
						output.ScriptType,
						output.ScriptHex,
						output.ScriptAsm,
					)
				}

				query += strings.Join(values, ", ")
				query += ` ON CONFLICT (txid, vout) DO UPDATE SET
					address = EXCLUDED.address,
					value_sats = EXCLUDED.value_sats,
					script_type = EXCLUDED.script_type,
					script_hex = EXCLUDED.script_hex,
					script_asm = EXCLUDED.script_asm`

				_, err = tx.Exec(query, args...)
				if err != nil {
					return fmt.Errorf("failed to bulk store transaction outputs batch %d-%d: %w", i, end, err)
				}
			}
		}

		// Bulk transaction storage completed (metrics collected internally)

		return tx.Commit()
	}, 3) // Retry up to 3 times
}

// BatchStoreTransactionsInTx stores multiple transactions with their inputs and outputs within an existing transaction
func (db *DB) BatchStoreTransactionsInTx(tx *sql.Tx, transactions []*models.Transaction, inputs []*models.TransactionInput, outputs []*models.TransactionOutput) error {
	if len(transactions) == 0 {
		return nil
	}

	// Store transactions using bulk VALUES insert with batching
	if len(transactions) > 0 {
		// PostgreSQL has a limit of 65535 parameters, so we need to batch
		// Each transaction uses 10 parameters, so max batch size is 6553 transactions
		batchSize := 6000 // Conservative batch size

		for i := 0; i < len(transactions); i += batchSize {
			end := i + batchSize
			if end > len(transactions) {
				end = len(transactions)
			}

			batch := transactions[i:end]
			query := `
				INSERT INTO transaction (txid, block_height, block_hash, block_time, size, weight, fee_sats, is_coinbase, created_at, updated_at)
				VALUES `

			values := make([]string, len(batch))
			args := make([]interface{}, 0, len(batch)*10)

			for j, transaction := range batch {
				values[j] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
					j*10+1, j*10+2, j*10+3, j*10+4, j*10+5, j*10+6, j*10+7, j*10+8, j*10+9, j*10+10)

				args = append(args,
					transaction.Txid,
					transaction.BlockHeight,
					transaction.BlockHash,
					transaction.BlockTime,
					transaction.Size,
					transaction.Weight,
					transaction.FeeSats,
					transaction.IsCoinbase,
					transaction.CreatedAt,
					transaction.UpdatedAt,
				)
			}

			query += strings.Join(values, ", ")
			query += ` ON CONFLICT (txid) DO NOTHING`

			_, err := tx.Exec(query, args...)
			if err != nil {
				return fmt.Errorf("failed to bulk store transactions batch %d-%d: %w", i, end, err)
			}
		}
	}

	// Store inputs using bulk VALUES insert with batching
	if len(inputs) > 0 {
		batchSize := 6000 // Conservative batch size

		for i := 0; i < len(inputs); i += batchSize {
			end := i + batchSize
			if end > len(inputs) {
				end = len(inputs)
			}

			batch := inputs[i:end]
			query := `
				INSERT INTO transaction_input (txid, vout, prev_txid, prev_vout, script_sig, sequence, witness)
				VALUES `

			values := make([]string, len(batch))
			args := make([]interface{}, 0, len(batch)*7)

			for j, input := range batch {
				values[j] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
					j*7+1, j*7+2, j*7+3, j*7+4, j*7+5, j*7+6, j*7+7)

				// Handle witness field - convert nil to empty array for PostgreSQL
				witness := input.Witness
				if witness == nil {
					witness = []string{}
				}

				args = append(args,
					input.Txid,
					input.Vout,
					input.PrevTxid,
					input.PrevVout,
					input.ScriptSig,
					input.Sequence,
					pq.Array(witness),
				)
			}

			query += strings.Join(values, ", ")
			query += ` ON CONFLICT (txid, vout) DO NOTHING`

			_, err := tx.Exec(query, args...)
			if err != nil {
				return fmt.Errorf("failed to bulk store transaction inputs batch %d-%d: %w", i, end, err)
			}
		}
	}

	// Store outputs using bulk VALUES insert with batching
	if len(outputs) > 0 {
		batchSize := 6000 // Conservative batch size

		for i := 0; i < len(outputs); i += batchSize {
			end := i + batchSize
			if end > len(outputs) {
				end = len(outputs)
			}

			batch := outputs[i:end]
			query := `
				INSERT INTO transaction_output (txid, vout, address, value_sats, script_type, script_hex, script_asm)
				VALUES `

			values := make([]string, len(batch))
			args := make([]interface{}, 0, len(batch)*7)

			for j, output := range batch {
				values[j] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
					j*7+1, j*7+2, j*7+3, j*7+4, j*7+5, j*7+6, j*7+7)

				args = append(args,
					output.Txid,
					output.Vout,
					output.Address,
					output.ValueSats,
					output.ScriptType,
					output.ScriptHex,
					output.ScriptAsm,
				)
			}

			query += strings.Join(values, ", ")
			query += ` ON CONFLICT (txid, vout) DO NOTHING`

			_, err := tx.Exec(query, args...)
			if err != nil {
				return fmt.Errorf("failed to bulk store transaction outputs batch %d-%d: %w", i, end, err)
			}
		}
	}

	return nil
}

// BatchUpsertTransactionReferences processes multiple transaction references in a single transaction
func (db *DB) BatchUpsertTransactionReferences(txRefs []*models.TransactionReference) error {
	if len(txRefs) == 0 {
		return nil
	}

	return executeWithRetry(func() error {
		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback()

		// Set transaction isolation level to prevent deadlocks
		_, err = tx.Exec("SET TRANSACTION ISOLATION LEVEL READ COMMITTED")
		if err != nil {
			return fmt.Errorf("failed to set transaction isolation level: %w", err)
		}

		stmt := tx.Stmt(db.stmtCache["upsert_tx_reference"])
		defer stmt.Close()

		for _, txRef := range txRefs {
			_, err := stmt.Exec(
				txRef.Txid,
				txRef.Address,
				txRef.Direction,
				txRef.ValueSats,
				txRef.SenderAddress,
				txRef.ReceiverAddress,
				txRef.BlockHeight,
				txRef.BlockTimestamp,
				txRef.CreatedAt,
			)
			if err != nil {
				return fmt.Errorf("failed to upsert transaction reference %s:%s: %w", txRef.Txid, txRef.Address, err)
			}
		}

		return tx.Commit()
	}, 3) // Retry up to 3 times
}

// BatchUpsertTransactionReferencesInTx processes multiple transaction references within an existing transaction using bulk operations
func (db *DB) BatchUpsertTransactionReferencesInTx(tx *sql.Tx, txRefs []*models.TransactionReference) error {
	if len(txRefs) == 0 {
		return nil
	}

	// Use the existing bulk operation for maximum performance
	return db.bulkUpsertTxReferencesInTx(tx, txRefs)
}

// GetWatchedScriptsMap returns a map of watched scripts for faster lookup
func (db *DB) GetWatchedScriptsMap() (map[string]*models.WatchedScript, error) {
	scripts, err := db.GetWatchedScripts()
	if err != nil {
		return nil, err
	}

	scriptMap := make(map[string]*models.WatchedScript)
	for _, script := range scripts {
		scriptMap[script.ScriptHex] = script
	}

	return scriptMap, nil
}

// BatchUpsertUTXOsOptimized processes multiple UTXOs with larger batch sizes
func (db *DB) BatchUpsertUTXOsOptimized(utxos []*models.UTXO) error {
	if len(utxos) == 0 {
		return nil
	}

	return executeWithRetry(func() error {
		// Use larger batch sizes for better performance
		batchSize := 1000 // Increased from individual operations

		for i := 0; i < len(utxos); i += batchSize {
			end := i + batchSize
			if end > len(utxos) {
				end = len(utxos)
			}

			batch := utxos[i:end]
			if err := db.batchUpsertUTXOsBatch(batch); err != nil {
				return fmt.Errorf("failed to upsert UTXO batch %d-%d: %w", i, end, err)
			}
		}

		return nil
	}, 3)
}

// batchUpsertUTXOsBatch processes a single batch of UTXOs with deadlock prevention
func (db *DB) batchUpsertUTXOsBatch(utxos []*models.UTXO) error {
	// Sort UTXOs by (txid, vout) to ensure consistent lock ordering and prevent deadlocks
	sortedUTXOs := make([]*models.UTXO, len(utxos))
	copy(sortedUTXOs, utxos)
	sortUTXOsByTxidVout(sortedUTXOs)

	return executeWithRetry(func() error {
		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback()

		// Set transaction isolation level to prevent deadlocks
		_, err = tx.Exec("SET TRANSACTION ISOLATION LEVEL READ COMMITTED")
		if err != nil {
			return fmt.Errorf("failed to set transaction isolation level: %w", err)
		}

		// Use bulk VALUES insert for better performance
		query := `
		INSERT INTO utxo (txid, vout, address, script_hex, value_sats, status, block_height, block_hash, first_seen_at)
		VALUES `

		values := make([]string, len(sortedUTXOs))
		args := make([]interface{}, 0, len(sortedUTXOs)*9)

		for j, utxo := range sortedUTXOs {
			values[j] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				j*9+1, j*9+2, j*9+3, j*9+4, j*9+5, j*9+6, j*9+7, j*9+8, j*9+9)

			args = append(args,
				utxo.Txid,
				utxo.Vout,
				utxo.Address,
				utxo.ScriptHex,
				utxo.ValueSats,
				utxo.Status,
				utxo.BlockHeight,
				utxo.BlockHash,
				utxo.FirstSeenAt,
			)
		}

		query += strings.Join(values, ", ")
		query += ` ON CONFLICT (txid, vout) 
		DO UPDATE SET 
			status = EXCLUDED.status,
			block_height = EXCLUDED.block_height,
			block_hash = EXCLUDED.block_hash`

		_, err = tx.Exec(query, args...)
		if err != nil {
			return err
		}

		return tx.Commit()
	}, 5) // Retry up to 5 times for deadlock recovery
}

// ExecuteBulkOperations executes all database operations in a single transaction for maximum performance
func (db *DB) ExecuteBulkOperations(transactions []*models.Transaction, inputs []*models.TransactionInput, outputs []*models.TransactionOutput, newUTXOs []*models.UTXO, spentUTXOs []*models.UTXO, txReferences []*models.TransactionReference, blockHashes []string, blockHeightMap map[string]int) error {
	if len(transactions) == 0 && len(newUTXOs) == 0 && len(spentUTXOs) == 0 && len(txReferences) == 0 {
		return nil
	}

	startTime := time.Now()
	log.Printf("🗄️  Starting database bulk operations:")
	log.Printf("   💳 Transactions: %d", len(transactions))
	log.Printf("   📥 Inputs: %d", len(inputs))
	log.Printf("   📤 Outputs: %d", len(outputs))
	log.Printf("   🆕 New UTXOs: %d", len(newUTXOs))
	log.Printf("   💸 Spent UTXOs: %d", len(spentUTXOs))
	log.Printf("   🔗 Transaction References: %d", len(txReferences))
	log.Printf("   📦 Blocks: %d", len(blockHashes))

	return executeWithRetry(func() error {
		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback()

		// Set transaction isolation level to prevent deadlocks
		_, err = tx.Exec("SET TRANSACTION ISOLATION LEVEL READ COMMITTED")
		if err != nil {
			return fmt.Errorf("failed to set transaction isolation level: %w", err)
		}

		// Store transactions in bulk
		if len(transactions) > 0 {
			txStart := time.Now()
			if err := db.bulkStoreTransactionsInTx(tx, transactions); err != nil {
				return fmt.Errorf("failed to bulk store transactions: %w", err)
			}
			txDuration := time.Since(txStart)
			log.Printf("   ✅ Stored %d transactions in %v (%.2f txs/sec)",
				len(transactions), txDuration, float64(len(transactions))/txDuration.Seconds())
		}

		// Store inputs in bulk
		if len(inputs) > 0 {
			inputStart := time.Now()
			if err := db.bulkStoreInputsInTx(tx, inputs); err != nil {
				return fmt.Errorf("failed to bulk store inputs: %w", err)
			}
			inputDuration := time.Since(inputStart)
			log.Printf("   ✅ Stored %d inputs in %v (%.2f inputs/sec)",
				len(inputs), inputDuration, float64(len(inputs))/inputDuration.Seconds())
		}

		// Store outputs in bulk
		if len(outputs) > 0 {
			outputStart := time.Now()
			if err := db.bulkStoreOutputsInTx(tx, outputs); err != nil {
				return fmt.Errorf("failed to bulk store outputs: %w", err)
			}
			outputDuration := time.Since(outputStart)
			log.Printf("   ✅ Stored %d outputs in %v (%.2f outputs/sec)",
				len(outputs), outputDuration, float64(len(outputs))/outputDuration.Seconds())
		}

		// Process new UTXOs in bulk
		if len(newUTXOs) > 0 {
			utxoStart := time.Now()
			if err := db.bulkUpsertUTXOsInTx(tx, newUTXOs); err != nil {
				return fmt.Errorf("failed to bulk upsert UTXOs: %w", err)
			}
			utxoDuration := time.Since(utxoStart)
			log.Printf("   ✅ Upserted %d new UTXOs in %v (%.2f utxos/sec)",
				len(newUTXOs), utxoDuration, float64(len(newUTXOs))/utxoDuration.Seconds())
		}

		// Process spent UTXOs in bulk
		if len(spentUTXOs) > 0 {
			spentStart := time.Now()
			if err := db.bulkMarkUTXOsSpentInTx(tx, spentUTXOs); err != nil {
				return fmt.Errorf("failed to bulk mark UTXOs as spent: %w", err)
			}
			spentDuration := time.Since(spentStart)
			log.Printf("   ✅ Marked %d UTXOs as spent in %v (%.2f utxos/sec)",
				len(spentUTXOs), spentDuration, float64(len(spentUTXOs))/spentDuration.Seconds())
		}

		// Process transaction references in bulk
		if len(txReferences) > 0 {
			refStart := time.Now()
			if err := db.bulkUpsertTxReferencesInTx(tx, txReferences); err != nil {
				return fmt.Errorf("failed to bulk upsert transaction references: %w", err)
			}
			refDuration := time.Since(refStart)
			log.Printf("   ✅ Upserted %d transaction references in %v (%.2f refs/sec)",
				len(txReferences), refDuration, float64(len(txReferences))/refDuration.Seconds())
		}

		// Update index progress for all blocks
		if len(blockHashes) > 0 {
			progressStart := time.Now()
			if err := db.bulkUpdateIndexProgressInTx(tx, blockHashes, blockHeightMap); err != nil {
				return fmt.Errorf("failed to bulk update index progress: %w", err)
			}
			progressDuration := time.Since(progressStart)
			log.Printf("   ✅ Updated index progress for %d blocks in %v", len(blockHashes), progressDuration)
		}

		commitStart := time.Now()
		err = tx.Commit()
		commitDuration := time.Since(commitStart)
		totalDuration := time.Since(startTime)

		if err != nil {
			return fmt.Errorf("failed to commit transaction after %v: %w", totalDuration, err)
		}

		log.Printf("🗄️  Database bulk operations completed successfully!")
		log.Printf("   ⏱️  Total database time: %v", totalDuration)
		log.Printf("   💾 Commit time: %v", commitDuration)
		log.Printf("   📊 Total records processed: %d", len(transactions)+len(inputs)+len(outputs)+len(newUTXOs)+len(spentUTXOs)+len(txReferences))

		return nil
	}, 5) // Retry up to 5 times for deadlock recovery
}

// bulkStoreTransactionsInTx stores transactions in bulk within a transaction
func (db *DB) bulkStoreTransactionsInTx(tx *sql.Tx, transactions []*models.Transaction) error {
	if len(transactions) == 0 {
		return nil
	}

	// Use larger batch sizes for better performance
	batchSize := 5000 // Increased batch size
	totalBatches := (len(transactions) + batchSize - 1) / batchSize

	log.Printf("   💳 Storing %d transactions in %d batches (batch size: %d)", len(transactions), totalBatches, batchSize)

	for i := 0; i < len(transactions); i += batchSize {
		end := i + batchSize
		if end > len(transactions) {
			end = len(transactions)
		}

		batch := transactions[i:end]
		batchNum := (i / batchSize) + 1

		query := `
			INSERT INTO transaction (txid, block_height, block_hash, block_time, size, weight, fee_sats, is_coinbase, created_at, updated_at)
			VALUES `

		values := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)*10)

		for j, transaction := range batch {
			values[j] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				j*10+1, j*10+2, j*10+3, j*10+4, j*10+5, j*10+6, j*10+7, j*10+8, j*10+9, j*10+10)

			args = append(args,
				transaction.Txid,
				transaction.BlockHeight,
				transaction.BlockHash,
				transaction.BlockTime,
				transaction.Size,
				transaction.Weight,
				transaction.FeeSats,
				transaction.IsCoinbase,
				transaction.CreatedAt,
				time.Now(),
			)
		}

		query += strings.Join(values, ", ")
		query += ` ON CONFLICT (txid) DO UPDATE SET
			block_height = EXCLUDED.block_height,
			block_hash = EXCLUDED.block_hash,
			block_time = EXCLUDED.block_time,
			size = EXCLUDED.size,
			weight = EXCLUDED.weight,
			fee_sats = EXCLUDED.fee_sats,
			is_coinbase = EXCLUDED.is_coinbase,
			updated_at = EXCLUDED.updated_at`

		_, err := tx.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("failed to bulk store transactions batch %d-%d: %w", i, end, err)
		}

		if batchNum%5 == 0 || batchNum == totalBatches {
			log.Printf("   💳 Transaction batch %d/%d completed (%d transactions)", batchNum, totalBatches, len(batch))
		}
	}

	return nil
}

// bulkStoreInputsInTx stores transaction inputs in bulk within a transaction
func (db *DB) bulkStoreInputsInTx(tx *sql.Tx, inputs []*models.TransactionInput) error {
	if len(inputs) == 0 {
		return nil
	}

	batchSize := 8000 // Large batch size for inputs

	for i := 0; i < len(inputs); i += batchSize {
		end := i + batchSize
		if end > len(inputs) {
			end = len(inputs)
		}

		batch := inputs[i:end]
		query := `
			INSERT INTO transaction_input (txid, vout, prev_txid, prev_vout, script_sig, sequence, witness)
			VALUES `

		values := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)*7)

		for j, input := range batch {
			values[j] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				j*7+1, j*7+2, j*7+3, j*7+4, j*7+5, j*7+6, j*7+7)

			// Handle witness field - convert nil to empty array for PostgreSQL
			witness := input.Witness
			if witness == nil {
				witness = []string{}
			}

			args = append(args,
				input.Txid,
				input.Vout,
				input.PrevTxid,
				input.PrevVout,
				input.ScriptSig,
				input.Sequence,
				pq.Array(witness),
			)
		}

		query += strings.Join(values, ", ")
		query += ` ON CONFLICT (txid, vout) DO UPDATE SET
			prev_txid = EXCLUDED.prev_txid,
			prev_vout = EXCLUDED.prev_vout,
			script_sig = EXCLUDED.script_sig,
			sequence = EXCLUDED.sequence,
			witness = EXCLUDED.witness`

		_, err := tx.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("failed to bulk store transaction inputs batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// bulkStoreOutputsInTx stores transaction outputs in bulk within a transaction
func (db *DB) bulkStoreOutputsInTx(tx *sql.Tx, outputs []*models.TransactionOutput) error {
	if len(outputs) == 0 {
		return nil
	}

	batchSize := 8000 // Large batch size for outputs

	for i := 0; i < len(outputs); i += batchSize {
		end := i + batchSize
		if end > len(outputs) {
			end = len(outputs)
		}

		batch := outputs[i:end]
		query := `
			INSERT INTO transaction_output (txid, vout, address, value_sats, script_type, script_hex, script_asm)
			VALUES `

		values := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)*7)

		for j, output := range batch {
			values[j] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				j*7+1, j*7+2, j*7+3, j*7+4, j*7+5, j*7+6, j*7+7)

			args = append(args,
				output.Txid,
				output.Vout,
				output.Address,
				output.ValueSats,
				output.ScriptType,
				output.ScriptHex,
				output.ScriptAsm,
			)
		}

		query += strings.Join(values, ", ")
		query += ` ON CONFLICT (txid, vout) DO UPDATE SET
			address = EXCLUDED.address,
			value_sats = EXCLUDED.value_sats,
			script_type = EXCLUDED.script_type,
			script_hex = EXCLUDED.script_hex,
			script_asm = EXCLUDED.script_asm`

		_, err := tx.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("failed to bulk store transaction outputs batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// bulkUpsertUTXOsInTx processes multiple UTXOs in bulk within a transaction
func (db *DB) bulkUpsertUTXOsInTx(tx *sql.Tx, utxos []*models.UTXO) error {
	if len(utxos) == 0 {
		return nil
	}

	// Sort UTXOs by (txid, vout) to ensure consistent lock ordering and prevent deadlocks
	sortedUTXOs := make([]*models.UTXO, len(utxos))
	copy(sortedUTXOs, utxos)
	sortUTXOsByTxidVout(sortedUTXOs)

	batchSize := 5000 // Large batch size for UTXOs
	totalBatches := (len(sortedUTXOs) + batchSize - 1) / batchSize

	log.Printf("   🆕 Upserting %d new UTXOs in %d batches (batch size: %d)", len(sortedUTXOs), totalBatches, batchSize)

	for i := 0; i < len(sortedUTXOs); i += batchSize {
		end := i + batchSize
		if end > len(sortedUTXOs) {
			end = len(sortedUTXOs)
		}

		batch := sortedUTXOs[i:end]
		batchNum := (i / batchSize) + 1

		query := `
		INSERT INTO utxo (txid, vout, address, script_hex, value_sats, status, block_height, block_hash, first_seen_at)
		VALUES `

		values := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)*9)

		for j, utxo := range batch {
			values[j] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				j*9+1, j*9+2, j*9+3, j*9+4, j*9+5, j*9+6, j*9+7, j*9+8, j*9+9)

			args = append(args,
				utxo.Txid,
				utxo.Vout,
				utxo.Address,
				utxo.ScriptHex,
				utxo.ValueSats,
				utxo.Status,
				utxo.BlockHeight,
				utxo.BlockHash,
				utxo.FirstSeenAt,
			)
		}

		query += strings.Join(values, ", ")
		query += ` ON CONFLICT (txid, vout) 
		DO UPDATE SET 
			status = EXCLUDED.status,
			block_height = EXCLUDED.block_height,
			block_hash = EXCLUDED.block_hash`

		_, err := tx.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("failed to bulk upsert UTXOs batch %d-%d: %w", i, end, err)
		}

		if batchNum%5 == 0 || batchNum == totalBatches {
			log.Printf("   🆕 UTXO batch %d/%d completed (%d UTXOs)", batchNum, totalBatches, len(batch))
		}
	}

	return nil
}

// bulkMarkUTXOsSpentInTx marks multiple UTXOs as spent in bulk within a transaction
func (db *DB) bulkMarkUTXOsSpentInTx(tx *sql.Tx, spentUTXOs []*models.UTXO) error {
	if len(spentUTXOs) == 0 {
		return nil
	}

	// Sort UTXOs by (txid, vout) to ensure consistent lock ordering and prevent deadlocks
	sortedUTXOs := make([]*models.UTXO, len(spentUTXOs))
	copy(sortedUTXOs, spentUTXOs)
	sortUTXOsByTxidVout(sortedUTXOs)

	// Use the most efficient approach: single UPDATE with IN clause
	// This leverages the primary key index (txid, vout) for maximum performance
	batchSize := 1000 // Process in batches to avoid parameter limits

	for i := 0; i < len(sortedUTXOs); i += batchSize {
		end := i + batchSize
		if end > len(sortedUTXOs) {
			end = len(sortedUTXOs)
		}

		batch := sortedUTXOs[i:end]

		// Build the IN clause for the batch
		placeholders := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)*2+2)

		for j, utxo := range batch {
			placeholders[j] = fmt.Sprintf("($%d, $%d)", j*2+1, j*2+2)
			args = append(args, utxo.Txid, utxo.Vout)
		}

		// Add the spent_at and spent_by_txid parameters (use the first UTXO's values as they should be the same)
		args = append(args, batch[0].SpentAt, batch[0].SpentByTxid)

		query := fmt.Sprintf(`
			UPDATE utxo 
			SET status = 'spent', spent_at = $%d, spent_by_txid = $%d
			WHERE (txid, vout) IN (%s)`,
			len(args)-1, len(args), strings.Join(placeholders, ", "))

		_, err := tx.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("failed to bulk mark UTXOs as spent batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// BatchUpsertUTXOsInTx processes multiple UTXOs within an existing transaction using bulk operations
func (db *DB) BatchUpsertUTXOsInTx(tx *sql.Tx, utxos []*models.UTXO) error {
	if len(utxos) == 0 {
		return nil
	}

	// Use the existing bulk operation for maximum performance
	return db.bulkUpsertUTXOsInTx(tx, utxos)
}

// BatchMarkUTXOsSpentInTx marks multiple UTXOs as spent within an existing transaction using bulk operations
func (db *DB) BatchMarkUTXOsSpentInTx(tx *sql.Tx, spentUTXOs []*models.UTXO) error {
	if len(spentUTXOs) == 0 {
		return nil
	}

	// Use the existing bulk operation for maximum performance
	return db.bulkMarkUTXOsSpentInTx(tx, spentUTXOs)
}

// bulkUpsertTxReferencesInTx processes multiple transaction references in bulk within a transaction
func (db *DB) bulkUpsertTxReferencesInTx(tx *sql.Tx, txRefs []*models.TransactionReference) error {
	if len(txRefs) == 0 {
		return nil
	}

	batchSize := 5000 // Large batch size for transaction references

	for i := 0; i < len(txRefs); i += batchSize {
		end := i + batchSize
		if end > len(txRefs) {
			end = len(txRefs)
		}

		batch := txRefs[i:end]
		query := `
			INSERT INTO tx_reference (txid, address, direction, value_sats, sender_address, receiver_address, block_height, block_timestamp, created_at)
			VALUES `

		values := make([]string, len(batch))
		args := make([]interface{}, 0, len(batch)*9)

		for j, txRef := range batch {
			values[j] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				j*9+1, j*9+2, j*9+3, j*9+4, j*9+5, j*9+6, j*9+7, j*9+8, j*9+9)

			args = append(args,
				txRef.Txid,
				txRef.Address,
				txRef.Direction,
				txRef.ValueSats,
				txRef.SenderAddress,
				txRef.ReceiverAddress,
				txRef.BlockHeight,
				txRef.BlockTimestamp,
				txRef.CreatedAt,
			)
		}

		query += strings.Join(values, ", ")
		query += ` ON CONFLICT (txid, address, direction) 
		DO UPDATE SET 
			value_sats = EXCLUDED.value_sats,
			sender_address = EXCLUDED.sender_address,
			receiver_address = EXCLUDED.receiver_address,
			block_height = EXCLUDED.block_height,
			block_timestamp = EXCLUDED.block_timestamp`

		_, err := tx.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("failed to bulk upsert transaction references batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// bulkUpdateIndexProgressInTx updates index progress for multiple blocks in bulk within a transaction
func (db *DB) bulkUpdateIndexProgressInTx(tx *sql.Tx, blockHashes []string, blockHeightMap map[string]int) error {
	if len(blockHashes) == 0 {
		return nil
	}

	// Find the highest block height to update progress to
	var maxHeight int
	var latestBlockHash string

	for _, blockHash := range blockHashes {
		height := blockHeightMap[blockHash]
		if height > maxHeight {
			maxHeight = height
			latestBlockHash = blockHash
		}
	}

	// Update index progress to the latest block
	query := `
		INSERT INTO index_progress (id, last_height, last_block_hash, updated_at)
		VALUES (true, $1, $2, $3)
		ON CONFLICT (id) 
		DO UPDATE SET 
			last_height = EXCLUDED.last_height,
			last_block_hash = EXCLUDED.last_block_hash,
			updated_at = EXCLUDED.updated_at
	`

	_, err := tx.Exec(query, maxHeight, latestBlockHash, time.Now())
	if err != nil {
		return fmt.Errorf("failed to bulk update index progress: %w", err)
	}

	return nil
}
