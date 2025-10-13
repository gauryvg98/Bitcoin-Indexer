using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace BitcoinIndexer.Core.Models;

/// <summary>
/// Represents metadata about a Bitcoin address (matches Go address_metadata table)
/// </summary>
public class AddressMetadata
{
    /// <summary>
    /// Bitcoin address (primary key)
    /// </summary>
    [Key]
    [MaxLength(100)]
    public string Address { get; set; } = string.Empty;

    /// <summary>
    /// When this address was first seen
    /// </summary>
    public DateTime? FirstSeenAt { get; set; }

    /// <summary>
    /// When this address was last seen
    /// </summary>
    public DateTime? LastSeenAt { get; set; }

    /// <summary>
    /// Total received amount in satoshis
    /// </summary>
    public long TotalReceivedSats { get; set; }

    /// <summary>
    /// Total sent amount in satoshis
    /// </summary>
    public long TotalSentSats { get; set; }

    /// <summary>
    /// Total number of transactions for this address
    /// </summary>
    public int TransactionCount { get; set; }

    /// <summary>
    /// Whether this address is being watched
    /// </summary>
    public bool IsWatched { get; set; }

    /// <summary>
    /// When this metadata was first indexed
    /// </summary>
    public DateTime CreatedAt { get; set; } = DateTime.UtcNow;

    /// <summary>
    /// When this metadata was last updated
    /// </summary>
    public DateTime? UpdatedAt { get; set; }
}
