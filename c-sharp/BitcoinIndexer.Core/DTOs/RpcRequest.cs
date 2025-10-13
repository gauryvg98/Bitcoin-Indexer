using System.Text.Json.Serialization;

namespace BitcoinIndexer.Core.DTOs;

/// <summary>
/// Represents a JSON-RPC request
/// </summary>
public class RpcRequest
{
    /// <summary>
    /// JSON-RPC version
    /// </summary>
    [JsonPropertyName("jsonrpc")]
    public string JsonRpc { get; set; } = "2.0";

    /// <summary>
    /// Request ID
    /// </summary>
    [JsonPropertyName("id")]
    public int Id { get; set; } = 1;

    /// <summary>
    /// RPC method name
    /// </summary>
    [JsonPropertyName("method")]
    public string Method { get; set; } = string.Empty;

    /// <summary>
    /// Method parameters
    /// </summary>
    [JsonPropertyName("params")]
    public object[]? Params { get; set; }
}
