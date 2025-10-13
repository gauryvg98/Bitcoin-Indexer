namespace BitcoinIndexer.Core.Models;

/// <summary>
/// Represents address balance information
/// </summary>
public class Balance
{
    /// <summary>
    /// Bitcoin address
    /// </summary>
    public string Address { get; set; } = string.Empty;

    /// <summary>
    /// Confirmed balance in satoshis
    /// </summary>
    public long ConfirmedSats { get; set; }

    /// <summary>
    /// Unconfirmed balance in satoshis
    /// </summary>
    public long UnconfirmedSats { get; set; }

    /// <summary>
    /// Total balance in satoshis
    /// </summary>
    public long TotalSats { get; set; }

    /// <summary>
    /// Total number of UTXOs
    /// </summary>
    public int UtxoCount { get; set; }

    /// <summary>
    /// Number of confirmed UTXOs
    /// </summary>
    public int ConfirmedUtxoCount { get; set; }

    /// <summary>
    /// Last indexed block height
    /// </summary>
    public int LastIndexedHeight { get; set; }
}
