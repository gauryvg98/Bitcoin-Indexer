using System.Text.Json.Serialization;

namespace BitcoinIndexer.Core.DTOs;

/// <summary>
/// Represents a Bitcoin transaction output
/// </summary>
public class BitcoinTransactionOutput
{
    /// <summary>
    /// Output value in BTC
    /// </summary>
    [JsonPropertyName("value")]
    public decimal Value { get; set; }

    /// <summary>
    /// Output index
    /// </summary>
    [JsonPropertyName("n")]
    public int N { get; set; }

    /// <summary>
    /// Script public key
    /// </summary>
    [JsonPropertyName("scriptPubKey")]
    public BitcoinScriptPubKey ScriptPubKey { get; set; } = new();
}

/// <summary>
/// Represents a script public key
/// </summary>
public class BitcoinScriptPubKey
{
    /// <summary>
    /// Script assembly
    /// </summary>
    [JsonPropertyName("asm")]
    public string Asm { get; set; } = string.Empty;

    /// <summary>
    /// Script hex
    /// </summary>
    [JsonPropertyName("hex")]
    public string Hex { get; set; } = string.Empty;

    /// <summary>
    /// Script type
    /// </summary>
    [JsonPropertyName("type")]
    public string Type { get; set; } = string.Empty;

    /// <summary>
    /// Bitcoin address (singular, newer Bitcoin Core versions)
    /// </summary>
    [JsonPropertyName("address")]
    public string? Address { get; set; }

    /// <summary>
    /// Bitcoin addresses (plural, older Bitcoin Core versions)
    /// </summary>
    [JsonPropertyName("addresses")]
    public string[]? Addresses { get; set; }
}
