# Bitcoin Indexer - C# Implementation

A high-performance Bitcoin blockchain indexer built with .NET 9 and Entity Framework Core, designed to track UTXOs, transactions, and provide real-time blockchain data.

## Features

- **Complete UTXO Tracking**: Stores ALL transaction outputs including OP_RETURN and unknown script types
- **Real-time Indexing**: Processes new blocks and mempool transactions automatically
- **Sophisticated Address Extraction**: Handles all Bitcoin address types and derives identifiers for non-standard scripts
- **High Performance**: Uses Entity Framework Core with optimized batch operations
- **Concurrent Processing**: Implements proper DbContext management with `IServiceScopeFactory`
- **RESTful API**: Provides endpoints for querying blockchain data
- **PostgreSQL Support**: Uses PostgreSQL for robust data storage

## Architecture

### Project Structure

```
BitcoinIndexer.Api/           # Web API layer with controllers
BitcoinIndexer.Application/   # Business logic and services
BitcoinIndexer.Core/          # Domain models and interfaces
BitcoinIndexer.Infrastructure/ # Data access and external services
```

### Key Components

- **BitcoinIndexerService**: Core indexing logic with background processing
- **BitcoinIndexerHostedService**: Background service for automatic startup
- **BitcoinRpcClient**: Bitcoin Core RPC integration
- **BitcoinIndexerRepository**: Data access layer with Entity Framework Core
- **AddressExtractor**: Sophisticated address extraction utility

## Quick Start

### Prerequisites

- .NET 9 SDK
- PostgreSQL database
- Bitcoin Core node with RPC enabled

### Configuration

1. **Copy environment configuration**:
   ```bash
   cp config.env.example config.env
   ```

2. **Update `config.env`** with your settings:
   ```env
   BTC_RPC_URL=https://your-bitcoin-node:8332/
   BTC_RPC_USERNAME=your_username
   BTC_RPC_PASSWORD=your_password
   DATABASE_URL=postgres://user:password@localhost/bitcoin_indexer?sslmode=disable
   ```

3. **Run database migrations**:
   ```bash
   dotnet ef database update --project BitcoinIndexer.Infrastructure --startup-project BitcoinIndexer.Api
   ```

4. **Start the application**:
   ```bash
   dotnet run --project BitcoinIndexer.Api
   ```

The indexer will automatically start processing blocks and begin backfilling from the last processed height.

## API Endpoints

### Health Check
```http
GET /health
```

### Indexer Status
```http
GET /v1/btc/status
```

### Wallet Operations
```http
GET /v1/btc/wallet/{address}/balance?confirmations=6
GET /v1/btc/wallet/{address}/utxos
GET /v1/btc/wallet/{address}/transactions?limit=50&offset=0
```

### Watch Targets
```http
POST /v1/btc/watch-targets
GET /v1/btc/watch-targets
```

## UTXO Processing Logic

### Address Extraction Priority

The indexer uses a sophisticated address extraction system:

1. **Primary**: Bitcoin Core's `address` field (singular, newer versions)
2. **Fallback**: Bitcoin Core's `addresses[0]` field (plural, older versions)  
3. **Final Fallback**: Derived identifier from script hex

### Script Type Detection

Supports all Bitcoin script types:
- **P2PKH**: Legacy addresses (`1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa`)
- **P2SH**: Multisig addresses (`3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy`)
- **P2WPKH**: Bech32 addresses (`bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4`)
- **P2WSH**: Bech32 multisig
- **P2TR**: Bech32m Taproot addresses (`bc1p...`)
- **OP_RETURN**: Data storage transactions

### UTXO Storage Strategy

- **ALL Outputs Stored**: Every transaction output becomes a UTXO
- **Derived Identifiers**: Non-standard scripts get identifiers like `p2pkh_abc123`, `op_return_xyz789`
- **Status Tracking**: `confirmed` (in block) or `mempool` (unconfirmed)
- **Spent Tracking**: UTXOs marked as spent when referenced as inputs

## Performance Optimizations

### Database Operations
- **Batch Processing**: Uses Entity Framework Core batch operations
- **No Tracking**: `QueryTrackingBehavior.NoTracking` for read operations
- **Connection Pooling**: PostgreSQL connection pooling enabled
- **Raw SQL**: Uses raw SQL for complex operations like upserts

### Concurrency Management
- **IServiceScopeFactory**: Creates new DbContext for each block to avoid concurrency issues
- **SemaphoreSlim**: Prevents concurrent block processing
- **Background Services**: Non-blocking background processing

### Memory Management
- **Caching**: Watched scripts cached with 5-minute expiry
- **Streaming**: Processes large blocks without loading everything into memory
- **Disposal**: Proper disposal of resources and database connections

## Configuration Options

### Indexer Configuration (`appsettings.json`)

```json
{
  "IndexerConfiguration": {
    "BlockPollIntervalSeconds": 10,
    "MempoolPollIntervalSeconds": 5,
    "BackfillWorkers": 10,
    "MaxReorgDepth": 10
  }
}
```

### Database Configuration

```json
{
  "ConnectionStrings": {
    "DefaultConnection": "postgres://user:password@localhost/bitcoin_indexer?sslmode=disable"
  }
}
```

## Monitoring and Logging

### Health Checks
- Application health: `GET /health`
- Indexer status: `GET /v1/btc/status`

### Logging
- Structured logging with Serilog
- Entity Framework query logging (development only)
- Performance metrics and timing

### Metrics Available
- `last_height`: Last processed block height
- `total_processed_blocks`: Total blocks processed
- `total_processed_transactions`: Total transactions processed
- `is_running`: Indexer running status
- `error_message`: Any current errors

## Troubleshooting

### Common Issues

1. **DbContext Concurrency Errors**
   - **Solution**: The implementation uses `IServiceScopeFactory` to create new contexts per block
   - **Check**: Ensure proper disposal of scopes

2. **Database Connection Issues**
   - **Solution**: Verify PostgreSQL connection string and network access
   - **Check**: Database server is running and accessible

3. **Bitcoin RPC Connection Issues**
   - **Solution**: Verify RPC credentials and network access
   - **Check**: Bitcoin Core node is running with RPC enabled

4. **Missing UTXOs**
   - **Solution**: The implementation now stores ALL UTXOs including OP_RETURN
   - **Check**: Verify address extraction is working correctly

### Performance Issues

1. **Slow Block Processing**
   - **Check**: Database connection pool settings
   - **Check**: Entity Framework query performance
   - **Check**: Network latency to Bitcoin node

2. **High Memory Usage**
   - **Check**: Batch size settings
   - **Check**: Caching configuration
   - **Check**: Connection disposal

## Development

### Building
```bash
dotnet build
```

### Testing
```bash
dotnet test
```

### Database Migrations
```bash
# Add migration
dotnet ef migrations add MigrationName --project BitcoinIndexer.Infrastructure --startup-project BitcoinIndexer.Api

# Update database
dotnet ef database update --project BitcoinIndexer.Infrastructure --startup-project BitcoinIndexer.Api
```

### Code Quality
- Follows C# coding conventions
- Uses dependency injection
- Implements proper error handling
- Includes comprehensive logging

## Comparison with Go Implementation

This C# implementation is functionally equivalent to the Go implementation:

- ✅ **Same UTXO Storage**: Both store ALL UTXOs including OP_RETURN
- ✅ **Same Address Extraction**: Both use identical address extraction logic
- ✅ **Same Database Schema**: Both use the same PostgreSQL schema
- ✅ **Same API Endpoints**: Both provide equivalent REST APIs
- ✅ **Same Performance**: Both achieve similar indexing performance

See [IMPLEMENTATION_COMPARISON.md](../IMPLEMENTATION_COMPARISON.md) for detailed comparison.

## License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.