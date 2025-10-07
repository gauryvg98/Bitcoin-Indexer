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

-- Complete transaction storage
CREATE TABLE IF NOT EXISTS transaction (
  txid TEXT PRIMARY KEY,
  block_height INT,
  block_hash TEXT,
  block_time TIMESTAMPTZ,
  size INT,
  weight INT,
  fee_sats BIGINT,
  is_coinbase BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

-- Transaction inputs (spending records)
CREATE TABLE IF NOT EXISTS transaction_input (
  txid TEXT NOT NULL,
  vout INT NOT NULL,
  prev_txid TEXT NOT NULL,
  prev_vout INT NOT NULL,
  script_sig TEXT,
  sequence BIGINT,
  witness TEXT[], -- Array of witness data
  PRIMARY KEY (txid, vout),
  FOREIGN KEY (txid) REFERENCES transaction(txid) ON DELETE CASCADE
);

-- Transaction outputs (creation records)
CREATE TABLE IF NOT EXISTS transaction_output (
  txid TEXT NOT NULL,
  vout INT NOT NULL,
  address TEXT NOT NULL,
  value_sats BIGINT NOT NULL,
  script_type TEXT,
  script_hex TEXT,
  script_asm TEXT,
  PRIMARY KEY (txid, vout),
  FOREIGN KEY (txid) REFERENCES transaction(txid) ON DELETE CASCADE
);

-- UTXO mirror for ALL addresses (not just watched ones)
CREATE TABLE IF NOT EXISTS utxo (
  txid TEXT NOT NULL,
  vout INT NOT NULL,
  address TEXT NOT NULL,        -- ALL addresses, not just watched
  script_hex TEXT NOT NULL,
  value_sats BIGINT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('mempool','confirmed','spent')),
  block_height INT,
  block_hash TEXT,
  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  spent_at TIMESTAMPTZ,
  spent_by_txid TEXT, -- Which transaction spent this UTXO
  PRIMARY KEY (txid, vout),
  FOREIGN KEY (txid) REFERENCES transaction(txid) ON DELETE CASCADE
);

-- Block information for better context
CREATE TABLE IF NOT EXISTS block_info (
  height INT PRIMARY KEY,
  hash TEXT NOT NULL UNIQUE,
  previous_hash TEXT,
  timestamp TIMESTAMPTZ NOT NULL,
  size INT,
  weight INT,
  transaction_count INT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Address metadata for enhanced tracking
CREATE TABLE IF NOT EXISTS address_metadata (
  address TEXT PRIMARY KEY,
  first_seen_at TIMESTAMPTZ,
  last_seen_at TIMESTAMPTZ,
  total_received_sats BIGINT DEFAULT 0,
  total_sent_sats BIGINT DEFAULT 0,
  transaction_count INT DEFAULT 0,
  is_watched BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

-- Transaction flow tracking for watched addresses only
CREATE TABLE IF NOT EXISTS tx_reference (
  id BIGSERIAL PRIMARY KEY,
  txid TEXT NOT NULL,
  address TEXT NOT NULL,
  direction TEXT NOT NULL CHECK (direction IN ('in','out')),
  value_sats BIGINT NOT NULL,
  sender_address TEXT NULL, -- Computed sender for incoming transactions
  receiver_address TEXT NULL, -- Computed receiver for outgoing transactions
  block_height INT,
  block_timestamp INT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (txid, address, direction)
);

-- Indexing progress and tip continuity
CREATE TABLE IF NOT EXISTS index_progress (
  id BOOLEAN PRIMARY KEY DEFAULT TRUE,
  last_height INT,
  last_block_hash TEXT,
  updated_at TIMESTAMPTZ DEFAULT now()
);


-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_utxo_address ON utxo (address);
CREATE INDEX IF NOT EXISTS idx_utxo_status ON utxo (status);
CREATE INDEX IF NOT EXISTS idx_utxo_blockhash ON utxo (block_hash);
CREATE INDEX IF NOT EXISTS idx_utxo_spent_by ON utxo (spent_by_txid);
CREATE INDEX IF NOT EXISTS idx_watched_script_address ON watched_script (address);
CREATE INDEX IF NOT EXISTS idx_watched_script_hex ON watched_script (script_hex);

-- Transaction indexes
CREATE INDEX IF NOT EXISTS idx_transaction_block_height ON transaction (block_height);
CREATE INDEX IF NOT EXISTS idx_transaction_block_time ON transaction (block_time);
CREATE INDEX IF NOT EXISTS idx_transaction_coinbase ON transaction (is_coinbase);

-- Transaction input indexes
CREATE INDEX IF NOT EXISTS idx_tx_input_prev ON transaction_input (prev_txid, prev_vout);
CREATE INDEX IF NOT EXISTS idx_tx_input_txid ON transaction_input (txid);

-- Transaction output indexes
CREATE INDEX IF NOT EXISTS idx_tx_output_address ON transaction_output (address);
CREATE INDEX IF NOT EXISTS idx_tx_output_txid ON transaction_output (txid);
CREATE INDEX IF NOT EXISTS idx_tx_output_value ON transaction_output (value_sats);

-- Composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_utxo_address_status ON utxo (address, status);
CREATE INDEX IF NOT EXISTS idx_tx_output_address_value ON transaction_output (address, value_sats);
CREATE INDEX IF NOT EXISTS idx_tx_input_prev_composite ON transaction_input (prev_txid, prev_vout, txid);

-- Block and address metadata indexes
CREATE INDEX IF NOT EXISTS idx_block_info_hash ON block_info (hash);
CREATE INDEX IF NOT EXISTS idx_block_info_timestamp ON block_info (timestamp);
CREATE INDEX IF NOT EXISTS idx_address_metadata_watched ON address_metadata (is_watched);
CREATE INDEX IF NOT EXISTS idx_address_metadata_last_seen ON address_metadata (last_seen_at);

-- Transaction reference indexes
CREATE INDEX IF NOT EXISTS idx_tx_ref_address ON tx_reference (address);
CREATE INDEX IF NOT EXISTS idx_tx_ref_height ON tx_reference (block_height);
CREATE INDEX IF NOT EXISTS idx_tx_ref_txid ON tx_reference (txid);

