package models

import (
	"time"
)

// WatchTarget represents addresses or HD wallets to monitor
type WatchTarget struct {
	ID               int64     `json:"id" db:"id"`
	Kind             string    `json:"kind" db:"kind"` // "address" or "xpub"
	Address          *string   `json:"address,omitempty" db:"address"`
	Xpub             *string   `json:"xpub,omitempty" db:"xpub"`
	DerivationScheme *string   `json:"derivation_scheme,omitempty" db:"derivation_scheme"`
	Account          *int      `json:"account,omitempty" db:"account"`
	GapLimit         int       `json:"gap_limit" db:"gap_limit"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

// WatchedScript represents derived scripts we monitor
type WatchedScript struct {
	ID        int64  `json:"id" db:"id"`
	Address   string `json:"address" db:"address"`
	ScriptHex string `json:"script_hex" db:"script_hex"`
	Type      string `json:"type" db:"type"` // p2wpkh, p2tr, etc.
	TargetID  int64  `json:"target_id" db:"target_id"`
}

// UTXO represents unspent transaction outputs
type UTXO struct {
	Txid        string     `json:"txid" db:"txid"`
	Vout        int        `json:"vout" db:"vout"`
	Address     string     `json:"address" db:"address"`
	ScriptHex   string     `json:"script_hex" db:"script_hex"`
	ValueSats   int64      `json:"value_sats" db:"value_sats"`
	Status      string     `json:"status" db:"status"` // mempool, confirmed, spent
	BlockHeight *int       `json:"block_height,omitempty" db:"block_height"`
	BlockHash   *string    `json:"block_hash,omitempty" db:"block_hash"`
	FirstSeenAt time.Time  `json:"first_seen_at" db:"first_seen_at"`
	SpentAt     *time.Time `json:"spent_at,omitempty" db:"spent_at"`
	SpentByTxid *string    `json:"spent_by_txid,omitempty" db:"spent_by_txid"`
}

// IndexProgress tracks indexing state
type IndexProgress struct {
	ID            bool      `json:"id" db:"id"`
	LastHeight    int       `json:"last_height" db:"last_height"`
	LastBlockHash string    `json:"last_block_hash" db:"last_block_hash"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// Balance represents address balance information
type Balance struct {
	Address           string `json:"address"`
	Pending           int64  `json:"pending"`
	Confirmed         int64  `json:"confirmed"`
	Total             int64  `json:"total"`
	LastIndexedHeight int    `json:"last_indexed_height"`
}

// UTXOInfo represents UTXO information for API responses
type UTXOInfo struct {
	Txid          string  `json:"txid"`
	Vout          int     `json:"vout"`
	ValueSats     int64   `json:"value_sats"`
	Status        string  `json:"status"`
	Confirmations int     `json:"confirmations"`
	BlockHeight   *int    `json:"block_height,omitempty"`
	BlockHash     *string `json:"block_hash,omitempty"`
	FirstSeenAt   string  `json:"first_seen_at"`
	SpentAt       *string `json:"spent_at,omitempty"`
}

// Transaction represents a complete Bitcoin transaction
type Transaction struct {
	Txid        string     `json:"txid" db:"txid"`
	BlockHeight *int       `json:"block_height,omitempty" db:"block_height"`
	BlockHash   *string    `json:"block_hash,omitempty" db:"block_hash"`
	BlockTime   *time.Time `json:"block_time,omitempty" db:"block_time"`
	Size        *int       `json:"size,omitempty" db:"size"`
	Weight      *int       `json:"weight,omitempty" db:"weight"`
	FeeSats     *int64     `json:"fee_sats,omitempty" db:"fee_sats"`
	IsCoinbase  bool       `json:"is_coinbase" db:"is_coinbase"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty" db:"updated_at"`
}

// TransactionInput represents a transaction input (spending)
type TransactionInput struct {
	Txid      string   `json:"txid" db:"txid"`
	Vout      int      `json:"vout" db:"vout"`
	PrevTxid  string   `json:"prev_txid" db:"prev_txid"`
	PrevVout  int      `json:"prev_vout" db:"prev_vout"`
	ScriptSig *string  `json:"script_sig,omitempty" db:"script_sig"`
	Sequence  *int64   `json:"sequence,omitempty" db:"sequence"`
	Witness   []string `json:"witness,omitempty" db:"witness"`
}

// TransactionOutput represents a transaction output (creation)
type TransactionOutput struct {
	Txid       string  `json:"txid" db:"txid"`
	Vout       int     `json:"vout" db:"vout"`
	Address    string  `json:"address" db:"address"`
	ValueSats  int64   `json:"value_sats" db:"value_sats"`
	ScriptType *string `json:"script_type,omitempty" db:"script_type"`
	ScriptHex  *string `json:"script_hex,omitempty" db:"script_hex"`
	ScriptAsm  *string `json:"script_asm,omitempty" db:"script_asm"`
}

// BlockInfo represents block information
type BlockInfo struct {
	Height           int       `json:"height" db:"height"`
	Hash             string    `json:"hash" db:"hash"`
	PreviousHash     *string   `json:"previous_hash,omitempty" db:"previous_hash"`
	Timestamp        time.Time `json:"timestamp" db:"timestamp"`
	Size             *int      `json:"size,omitempty" db:"size"`
	Weight           *int      `json:"weight,omitempty" db:"weight"`
	TransactionCount *int      `json:"transaction_count,omitempty" db:"transaction_count"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

// AddressMetadata represents enhanced address information
type AddressMetadata struct {
	Address           string     `json:"address" db:"address"`
	FirstSeenAt       *time.Time `json:"first_seen_at,omitempty" db:"first_seen_at"`
	LastSeenAt        *time.Time `json:"last_seen_at,omitempty" db:"last_seen_at"`
	TotalReceivedSats int64      `json:"total_received_sats" db:"total_received_sats"`
	TotalSentSats     int64      `json:"total_sent_sats" db:"total_sent_sats"`
	TransactionCount  int        `json:"transaction_count" db:"transaction_count"`
	IsWatched         bool       `json:"is_watched" db:"is_watched"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         *time.Time `json:"updated_at,omitempty" db:"updated_at"`
}

// TransactionReference represents transaction flow for watched addresses
type TransactionReference struct {
	ID              int64     `json:"id" db:"id"`
	Txid            string    `json:"txid" db:"txid"`
	Address         string    `json:"address" db:"address"`
	Direction       string    `json:"direction" db:"direction"` // "in" or "out"
	ValueSats       int64     `json:"value_sats" db:"value_sats"`
	SenderAddress   *string   `json:"sender_address,omitempty" db:"sender_address"`
	ReceiverAddress *string   `json:"receiver_address,omitempty" db:"receiver_address"`
	BlockHeight     *int      `json:"block_height,omitempty" db:"block_height"`
	BlockTimestamp  *int      `json:"block_timestamp,omitempty" db:"block_timestamp"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}
