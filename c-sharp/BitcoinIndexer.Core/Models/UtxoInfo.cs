namespace BitcoinIndexer.Core.Models;

/// <summary>
/// Represents UTXO information for API responses
/// </summary>
public class UtxoInfo
{
    /// <summary>
    /// Transaction ID
    /// </summary>
    public string Txid { get; set; } = string.Empty;

    /// <summary>
    /// Output index
    /// </summary>
    public int Vout { get; set; }

    /// <summary>
    /// Value in satoshis
    /// </summary>
    public long ValueSats { get; set; }

    /// <summary>
    /// Status: mempool, confirmed, spent
    /// </summary>
    public string Status { get; set; } = string.Empty;

    /// <summary>
    /// Number of confirmations
    /// </summary>
    public int Confirmations { get; set; }

    /// <summary>
    /// Block height
    /// </summary>
    public int? BlockHeight { get; set; }

    /// <summary>
    /// Block hash
    /// </summary>
    public string? BlockHash { get; set; }

    /// <summary>
    /// When first seen (ISO 8601 format)
    /// </summary>
    public string FirstSeenAt { get; set; } = string.Empty;

    /// <summary>
    /// When spent (ISO 8601 format)
    /// </summary>
    public string? SpentAt { get; set; }
}
