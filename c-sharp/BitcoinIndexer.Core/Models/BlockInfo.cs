using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace BitcoinIndexer.Core.Models;

/// <summary>
/// Represents information about a Bitcoin block
/// </summary>
public class BlockInfo
{
    /// <summary>
    /// Block height (primary key)
    /// </summary>
    [Key]
    public int Height { get; set; }

    /// <summary>
    /// Block hash
    /// </summary>
    [MaxLength(64)]
    public string Hash { get; set; } = string.Empty;

    /// <summary>
    /// Previous block hash
    /// </summary>
    [MaxLength(64)]
    public string? PreviousHash { get; set; }

    /// <summary>
    /// Block timestamp
    /// </summary>
    public DateTime Timestamp { get; set; }

    /// <summary>
    /// Block size in bytes
    /// </summary>
    public int? Size { get; set; }

    /// <summary>
    /// Block weight
    /// </summary>
    public int? Weight { get; set; }

    /// <summary>
    /// Number of transactions in the block
    /// </summary>
    public int? TransactionCount { get; set; }

    /// <summary>
    /// When this block was first indexed
    /// </summary>
    public DateTime CreatedAt { get; set; } = DateTime.UtcNow;
}
