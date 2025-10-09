# Bitcoin Indexer

A high-performance Bitcoin blockchain indexer written in Go that tracks wallet balances, transactions, and UTXO states in real-time. Built for scalability with parallel block processing and comprehensive transaction analysis.

[![License](https://img.shields.io/badge/License-Unlicense-blue.svg)](https://unlicense.org/)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-13+-316192?logo=postgresql)](https://www.postgresql.org/)

## 🚀 Features

### Core Functionality
- **Real-time UTXO Tracking**: Monitor all unspent transaction outputs across the blockchain
- **Comprehensive Transaction Parsing**: Store complete transaction data including inputs, outputs, and metadata
- **Address Monitoring**: Watch specific Bitcoin addresses or HD wallet (xpub) derivations
- **Parallel Block Processing**: High-performance indexing with configurable worker pools
- **Smart Address Extraction**: Automatically handles multiple Bitcoin Core API versions (singular `address` and plural `addresses` fields)
- **Spent UTXO Tracking**: Track which transaction spent each UTXO with `spent_by_txid` field

### Advanced Features
- **HD Wallet Support**: BIP32/BIP44/BIP49/BIP84 derivation for extended public keys
- **Mempool Monitoring**: Real-time tracking of unconfirmed transactions
- **Reorg Handling**: Automatic chain reorganization detection and recovery
- **Webhook Support**: HTTP callbacks for transaction events
- **RESTful API**: Query balances, transactions, and UTXOs via HTTP endpoints

### Performance Optimizations
- **Batch Database Operations**: Minimize database round-trips with bulk inserts/updates
- **Connection Pooling**: Optimized PostgreSQL connection management
- **Prepared Statements**: Pre-compiled SQL for faster queries
- **Concurrent Processing**: Parallel block processing with worker pools
- **Script Caching**: In-memory cache for watched scripts

## 📋 Table of Contents

- [Architecture](#architecture)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [API Reference](#api-reference)
- [Database Schema](#database-schema)
- [Development](#development)
- [Performance](#performance)
- [Contributing](#contributing)
- [License](#license)

## 🏗 Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Bitcoin Core RPC                        │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    Bitcoin Indexer                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   RPC Client │  │    Parser    │  │   Database   │      │
│  │              │─▶│              │─▶│              │      │
│  │ - GetBlock   │  │ - ParseBlock │  │ - PostgreSQL │      │
│  │ - GetTx      │  │ - ParseTx    │  │ - UTXO Store │      │
│  │ - Mempool    │  │ - ExtractAddr│  │ - Tx Store   │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Indexer    │  │  HD Wallet   │  │  API Server  │      │
│  │              │  │              │  │              │      │
│  │ - Sequential │  │ - BIP32/44   │  │ - REST API   │      │
│  │ - Parallel   │  │ - BIP49/84   │  │ - Webhooks   │      │
│  │ - Mempool    │  │ - Derivation │  │ - Handlers   │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                     PostgreSQL Database                      │
│  • UTXOs  • Transactions  • Addresses  • Watch Targets      │
└─────────────────────────────────────────────────────────────┘
```

### Components

- **RPC Client**: Communicates with Bitcoin Core node with retry logic and connection pooling
- **Parser**: Extracts addresses, parses transactions, and manages UTXO state changes
- **Indexer**: Coordinates block processing (sequential, parallel, or mempool modes)
- **Database**: PostgreSQL storage with optimized schema and indexes
- **API Server**: HTTP REST API for querying indexed data
- **HD Wallet**: Derives addresses from extended public keys

## 📦 Installation

### Prerequisites

- **Go 1.21+**: [Download Go](https://go.dev/dl/)
- **PostgreSQL 13+**: [Download PostgreSQL](https://www.postgresql.org/download/)
- **Bitcoin Core**: Running node with RPC enabled and `txindex=1`

### Quick Start

1. **Clone the repository**
```bash
git clone https://github.com/gauryvg98/Bitcoin-Indexer.git
cd Bitcoin-Indexer
```

2. **Install dependencies**
```bash
go mod download
```

3. **Set up PostgreSQL**
```bash
# Create database
createdb bitcoin_indexer

# Choose setup method:
# Option A: Fresh installation (recommended for new setups)
./setup_comprehensive.sh

# Option B: Check current schema status
./check_schema_diff.sh

# Option C: Migrate existing database (preserves data)
./migrate_to_bulk_operations.sh
```

4. **Configure environment**
```bash
cp config.env.example config.env
# Edit config.env with your settings
```

5. **Build the indexer**
```bash
make build
# or
go build -o bitcoin-indexer ./cmd/indexer
```

6. **Run the indexer**
```bash
./bitcoin-indexer
```

## ⚙️ Configuration

Create a `config.env` file in the project root:

```env
# Bitcoin Core RPC Configuration
BITCOIN_RPC_HOST=localhost:8332
BITCOIN_RPC_USER=your_rpc_username
BITCOIN_RPC_PASSWORD=your_rpc_password

# Database Configuration
DATABASE_URL=postgres://user:password@localhost:5432/bitcoin_indexer?sslmode=disable

# API Server Configuration
API_PORT=8080

# Indexer Settings
START_HEIGHT=0                    # Block height to start indexing from
END_HEIGHT=0                      # End height (0 = latest)
INDEXER_MODE=sequential           # sequential, parallel, or mempool
PARALLEL_WORKERS=10               # Number of parallel workers (parallel mode only)

# Performance Tuning
BATCH_SIZE=100                    # Transactions per batch
CONNECTION_POOL_SIZE=50           # Database connection pool size
```

### Bitcoin Core Configuration

Ensure your `bitcoin.conf` includes:

```conf
# RPC Settings
server=1
rpcuser=your_rpc_username
rpcpassword=your_rpc_password
rpcport=8332

# Required for full indexing
txindex=1

# Optional: Increase RPC timeout for large blocks
rpcworkqueue=256
rpcthreads=16
```

## 🔧 Usage

### Basic Commands

```bash
# Build the project
make build

# Run the indexer
./bitcoin-indexer

# Run with custom config
./bitcoin-indexer -config=/path/to/config.env

# Run tests
make test

# Clean build artifacts
make clean
```

### Indexing Modes

#### 1. Sequential Mode (Default)
Processes blocks one at a time in order. Safe and reliable.
```bash
INDEXER_MODE=sequential ./bitcoin-indexer
```

#### 2. Parallel Mode
Processes multiple blocks concurrently for faster indexing.
```bash
INDEXER_MODE=parallel PARALLEL_WORKERS=10 ./bitcoin-indexer
```

#### 3. Mempool Mode
Monitors unconfirmed transactions in real-time.
```bash
INDEXER_MODE=mempool ./bitcoin-indexer
```

### Watch an Address

```bash
# Using curl
curl -X POST http://localhost:8080/api/v1/watch \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "address",
    "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
  }'

# Or use the example script
./examples/add_address.sh 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa
```

### Watch an xPub

```bash
curl -X POST http://localhost:8080/api/v1/watch \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "xpub",
    "xpub": "xpub6CUGRUonZSQ4TWtTMmzXdrXDtypWKiKrhko4egpiMZbpiaQL2jkwSB1icqYh2cfDfVxdx4df189oLKnC5fSwqPfgyP3hooxujYzAu3fDVmz",
    "derivation_scheme": "bip84",
    "gap_limit": 20
  }'
```

## 📡 API Reference

### Endpoints

#### **GET** `/api/v1/balance/:address`
Get balance for an address.

**Response:**
```json
{
  "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
  "pending": 0,
  "confirmed": 5000000000,
  "total": 5000000000,
  "last_indexed_height": 800000
}
```

#### **GET** `/api/v1/utxos/:address`
Get all UTXOs for an address.

**Response:**
```json
{
  "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
  "utxos": [
    {
      "txid": "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b",
      "vout": 0,
      "value_sats": 5000000000,
      "status": "confirmed",
      "confirmations": 100,
      "block_height": 799900,
      "spent_by_txid": null
    }
  ]
}
```

#### **GET** `/api/v1/transactions/:address`
Get transaction history for an address.

**Query Parameters:**
- `limit`: Number of transactions to return (default: 50, max: 100)

**Response:**
```json
{
  "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
  "transactions": [
    {
      "txid": "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b",
      "direction": "in",
      "value_sats": 5000000000,
      "block_height": 0,
      "block_timestamp": 1231006505,
      "sender_address": null,
      "receiver_address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
    }
  ]
}
```

#### **POST** `/api/v1/watch`
Add an address or xpub to watch list.

**Request Body:**
```json
{
  "kind": "address",
  "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
}
```

#### **GET** `/api/v1/transaction/:txid`
Get detailed transaction information.

**Response:**
```json
{
  "txid": "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b",
  "block_height": 0,
  "block_hash": "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f",
  "block_time": "2009-01-03T18:15:05Z",
  "size": 204,
  "weight": 816,
  "fee_sats": 0,
  "is_coinbase": true,
  "inputs": [],
  "outputs": [
    {
      "vout": 0,
      "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
      "value_sats": 5000000000,
      "script_type": "p2pkh"
    }
  ]
}
```

#### **GET** `/api/v1/health`
Health check endpoint.

**Response:**
```json
{
  "status": "ok",
  "last_indexed_height": 800000,
  "last_indexed_hash": "00000000000000000001a7b9c8e9..."
}
```

## 🗄 Database Schema

### Core Tables

#### `utxo`
Tracks all unspent transaction outputs.
```sql
CREATE TABLE utxo (
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
  spent_by_txid TEXT,  -- Tracks which transaction spent this UTXO
  PRIMARY KEY (txid, vout)
);
```

#### `transaction`
Complete transaction metadata.
```sql
CREATE TABLE transaction (
  txid TEXT PRIMARY KEY,
  block_height INT,
  block_hash TEXT,
  block_time TIMESTAMPTZ,
  size INT,
  weight INT,
  fee_sats BIGINT,
  is_coinbase BOOLEAN DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

#### `transaction_input` & `transaction_output`
Detailed input/output data for full transaction reconstruction.

#### `watch_target` & `watched_script`
Addresses and xpubs being monitored, with derived scripts.

#### `tx_reference`
Transaction flow tracking for watched addresses (incoming/outgoing).

### Key Features

- **Indexed for Performance**: All critical columns have indexes
- **Foreign Key Constraints**: Data integrity enforcement
- **JSONB Support**: Flexible data storage for webhooks
- **Composite Indexes**: Optimized for common query patterns

## 🚀 Performance

### Benchmarks

Tested on AWS EC2 t3.large (2 vCPU, 8GB RAM):

| Mode       | Blocks/sec | Time to index 100k blocks |
|------------|------------|---------------------------|
| Sequential | 15-20      | ~83 minutes               |
| Parallel   | 80-100     | ~16 minutes               |

### Optimization Tips

1. **Use Parallel Mode** for initial indexing of historical blocks
2. **Increase `PARALLEL_WORKERS`** based on CPU cores (recommended: 2x cores)
3. **Tune PostgreSQL** settings:
   ```conf
   shared_buffers = 2GB
   work_mem = 64MB
   maintenance_work_mem = 512MB
   effective_cache_size = 6GB
   max_connections = 100
   ```
4. **Use SSD storage** for PostgreSQL data directory
5. **Batch size**: Increase for bulk imports, decrease for real-time updates

## 🛠 Development

### Project Structure

```
bitcoin-indexer/
├── cmd/
│   └── indexer/           # Main application entry point
├── internal/
│   ├── api/               # HTTP API handlers and routes
│   ├── bitcoin/           # Bitcoin Core RPC client
│   ├── config/            # Configuration loading
│   ├── database/          # PostgreSQL interface
│   ├── indexer/           # Block processing logic
│   ├── models/            # Data models
│   ├── parser/            # Transaction parsing
│   └── wallet/            # HD wallet derivation
├── examples/              # Example scripts
├── go.mod                 # Go dependencies
├── Makefile              # Build automation
└── README.md             # This file
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/parser/...
```

### Adding Features

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for new functionality
5. Run tests (`go test ./...`)
6. Commit your changes (`git commit -m 'Add amazing feature'`)
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

## 🔍 Troubleshooting

### Common Issues

**"Connection refused" to Bitcoin Core**
- Ensure Bitcoin Core is running
- Check `bitcoin.conf` has correct RPC credentials
- Verify firewall allows connections to RPC port

**"Database connection failed"**
- Verify PostgreSQL is running
- Check DATABASE_URL format
- Ensure database exists

**"Block processing is slow"**
- Try parallel mode with more workers
- Check Bitcoin Core is fully synced
- Monitor system resources (CPU, RAM, disk I/O)

**"Address not found"**
- Ensure the address is being watched (`POST /api/v1/watch`)
- Check indexer has processed relevant blocks
- Verify address format is correct

## 📄 License

This project is released into the public domain under the [Unlicense](LICENSE).

## 🙏 Acknowledgments

- Bitcoin Core developers for the robust node implementation
- Go community for excellent libraries
- PostgreSQL team for the powerful database

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/gauryvg98/Bitcoin-Indexer/issues)
- **Discussions**: [GitHub Discussions](https://github.com/gauryvg98/Bitcoin-Indexer/discussions)

---

**Built with ❤️ for the Bitcoin community**

