using BitcoinIndexer.Core.Configuration;
using BitcoinIndexer.Core.DTOs;
using BitcoinIndexer.Core.Interfaces;
using BitcoinIndexer.Core.Models;
using BitcoinIndexer.Core.Utilities;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace BitcoinIndexer.Application.Services;

/// <summary>
/// Bitcoin indexer service implementation
/// </summary>
public class BitcoinIndexerService : IBitcoinIndexerService, IDisposable
{
    private readonly IBitcoinRpcClient _rpcClient;
    private readonly IServiceScopeFactory _scopeFactory;
    private readonly IndexerConfiguration _config;
    private readonly ILogger<BitcoinIndexerService> _logger;
    private readonly SemaphoreSlim _processingSemaphore;
    private readonly Dictionary<string, WatchedScript> _watchedScriptsCache;
    private readonly SemaphoreSlim _cacheSemaphore;
    private DateTime _cacheExpiry;

    private volatile bool _isRunning;
    private volatile int _lastProcessedHeight;
    private volatile string? _lastProcessedBlockHash;
    private DateTime _lastUpdateTime;
    private volatile int _totalProcessedBlocks;
    private volatile int _totalProcessedTransactions;
    private volatile string? _errorMessage;

    public BitcoinIndexerService(
        IBitcoinRpcClient rpcClient,
        IServiceScopeFactory scopeFactory,
        IOptions<IndexerConfiguration> config,
        ILogger<BitcoinIndexerService> logger)
    {
        _rpcClient = rpcClient;
        _scopeFactory = scopeFactory;
        _config = config.Value;
        _logger = logger;
        _processingSemaphore = new SemaphoreSlim(1, 1);
        _watchedScriptsCache = new Dictionary<string, WatchedScript>();
        _cacheSemaphore = new SemaphoreSlim(1, 1);
        _cacheExpiry = DateTime.UtcNow.AddHours(-1); // Force initial cache load
    }


    public async Task StartAsync(CancellationToken cancellationToken = default)
    {
        if (_isRunning)
        {
            _logger.LogWarning("Indexer is already running");
            return;
        }

        _isRunning = true;
        _errorMessage = null;
        _logger.LogInformation("Bitcoin indexer started");

        // Start block polling
        _ = Task.Run(() => BlockPollerAsync(cancellationToken), cancellationToken);

        // Start mempool polling
        // _ = Task.Run(() => MempoolPollerAsync(cancellationToken), cancellationToken);
    }

    public async Task StopAsync(CancellationToken cancellationToken = default)
    {
        if (!_isRunning)
        {
            _logger.LogWarning("Indexer is not running");
            return;
        }

        _isRunning = false;
        _logger.LogInformation("Bitcoin indexer stopped");
    }

    public async Task<IndexerStatus> GetStatusAsync()
    {
        return new IndexerStatus
        {
            IsRunning = _isRunning,
            LastProcessedHeight = _lastProcessedHeight,
            LastProcessedBlockHash = _lastProcessedBlockHash,
            LastUpdateTime = _lastUpdateTime,
            TotalProcessedBlocks = _totalProcessedBlocks,
            TotalProcessedTransactions = _totalProcessedTransactions,
            ErrorMessage = _errorMessage
        };
    }

    public async Task ProcessBlocksAsync(int startHeight, int endHeight, CancellationToken cancellationToken = default)
    {
        _logger.LogInformation("Starting to process blocks from {StartHeight} to {EndHeight}", startHeight, endHeight);
        
        await _processingSemaphore.WaitAsync(cancellationToken);
        try
        {
            for (int height = startHeight; height <= endHeight; height++)
            {
                if (cancellationToken.IsCancellationRequested)
                    break;

                _logger.LogInformation("About to process block {Height}", height);
                await ProcessBlockAsync(height, cancellationToken);
                _logger.LogInformation("Completed processing block {Height}", height);
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error in ProcessBlocksAsync from {StartHeight} to {EndHeight}", startHeight, endHeight);
            throw;
        }
        finally
        {
            _processingSemaphore.Release();
        }
    }

    public async Task ProcessMempoolAsync(CancellationToken cancellationToken = default)
    {
        try
        {
            var mempoolTxids = await _rpcClient.GetRawMempoolAsync();
            _logger.LogDebug("Processing {Count} mempool transactions", mempoolTxids.Length);

            foreach (var txid in mempoolTxids)
            {
                if (cancellationToken.IsCancellationRequested)
                    break;

                try
                {
                    var transaction = await _rpcClient.GetRawTransactionAsync(txid);
                    
                    // Create a new scope for each mempool transaction
                    using var scope = _scopeFactory.CreateScope();
                    var repository = scope.ServiceProvider.GetRequiredService<IBitcoinIndexerRepository>();
                    
                    await ProcessTransactionAsync(transaction, null, null, null, repository, cancellationToken);
                }
                catch (Exception ex)
                {
                    _logger.LogWarning(ex, "Failed to process mempool transaction {Txid}", txid);
                }
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error processing mempool");
            _errorMessage = ex.Message;
        }
    }

    public async Task<IndexerMetrics> GetMetricsAsync()
    {
        // This is a simplified implementation
        // In a real implementation, you would collect and aggregate metrics
        return new IndexerMetrics
        {
            TotalBlocks = _totalProcessedBlocks,
            TotalTransactions = _totalProcessedTransactions,
            LastUpdated = DateTime.UtcNow
        };
    }

    private async Task BlockPollerAsync(CancellationToken cancellationToken)
    {
        var timer = new PeriodicTimer(_config.BlockPollInterval);
        
        try
        {
            while (!cancellationToken.IsCancellationRequested && _isRunning)
            {
                try
                {
                    await ProcessBlocksAsync(cancellationToken);
                }
                catch (Exception ex)
                {
                    _logger.LogError(ex, "Error in block poller");
                    _errorMessage = ex.Message;
                }

                await timer.WaitForNextTickAsync(cancellationToken);
            }
        }
        finally
        {
            timer.Dispose();
        }
    }

    private async Task MempoolPollerAsync(CancellationToken cancellationToken)
    {
        var timer = new PeriodicTimer(_config.MempoolPollInterval);
        
        try
        {
            while (!cancellationToken.IsCancellationRequested && _isRunning)
            {
                try
                {
                    await ProcessMempoolAsync(cancellationToken);
                }
                catch (Exception ex)
                {
                    _logger.LogError(ex, "Error in mempool poller");
                    _errorMessage = ex.Message;
                }

                await timer.WaitForNextTickAsync(cancellationToken);
            }
        }
        finally
        {
            timer.Dispose();
        }
    }

    private async Task ProcessBlocksAsync(CancellationToken cancellationToken)
    {
        try
        {
            var currentHeight = await _rpcClient.GetBlockCountAsync();
            
            // Create a scope to get the repository
            using var scope = _scopeFactory.CreateScope();
            var repository = scope.ServiceProvider.GetRequiredService<IBitcoinIndexerRepository>();
            var progress = await repository.GetIndexProgressAsync();
            
            var startHeight = progress?.LastHeight + 1 ?? 0;
            
            if (startHeight <= currentHeight)
            {
                _logger.LogInformation("Processing blocks from height {StartHeight} to {CurrentHeight}", 
                    startHeight, currentHeight);
                
                await ProcessBlocksAsync(startHeight, currentHeight, cancellationToken);
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error processing blocks");
            _errorMessage = ex.Message;
        }
    }

    private async Task ProcessBlockAsync(int height, CancellationToken cancellationToken)
    {
        // Create a new scope for each block to avoid DbContext concurrency issues
        using var scope = _scopeFactory.CreateScope();
        var repository = scope.ServiceProvider.GetRequiredService<IBitcoinIndexerRepository>();
        
        try
        {
            _logger.LogInformation("Processing block at height {Height}", height);
            
            var blockHash = await _rpcClient.GetBlockHashAsync(height);
            _logger.LogInformation("Got block hash {BlockHash} for height {Height}", blockHash, height);
            
            var block = await _rpcClient.GetBlockAsync(blockHash);
            _logger.LogInformation("Got block data for height {Height}, hash {Hash}, {TxCount} transactions", 
                height, block.Hash, block.Transactions.Length);

            // Store block info
            var blockInfo = new BlockInfo
            {
                Hash = block.Hash,
                Height = block.Height,
                PreviousHash = block.PreviousHash,
                Timestamp = DateTimeOffset.FromUnixTimeSeconds(block.Time).UtcDateTime,
                Size = 0, // Would need to get from block data
                Weight = 0, // Would need to get from block data
                TransactionCount = block.Transactions.Length,
                CreatedAt = DateTime.UtcNow
            };

            await repository.AddBlockInfoAsync(blockInfo);
            _logger.LogInformation("Stored block info for height {Height}", height);

            // Process transactions
            foreach (var tx in block.Transactions)
            {
                await ProcessTransactionAsync(tx, block.Height, block.Hash, DateTimeOffset.FromUnixTimeSeconds(block.Time).UtcDateTime, repository, cancellationToken);
            }

            // Save all changes for this block first
            await repository.SaveChangesAsync();

            // Update progress after saving all changes
            var indexProgress = new IndexProgress
            {
                LastHeight = height,
                LastBlockHash = blockHash,
                UpdatedAt = DateTime.UtcNow
            };

            await repository.UpsertIndexProgressAsync(indexProgress);

            // Update status
            _lastProcessedHeight = height;
            _lastProcessedBlockHash = blockHash;
            _lastUpdateTime = DateTime.UtcNow;
            _totalProcessedBlocks++;
            _totalProcessedTransactions += block.Transactions.Length;

            _logger.LogDebug("Processed block {Height} with {TxCount} transactions", 
                height, block.Transactions.Length);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error processing block {Height}", height);
            _errorMessage = ex.Message;
        }
    }

    private async Task ProcessTransactionAsync(BitcoinTransaction tx, int? blockHeight, string? blockHash, DateTime? blockTime, IBitcoinIndexerRepository repository, CancellationToken cancellationToken)
    {
        try
        {
            // Store transaction
            var transaction = new Transaction
            {
                Txid = tx.Txid,
                BlockHeight = blockHeight,
                BlockHash = blockHash,
                BlockTime = blockTime,
                Size = tx.Size,
                Weight = tx.Weight,
                IsCoinbase = tx.Inputs.Length > 0 && string.IsNullOrEmpty(tx.Inputs[0].Txid),
                CreatedAt = blockTime ?? DateTime.UtcNow // Use block time for confirmed transactions, current time for mempool
            };

            await repository.AddTransactionAsync(transaction);

            // Process inputs
            for (int i = 0; i < tx.Inputs.Length; i++)
            {
                var input = tx.Inputs[i];
                
                // Skip coinbase inputs (they have no previous transaction)
                if (string.IsNullOrEmpty(input.Txid))
                {
                    continue;
                }
                
                var txInput = new TransactionInput
                {
                    Txid = tx.Txid,
                    Vout = i, // Use the index as the input number
                    PrevTxid = input.Txid,
                    PrevVout = input.Vout,
                    ScriptSig = input.ScriptSig.Hex,
                    Sequence = input.Sequence,
                    Witness = input.TxInWitness
                };

                await repository.AddTransactionInputAsync(txInput);

                // Mark previous output as spent
                await repository.UpdateUtxoStatusAsync(
                    input.Txid, input.Vout, "spent", blockTime, tx.Txid);
            }

            // Process outputs
            foreach (var output in tx.Outputs)
            {
                // Extract address with priority: address field > addresses[0] > derived identifier
                // This matches the Go implementation logic exactly
                var address = AddressExtractor.ExtractAddressFromScriptPubKey(output.ScriptPubKey);
                
                var txOutput = new TransactionOutput
                {
                    Txid = tx.Txid,
                    Vout = output.N,
                    Address = address,
                    ValueSats = (long)(output.Value * 100_000_000), // Convert BTC to satoshis
                    ScriptType = output.ScriptPubKey.Type,
                    ScriptHex = output.ScriptPubKey.Hex,
                    ScriptAsm = output.ScriptPubKey.Asm
                };

                await repository.AddTransactionOutputAsync(txOutput);

                // Create UTXO for ALL outputs (not just watched ones) - matching Go implementation
                // This now includes OP_RETURN, unknown scripts, and derived addresses
                var utxo = new Utxo
                {
                    Txid = txOutput.Txid,
                    Vout = txOutput.Vout,
                    Address = txOutput.Address, // Can be empty, derived identifier, or standard address
                    ScriptHex = txOutput.ScriptHex ?? string.Empty,
                    ValueSats = txOutput.ValueSats,
                    Status = blockHeight.HasValue ? "confirmed" : "mempool",
                    BlockHeight = blockHeight,
                    BlockHash = blockHash,
                    FirstSeenAt = blockTime ?? DateTime.UtcNow // Use block time for confirmed transactions, current time for mempool
                };

                await repository.UpsertUtxoAsync(utxo);

                // Only create transaction references for watched addresses (standard addresses only)
                if (!string.IsNullOrEmpty(address) && !address.StartsWith("p2pkh_") && 
                    !address.StartsWith("p2sh_") && !address.StartsWith("p2wpkh_") && 
                    !address.StartsWith("p2wsh_") && !address.StartsWith("p2tr_") && 
                    !address.StartsWith("op_return_") && !address.StartsWith("script_"))
                {
                    await ProcessWatchedAddressAsync(address, txOutput, blockHeight, repository, cancellationToken);
                }
            }

            // Don't call SaveChangesAsync here - let it be called at the block level
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error processing transaction {Txid}", tx.Txid);
            _errorMessage = ex.Message;
        }
    }

    private async Task ProcessWatchedAddressAsync(string address, TransactionOutput output, int? blockHeight, IBitcoinIndexerRepository repository, CancellationToken cancellationToken)
    {
        try
        {
            // Check if address is being watched
            var watchedScripts = await GetWatchedScriptsAsync();
            _logger.LogInformation("Processing address {Address}, watched scripts count: {Count}", address, watchedScripts.Count);
            
            if (watchedScripts.ContainsKey(address))
            {
                _logger.LogInformation("Address {Address} is being watched, creating transaction reference", address);
                
                // Create transaction reference for watched address (UTXO already created above)
                var txRef = new TransactionReference
                {
                    Txid = output.Txid,
                    Address = address,
                    Direction = "in", // This is an incoming transaction
                    ValueSats = output.ValueSats,
                    BlockHeight = blockHeight,
                    BlockTimestamp = blockHeight.HasValue ? (int)DateTimeOffset.UtcNow.ToUnixTimeSeconds() : null,
                    CreatedAt = DateTime.UtcNow
                };

                await repository.AddTransactionReferenceAsync(txRef);
                _logger.LogInformation("Created transaction reference for watched address {Address}: {Txid}", address, output.Txid);

                // Update address metadata
                var metadata = await repository.GetAddressMetadataAsync(address);
                if (metadata == null)
                {
                    metadata = new AddressMetadata
                    {
                        Address = address,
                        IsWatched = true,
                        FirstSeenAt = DateTime.UtcNow,
                        CreatedAt = DateTime.UtcNow
                    };
                }

                metadata.LastSeenAt = DateTime.UtcNow;
                metadata.TransactionCount++;
                metadata.TotalReceivedSats += output.ValueSats;
                metadata.UpdatedAt = DateTime.UtcNow;

                await repository.UpsertAddressMetadataAsync(metadata);
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error processing watched address {Address}", address);
        }
    }

    private async Task<Dictionary<string, WatchedScript>> GetWatchedScriptsAsync()
    {
        await _cacheSemaphore.WaitAsync();
        try
        {
            // Check if cache is still valid (refresh every 5 minutes)
            if (DateTime.UtcNow < _cacheExpiry && _watchedScriptsCache.Count > 0)
            {
                return _watchedScriptsCache;
            }

            // Cache expired or empty, refresh it
            using var scope = _scopeFactory.CreateScope();
            var repository = scope.ServiceProvider.GetRequiredService<IBitcoinIndexerRepository>();
            var scripts = await repository.GetAllWatchedScriptsAsync();
            _watchedScriptsCache.Clear();

            foreach (var script in scripts)
            {
                _watchedScriptsCache[script.Address] = script;
            }

            _cacheExpiry = DateTime.UtcNow.AddMinutes(5);
            return _watchedScriptsCache;
        }
        finally
        {
            _cacheSemaphore.Release();
        }
    }

    public async Task AddWatchTargetAsync(WatchTarget watchTarget)
    {
        using var scope = _scopeFactory.CreateScope();
        var repository = scope.ServiceProvider.GetRequiredService<IBitcoinIndexerRepository>();
        
        await repository.AddWatchTargetAsync(watchTarget);
        
        // For address-based watch targets, create a watched script entry
        if (watchTarget.Kind == "address" && !string.IsNullOrEmpty(watchTarget.Address))
        {
            var watchedScript = new WatchedScript
            {
                Address = watchTarget.Address,
                ScriptHex = watchTarget.Address, // For now, use address as script hex
                Type = "address",
                TargetId = watchTarget.Id
            };
            
            await repository.AddWatchedScriptAsync(watchedScript);
        }
        
        await InvalidateWatchedScriptsCacheAsync();
    }

    public async Task<IEnumerable<WatchTarget>> GetWatchTargetsAsync()
    {
        using var scope = _scopeFactory.CreateScope();
        var repository = scope.ServiceProvider.GetRequiredService<IBitcoinIndexerRepository>();
        return await repository.GetAllWatchTargetsAsync();
    }

    public async Task<Balance> GetBalanceAsync(string address, int confirmations)
    {
        using var scope = _scopeFactory.CreateScope();
        var repository = scope.ServiceProvider.GetRequiredService<IBitcoinIndexerRepository>();
        var utxos = await repository.GetUtxosByAddressAsync(address);
        var confirmedUtxos = new List<Utxo>();
        foreach (var utxo in utxos.Where(u => u.Status == "confirmed"))
        {
            if (utxo.BlockHeight == null || await GetConfirmationsAsync(utxo.BlockHeight.Value) >= confirmations)
            {
                confirmedUtxos.Add(utxo);
            }
        }

        var unconfirmedUtxos = utxos.Where(u => u.Status == "mempool").ToList();

        var balance = new Balance
        {
            Address = address,
            ConfirmedSats = confirmedUtxos.Sum(u => u.ValueSats),
            UnconfirmedSats = unconfirmedUtxos.Sum(u => u.ValueSats),
            TotalSats = utxos.Sum(u => u.ValueSats),
            UtxoCount = utxos.Count(),
            ConfirmedUtxoCount = confirmedUtxos.Count(),
            LastIndexedHeight = _lastProcessedHeight
        };

        return balance;
    }

    public async Task<IEnumerable<UtxoInfo>> GetUtxosAsync(string address)
    {
        using var scope = _scopeFactory.CreateScope();
        var repository = scope.ServiceProvider.GetRequiredService<IBitcoinIndexerRepository>();
        var utxos = await repository.GetUtxosByAddressAsync(address);
        var utxoInfos = new List<UtxoInfo>();

        foreach (var utxo in utxos)
        {
            var confirmations = utxo.BlockHeight.HasValue ? await GetConfirmationsAsync(utxo.BlockHeight.Value) : 0;
            utxoInfos.Add(new UtxoInfo
            {
                Txid = utxo.Txid,
                Vout = utxo.Vout,
                ValueSats = utxo.ValueSats,
                Status = utxo.Status,
                BlockHeight = utxo.BlockHeight,
                BlockHash = utxo.BlockHash,
                FirstSeenAt = utxo.FirstSeenAt.ToString("o"), // ISO 8601 format
                SpentAt = utxo.SpentAt?.ToString("o"),
                Confirmations = confirmations
            });
        }
        return utxoInfos;
    }

    public async Task<IEnumerable<TransactionReference>> GetTransactionHistoryAsync(string address, int limit, int offset)
    {
        using var scope = _scopeFactory.CreateScope();
        var repository = scope.ServiceProvider.GetRequiredService<IBitcoinIndexerRepository>();
        var allRefs = await repository.GetTransactionReferencesByAddressAsync(address);
        return allRefs.Skip(offset).Take(limit);
    }

    public async Task<long> GetTransactionHistoryCountAsync(string address)
    {
        using var scope = _scopeFactory.CreateScope();
        var repository = scope.ServiceProvider.GetRequiredService<IBitcoinIndexerRepository>();
        var allRefs = await repository.GetTransactionReferencesByAddressAsync(address);
        return allRefs.Count();
    }

    private async Task InvalidateWatchedScriptsCacheAsync()
    {
        await _cacheSemaphore.WaitAsync();
        try
        {
            _cacheExpiry = DateTime.UtcNow.AddHours(-1); // Force refresh on next access
            _logger.LogInformation("Watched scripts cache invalidated.");
        }
        finally
        {
            _cacheSemaphore.Release();
        }
    }

    private async Task<int> GetConfirmationsAsync(int blockHeight)
    {
        var currentBlockCount = await _rpcClient.GetBlockCountAsync();
        return currentBlockCount - blockHeight + 1;
    }

    public async Task<int> GetBlockCountAsync()
    {
        return await _rpcClient.GetBlockCountAsync();
    }

    public async Task<string> GetBlockHashAsync(int height)
    {
        return await _rpcClient.GetBlockHashAsync(height);
    }

    public async Task<BitcoinBlock> GetBlockAsync(string hash)
    {
        return await _rpcClient.GetBlockAsync(hash);
    }

    public void Dispose()
    {
        _processingSemaphore?.Dispose();
        _cacheSemaphore?.Dispose();
    }
}
