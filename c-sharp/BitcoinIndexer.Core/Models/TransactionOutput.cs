using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace BitcoinIndexer.Core.Models;

/// <summary>
/// Represents a transaction output (matches Go transaction_output table)
/// </summary>
public class TransactionOutput
{
    /// <summary>
    /// Transaction ID this output belongs to (part of composite key)
    /// </summary>
    [Required]
    [MaxLength(64)]
    public string Txid { get; set; } = string.Empty;

    /// <summary>
    /// Output index within the transaction (part of composite key)
    /// </summary>
    public int Vout { get; set; }

    /// <summary>
    /// Output value in satoshis
    /// </summary>
    public long ValueSats { get; set; }

    /// <summary>
    /// Bitcoin address
    /// </summary>
    [Required]
    [MaxLength(100)]
    public string Address { get; set; } = string.Empty;

    /// <summary>
    /// Script type (P2PKH, P2SH, P2WPKH, P2WSH, etc.)
    /// </summary>
    [MaxLength(20)]
    public string? ScriptType { get; set; }

    /// <summary>
    /// Output script (hex)
    /// </summary>
    [MaxLength(10000)]
    public string? ScriptHex { get; set; }

    /// <summary>
    /// Script assembly representation
    /// </summary>
    [MaxLength(10000)]
    public string? ScriptAsm { get; set; }
}
