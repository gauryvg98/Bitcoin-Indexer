using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace BitcoinIndexer.Core.Models;

/// <summary>
/// Represents a complete Bitcoin transaction
/// </summary>
public class Transaction
{
    /// <summary>
    /// Transaction ID (primary key)
    /// </summary>
    [Key]
    [MaxLength(64)]
    public string Txid { get; set; } = string.Empty;

    /// <summary>
    /// Block height where this transaction was confirmed
    /// </summary>
    public int? BlockHeight { get; set; }

    /// <summary>
    /// Block hash where this transaction was confirmed
    /// </summary>
    [MaxLength(64)]
    public string? BlockHash { get; set; }

    /// <summary>
    /// Block timestamp
    /// </summary>
    public DateTime? BlockTime { get; set; }

    /// <summary>
    /// Transaction size in bytes
    /// </summary>
    public int? Size { get; set; }

    /// <summary>
    /// Transaction weight
    /// </summary>
    public int? Weight { get; set; }

    /// <summary>
    /// Transaction fee in satoshis
    /// </summary>
    public long? FeeSats { get; set; }

    /// <summary>
    /// Whether this is a coinbase transaction
    /// </summary>
    public bool IsCoinbase { get; set; }

    /// <summary>
    /// When this transaction was first indexed
    /// </summary>
    public DateTime CreatedAt { get; set; } = DateTime.UtcNow;

    /// <summary>
    /// When this transaction was last updated
    /// </summary>
    public DateTime? UpdatedAt { get; set; }

    /// <summary>
    /// Navigation property to transaction inputs
    /// </summary>
    public ICollection<TransactionInput> Inputs { get; set; } = new List<TransactionInput>();

    /// <summary>
    /// Navigation property to transaction outputs
    /// </summary>
    public ICollection<TransactionOutput> Outputs { get; set; } = new List<TransactionOutput>();
}
