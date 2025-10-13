using BitcoinIndexer.Core.Models;
using BitcoinIndexer.Core.DTOs;

namespace BitcoinIndexer.Core.Interfaces;

/// <summary>
/// Interface for the Bitcoin indexer service
/// </summary>
public interface IBitcoinIndexerService
{
    /// <summary>
    /// Starts the indexing process
    /// </summary>
    Task StartAsync(CancellationToken cancellationToken = default);

    /// <summary>
    /// Stops the indexing process
    /// </summary>
    Task StopAsync(CancellationToken cancellationToken = default);

    /// <summary>
    /// Gets the current status of the indexer
    /// </summary>
    Task<IndexerStatus> GetStatusAsync();

    /// <summary>
    /// Processes blocks from a specific height range
    /// </summary>
    Task ProcessBlocksAsync(int startHeight, int endHeight, CancellationToken cancellationToken = default);

    /// <summary>
    /// Processes mempool transactions
    /// </summary>
    Task ProcessMempoolAsync(CancellationToken cancellationToken = default);

    /// <summary>
    /// Gets performance metrics
    /// </summary>
    Task<IndexerMetrics> GetMetricsAsync();

    /// <summary>
    /// Adds a watch target
    /// </summary>
    Task AddWatchTargetAsync(WatchTarget watchTarget);

    /// <summary>
    /// Gets all watch targets
    /// </summary>
    Task<IEnumerable<WatchTarget>> GetWatchTargetsAsync();

    /// <summary>
    /// Gets balance for an address
    /// </summary>
    Task<Balance> GetBalanceAsync(string address, int confirmations);

    /// <summary>
    /// Gets UTXOs for an address
    /// </summary>
    Task<IEnumerable<UtxoInfo>> GetUtxosAsync(string address);

    /// <summary>
    /// Gets transaction history for an address
    /// </summary>
    Task<IEnumerable<TransactionReference>> GetTransactionHistoryAsync(string address, int limit, int offset);

    /// <summary>
    /// Gets transaction history count for an address
    /// </summary>
    Task<long> GetTransactionHistoryCountAsync(string address);

    /// <summary>
    /// Gets the current block count from Bitcoin RPC
    /// </summary>
    Task<int> GetBlockCountAsync();

    /// <summary>
    /// Gets block hash for a given height
    /// </summary>
    Task<string> GetBlockHashAsync(int height);

    /// <summary>
    /// Gets block data for a given hash
    /// </summary>
    Task<BitcoinBlock> GetBlockAsync(string hash);
}

/// <summary>
/// Represents the status of the indexer
/// </summary>
public class IndexerStatus
{
    public bool IsRunning { get; set; }
    public int LastProcessedHeight { get; set; }
    public string? LastProcessedBlockHash { get; set; }
    public DateTime LastUpdateTime { get; set; }
    public int TotalProcessedBlocks { get; set; }
    public int TotalProcessedTransactions { get; set; }
    public string? ErrorMessage { get; set; }
}

/// <summary>
/// Represents indexer performance metrics
/// </summary>
public class IndexerMetrics
{
    public int TotalBlocks { get; set; }
    public int TotalTransactions { get; set; }
    public int TotalInputs { get; set; }
    public int TotalOutputs { get; set; }
    public int TotalNewUtxos { get; set; }
    public int TotalSpentUtxos { get; set; }
    public double BlocksPerSecond { get; set; }
    public double AverageTransactionsPerBlock { get; set; }
    public TimeSpan TotalProcessingTime { get; set; }
    public TimeSpan AverageBlockProcessingTime { get; set; }
    public DateTime LastUpdated { get; set; }
}
