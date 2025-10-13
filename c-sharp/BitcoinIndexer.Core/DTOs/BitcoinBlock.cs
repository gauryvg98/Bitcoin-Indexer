using System.Text.Json.Serialization;

namespace BitcoinIndexer.Core.DTOs;

/// <summary>
/// Represents a Bitcoin block
/// </summary>
public class BitcoinBlock
{
    /// <summary>
    /// Block hash
    /// </summary>
    [JsonPropertyName("hash")]
    public string Hash { get; set; } = string.Empty;

    /// <summary>
    /// Block height
    /// </summary>
    [JsonPropertyName("height")]
    public int Height { get; set; }

    /// <summary>
    /// Previous block hash
    /// </summary>
    [JsonPropertyName("previousblockhash")]
    public string? PreviousHash { get; set; }

    /// <summary>
    /// Block timestamp
    /// </summary>
    [JsonPropertyName("time")]
    public long Time { get; set; }

    /// <summary>
    /// Block transactions
    /// </summary>
    [JsonPropertyName("tx")]
    public BitcoinTransaction[] Transactions { get; set; } = Array.Empty<BitcoinTransaction>();

    /// <summary>
    /// Number of confirmations
    /// </summary>
    [JsonPropertyName("confirmations")]
    public int Confirmations { get; set; }
}
