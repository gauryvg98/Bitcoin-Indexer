using System.Text.Json.Serialization;

namespace BitcoinIndexer.Core.DTOs;

/// <summary>
/// Represents a JSON-RPC response
/// </summary>
public class RpcResponse
{
    /// <summary>
    /// JSON-RPC version
    /// </summary>
    [JsonPropertyName("jsonrpc")]
    public string JsonRpc { get; set; } = string.Empty;

    /// <summary>
    /// Request ID
    /// </summary>
    [JsonPropertyName("id")]
    public int Id { get; set; }

    /// <summary>
    /// Response result
    /// </summary>
    [JsonPropertyName("result")]
    public object? Result { get; set; }

    /// <summary>
    /// RPC error (if any)
    /// </summary>
    [JsonPropertyName("error")]
    public RpcError? Error { get; set; }
}

/// <summary>
/// Represents an RPC error
/// </summary>
public class RpcError
{
    /// <summary>
    /// Error code
    /// </summary>
    [JsonPropertyName("code")]
    public int Code { get; set; }

    /// <summary>
    /// Error message
    /// </summary>
    [JsonPropertyName("message")]
    public string Message { get; set; } = string.Empty;
}
