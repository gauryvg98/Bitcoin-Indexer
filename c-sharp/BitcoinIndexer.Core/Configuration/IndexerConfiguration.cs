using System.ComponentModel.DataAnnotations;

namespace BitcoinIndexer.Core.Configuration;

/// <summary>
/// Configuration settings for the Bitcoin indexer
/// </summary>
public class IndexerConfiguration
{
    /// <summary>
    /// Bitcoin RPC URL
    /// </summary>
    [Required]
    public string BitcoinRpcUrl { get; set; } = string.Empty;

    /// <summary>
    /// Bitcoin RPC username
    /// </summary>
    [Required]
    public string BitcoinRpcUsername { get; set; } = string.Empty;

    /// <summary>
    /// Bitcoin RPC password
    /// </summary>
    [Required]
    public string BitcoinRpcPassword { get; set; } = string.Empty;

    /// <summary>
    /// Database connection string
    /// </summary>
    [Required]
    public string DatabaseUrl { get; set; } = string.Empty;

    /// <summary>
    /// Block polling interval in seconds
    /// </summary>
    public int BlockPollIntervalSeconds { get; set; } = 10;

    /// <summary>
    /// Mempool polling interval in seconds
    /// </summary>
    public int MempoolPollIntervalSeconds { get; set; } = 5;

    /// <summary>
    /// Number of backfill workers
    /// </summary>
    [Range(1, int.MaxValue, ErrorMessage = "BackfillWorkers must be at least 1")]
    public int BackfillWorkers { get; set; } = 10;

    /// <summary>
    /// Maximum reorg depth
    /// </summary>
    public int MaxReorgDepth { get; set; } = 10;

    /// <summary>
    /// API server port
    /// </summary>
    public string ServerPort { get; set; } = "8080";

    /// <summary>
    /// Block polling interval as TimeSpan
    /// </summary>
    public TimeSpan BlockPollInterval => TimeSpan.FromSeconds(BlockPollIntervalSeconds);

    /// <summary>
    /// Mempool polling interval as TimeSpan
    /// </summary>
    public TimeSpan MempoolPollInterval => TimeSpan.FromSeconds(MempoolPollIntervalSeconds);
}
