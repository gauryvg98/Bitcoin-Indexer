using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace BitcoinIndexer.Core.Models;

/// <summary>
/// Represents a reference to a transaction for an address (matches Go tx_reference table)
/// </summary>
public class TransactionReference
{
    /// <summary>
    /// Primary key
    /// </summary>
    [Key]
    public int Id { get; set; }

    /// <summary>
    /// Transaction ID
    /// </summary>
    [Required]
    [MaxLength(64)]
    public string Txid { get; set; } = string.Empty;

    /// <summary>
    /// Bitcoin address
    /// </summary>
    [Required]
    [MaxLength(100)]
    public string Address { get; set; } = string.Empty;

    /// <summary>
    /// Direction: "in" or "out"
    /// </summary>
    [Required]
    [MaxLength(10)]
    public string Direction { get; set; } = string.Empty;

    /// <summary>
    /// Value in satoshis
    /// </summary>
    public long ValueSats { get; set; }

    /// <summary>
    /// Sender address (for incoming transactions)
    /// </summary>
    [MaxLength(100)]
    public string? SenderAddress { get; set; }

    /// <summary>
    /// Receiver address (for outgoing transactions)
    /// </summary>
    [MaxLength(100)]
    public string? ReceiverAddress { get; set; }

    /// <summary>
    /// Block height where this transaction was confirmed
    /// </summary>
    public int? BlockHeight { get; set; }

    /// <summary>
    /// Block timestamp (Unix timestamp)
    /// </summary>
    public int? BlockTimestamp { get; set; }

    /// <summary>
    /// When this reference was first indexed
    /// </summary>
    public DateTime CreatedAt { get; set; } = DateTime.UtcNow;
}
