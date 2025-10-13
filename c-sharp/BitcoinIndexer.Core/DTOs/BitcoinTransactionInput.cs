using System.Text.Json.Serialization;

namespace BitcoinIndexer.Core.DTOs;

/// <summary>
/// Represents a Bitcoin transaction input
/// </summary>
public class BitcoinTransactionInput
{
    /// <summary>
    /// Previous transaction ID
    /// </summary>
    [JsonPropertyName("txid")]
    public string? Txid { get; set; }

    /// <summary>
    /// Previous output index
    /// </summary>
    [JsonPropertyName("vout")]
    public int Vout { get; set; }

    /// <summary>
    /// Script signature
    /// </summary>
    [JsonPropertyName("scriptSig")]
    public BitcoinScriptSig ScriptSig { get; set; } = new();

    /// <summary>
    /// Sequence number
    /// </summary>
    [JsonPropertyName("sequence")]
    public long Sequence { get; set; }

    /// <summary>
    /// Witness data (for SegWit transactions)
    /// </summary>
    [JsonPropertyName("txinwitness")]
    public string[]? TxInWitness { get; set; }
}

/// <summary>
/// Represents a script signature
/// </summary>
public class BitcoinScriptSig
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
}
