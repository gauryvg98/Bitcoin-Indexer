using System.Text.Json.Serialization;

namespace BitcoinIndexer.Core.DTOs;

/// <summary>
/// Represents a Bitcoin transaction
/// </summary>
public class BitcoinTransaction
{
    /// <summary>
    /// Transaction ID
    /// </summary>
    [JsonPropertyName("txid")]
    public string Txid { get; set; } = string.Empty;

    /// <summary>
    /// Transaction hash
    /// </summary>
    [JsonPropertyName("hash")]
    public string Hash { get; set; } = string.Empty;

    /// <summary>
    /// Transaction version
    /// </summary>
    [JsonPropertyName("version")]
    public int Version { get; set; }

    /// <summary>
    /// Transaction size in bytes
    /// </summary>
    [JsonPropertyName("size")]
    public int Size { get; set; }

    /// <summary>
    /// Virtual size
    /// </summary>
    [JsonPropertyName("vsize")]
    public int VSize { get; set; }

    /// <summary>
    /// Transaction weight
    /// </summary>
    [JsonPropertyName("weight")]
    public int Weight { get; set; }

    /// <summary>
    /// Lock time
    /// </summary>
    [JsonPropertyName("locktime")]
    public int LockTime { get; set; }

    /// <summary>
    /// Transaction inputs
    /// </summary>
    [JsonPropertyName("vin")]
    public BitcoinTransactionInput[] Inputs { get; set; } = Array.Empty<BitcoinTransactionInput>();

    /// <summary>
    /// Transaction outputs
    /// </summary>
    [JsonPropertyName("vout")]
    public BitcoinTransactionOutput[] Outputs { get; set; } = Array.Empty<BitcoinTransactionOutput>();

    /// <summary>
    /// Raw transaction hex
    /// </summary>
    [JsonPropertyName("hex")]
    public string? Hex { get; set; }
}
