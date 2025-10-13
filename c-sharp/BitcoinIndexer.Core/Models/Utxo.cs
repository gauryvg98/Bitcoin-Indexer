using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace BitcoinIndexer.Core.Models;

/// <summary>
/// Represents unspent transaction outputs
/// </summary>
public class Utxo
{
    /// <summary>
    /// Transaction ID (part of composite key)
    /// </summary>
    [Required]
    [MaxLength(64)]
    public string Txid { get; set; } = string.Empty;

    /// <summary>
    /// Output index (part of composite key)
    /// </summary>
    public int Vout { get; set; }

    /// <summary>
    /// Bitcoin address
    /// </summary>
    [Required]
    [MaxLength(100)]
    public string Address { get; set; } = string.Empty;

    /// <summary>
    /// Script hex
    /// </summary>
    [Required]
    [MaxLength(1000)]
    public string ScriptHex { get; set; } = string.Empty;

    /// <summary>
    /// Value in satoshis
    /// </summary>
    public long ValueSats { get; set; }

    /// <summary>
    /// Status: mempool, confirmed, spent
    /// </summary>
    [Required]
    [MaxLength(20)]
    public string Status { get; set; } = string.Empty;

    /// <summary>
    /// Block height where this UTXO was confirmed
    /// </summary>
    public int? BlockHeight { get; set; }

    /// <summary>
    /// Block hash where this UTXO was confirmed
    /// </summary>
    [MaxLength(64)]
    public string? BlockHash { get; set; }

    /// <summary>
    /// When this UTXO was first seen
    /// </summary>
    public DateTime FirstSeenAt { get; set; } = DateTime.UtcNow;

    /// <summary>
    /// When this UTXO was spent (if applicable)
    /// </summary>
    public DateTime? SpentAt { get; set; }

    /// <summary>
    /// Transaction ID that spent this UTXO
    /// </summary>
    [MaxLength(64)]
    public string? SpentByTxid { get; set; }
}
