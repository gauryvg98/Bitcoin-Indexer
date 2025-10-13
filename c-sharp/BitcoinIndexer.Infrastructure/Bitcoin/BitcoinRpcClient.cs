using BitcoinIndexer.Core.Configuration;
using BitcoinIndexer.Core.DTOs;
using BitcoinIndexer.Core.Interfaces;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;
using System.Net;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace BitcoinIndexer.Infrastructure.Bitcoin;

/// <summary>
/// Bitcoin RPC client implementation
/// </summary>
public class BitcoinRpcClient : IBitcoinRpcClient
{
    private readonly HttpClient _httpClient;
    private readonly ILogger<BitcoinRpcClient> _logger;
    private readonly IndexerConfiguration _config;
    private readonly JsonSerializerOptions _jsonOptions;

    public string BaseUrl { get; }
    public string Username { get; }
    public string Password { get; }

    public BitcoinRpcClient(HttpClient httpClient, ILogger<BitcoinRpcClient> logger, IOptions<IndexerConfiguration> config)
    {
        _httpClient = httpClient;
        _logger = logger;
        _config = config.Value;

        BaseUrl = _config.BitcoinRpcUrl;
        Username = _config.BitcoinRpcUsername;
        Password = _config.BitcoinRpcPassword;

        _jsonOptions = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
            PropertyNameCaseInsensitive = true,
            NumberHandling = JsonNumberHandling.AllowReadingFromString
        };

        ConfigureHttpClient();
    }

    private void ConfigureHttpClient()
    {
        _httpClient.BaseAddress = new Uri(BaseUrl);
        _httpClient.Timeout = TimeSpan.FromSeconds(120); // Increased timeout for large blocks

        // Set up basic authentication
        var credentials = Convert.ToBase64String(Encoding.ASCII.GetBytes($"{Username}:{Password}"));
        _httpClient.DefaultRequestHeaders.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Basic", credentials);
    }

    /// <summary>
    /// Makes an RPC call with retry logic
    /// </summary>
    private async Task<T> CallWithRetryAsync<T>(string method, object[]? parameters = null, int maxRetries = 5)
    {
        var lastException = new Exception();

        for (int attempt = 0; attempt <= maxRetries; attempt++)
        {
            try
            {
                var response = await CallAsync<T>(method, parameters);
                return response;
            }
            catch (Exception ex)
            {
                lastException = ex;
                Console.WriteLine($"RPC call attempt {attempt + 1} failed: {ex.Message}");

                if (!IsRetryableError(ex) || attempt == maxRetries)
                {
                    break;
                }

                var delay = CalculateBackoffDelay(attempt);
                _logger.LogWarning("RPC call failed (attempt {Attempt}/{MaxRetries}): {Error}. Retrying in {Delay}...", 
                    attempt + 1, maxRetries + 1, ex.Message, delay);

                await Task.Delay(delay);
            }
        }

        throw new InvalidOperationException($"RPC call failed after {maxRetries + 1} attempts", lastException);
    }

    /// <summary>
    /// Makes a single RPC call
    /// </summary>
    private async Task<T> CallAsync<T>(string method, object[]? parameters = null)
    {
        var request = new RpcRequest
        {
            Method = method,
            Params = parameters
        };

        var json = JsonSerializer.Serialize(request, _jsonOptions);
        var content = new StringContent(json, Encoding.UTF8, "application/json");

        var response = await _httpClient.PostAsync("", content);
        var responseContent = await response.Content.ReadAsStringAsync();

        if (!response.IsSuccessStatusCode)
        {
            throw new HttpRequestException($"RPC call failed with status {response.StatusCode}: {responseContent}");
        }

        var rpcResponse = JsonSerializer.Deserialize<RpcResponse>(responseContent, _jsonOptions);
        if (rpcResponse == null)
        {
            throw new InvalidOperationException("Failed to deserialize RPC response");
        }

        if (rpcResponse.Error != null)
        {
            throw new InvalidOperationException($"RPC error: {rpcResponse.Error.Message}");
        }

        if (rpcResponse.Result == null)
        {
            return default(T)!;
        }

        // Handle special case for null results (e.g., spent UTXOs)
        if (rpcResponse.Result is JsonElement element && element.ValueKind == JsonValueKind.Null)
        {
            return default(T)!;
        }

        // Deserialize the result directly from the JsonElement
        if (rpcResponse.Result is JsonElement jsonElement)
        {
            return JsonSerializer.Deserialize<T>(jsonElement, _jsonOptions)!;
        }
        
        // Fallback to string deserialization
        return JsonSerializer.Deserialize<T>(rpcResponse.Result.ToString()!, _jsonOptions)!;
    }

    /// <summary>
    /// Checks if an error is retryable
    /// </summary>
    private static bool IsRetryableError(Exception ex)
    {
        return ex switch
        {
            HttpRequestException httpEx when httpEx.Message.Contains("timeout") => true,
            HttpRequestException httpEx when httpEx.Message.Contains("connection") => true,
            TaskCanceledException => true,
            _ => false
        };
    }

    /// <summary>
    /// Calculates exponential backoff delay with jitter
    /// </summary>
    private static TimeSpan CalculateBackoffDelay(int attempt)
    {
        var baseDelay = TimeSpan.FromMilliseconds(100);
        var delay = TimeSpan.FromMilliseconds(baseDelay.TotalMilliseconds * Math.Pow(2, attempt));
        
        // Add jitter
        var jitter = TimeSpan.FromMilliseconds(delay.TotalMilliseconds * 0.1 * (0.5 + Math.Sin(DateTimeOffset.UtcNow.ToUnixTimeMilliseconds())));
        delay = delay.Add(jitter);

        // Cap at 60 seconds
        var maxDelay = TimeSpan.FromSeconds(60);
        return delay > maxDelay ? maxDelay : delay;
    }

    public async Task<int> GetBlockCountAsync()
    {
        return await CallWithRetryAsync<int>("getblockcount");
    }

    public async Task<string> GetBlockHashAsync(int height)
    {
        return await CallWithRetryAsync<string>("getblockhash", new object[] { height });
    }

    public async Task<BitcoinBlock> GetBlockAsync(string hash)
    {
        return await CallWithRetryAsync<BitcoinBlock>("getblock", new object[] { hash, 2 });
    }

    public async Task<string[]> GetRawMempoolAsync()
    {
        var result = await CallWithRetryAsync<string[]>("getrawmempool", new object[] { false });
        return result ?? Array.Empty<string>();
    }

    public async Task<BitcoinTransaction> GetRawTransactionAsync(string txid)
    {
        return await CallWithRetryAsync<BitcoinTransaction>("getrawtransaction", new object[] { txid, true });
    }

    public async Task<Dictionary<string, object>?> GetTxOutAsync(string txid, int vout)
    {
        var result = await CallWithRetryAsync<Dictionary<string, object>?>("gettxout", new object[] { txid, vout });
        return result;
    }
}
