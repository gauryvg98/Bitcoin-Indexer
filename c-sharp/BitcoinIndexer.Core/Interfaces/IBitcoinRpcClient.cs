using BitcoinIndexer.Core.DTOs;

namespace BitcoinIndexer.Core.Interfaces;

/// <summary>
/// Interface for Bitcoin RPC client operations
/// </summary>
public interface IBitcoinRpcClient
{
    /// <summary>
    /// Gets the current block count
    /// </summary>
    Task<int> GetBlockCountAsync();

    /// <summary>
    /// Gets the block hash for a given height
    /// </summary>
    Task<string> GetBlockHashAsync(int height);

    /// <summary>
    /// Gets block information with transactions
    /// </summary>
    Task<BitcoinBlock> GetBlockAsync(string hash);

    /// <summary>
    /// Gets raw mempool transaction IDs
    /// </summary>
    Task<string[]> GetRawMempoolAsync();

    /// <summary>
    /// Gets raw transaction details
    /// </summary>
    Task<BitcoinTransaction> GetRawTransactionAsync(string txid);

    /// <summary>
    /// Gets unspent transaction output information
    /// </summary>
    Task<Dictionary<string, object>?> GetTxOutAsync(string txid, int vout);

    /// <summary>
    /// Gets the base URL of the RPC client
    /// </summary>
    string BaseUrl { get; }

    /// <summary>
    /// Gets the username of the RPC client
    /// </summary>
    string Username { get; }

    /// <summary>
    /// Gets the password of the RPC client
    /// </summary>
    string Password { get; }
}
