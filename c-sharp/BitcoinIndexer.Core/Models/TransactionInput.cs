using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace BitcoinIndexer.Core.Models;

/// <summary>
/// Represents a transaction input (matches Go transaction_input table)
/// </summary>
public class TransactionInput
{
    /// <summary>
    /// Transaction ID this input belongs to (part of composite key)
    /// </summary>
    [Required]
    [MaxLength(64)]
    public string Txid { get; set; } = string.Empty;

    /// <summary>
    /// Input index within the transaction (part of composite key)
    /// </summary>
    public int Vout { get; set; }

    /// <summary>
    /// Previous transaction ID (null for coinbase)
    /// </summary>
    [MaxLength(64)]
    public string? PrevTxid { get; set; }

    /// <summary>
    /// Previous output index (null for coinbase)
    /// </summary>
    public int PrevVout { get; set; }

    /// <summary>
    /// Input script (hex)
    /// </summary>
    [MaxLength(10000)]
    public string? ScriptSig { get; set; }

    /// <summary>
    /// Sequence number
    /// </summary>
    public long Sequence { get; set; }

    /// <summary>
    /// Witness data (for SegWit transactions) - stored as text array
    /// </summary>
    public string[]? Witness { get; set; }
}
