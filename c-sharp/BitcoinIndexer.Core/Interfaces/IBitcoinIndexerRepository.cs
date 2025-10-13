using BitcoinIndexer.Core.Models;

namespace BitcoinIndexer.Core.Interfaces;

/// <summary>
/// Repository interface for Bitcoin indexer specific operations
/// </summary>
public interface IBitcoinIndexerRepository
{
    // WatchTarget operations
    Task<WatchTarget?> GetWatchTargetByAddressAsync(string address);
    Task<WatchTarget?> GetWatchTargetByXpubAsync(string xpub);
    Task<IEnumerable<WatchTarget>> GetAllWatchTargetsAsync();
    Task<WatchTarget> AddWatchTargetAsync(WatchTarget watchTarget);
    Task UpdateWatchTargetAsync(WatchTarget watchTarget);
    Task DeleteWatchTargetAsync(int id);

    // WatchedScript operations
    Task<WatchedScript?> GetWatchedScriptByScriptHexAsync(string scriptHex);
    Task<IEnumerable<WatchedScript>> GetWatchedScriptsByAddressAsync(string address);
    Task<IEnumerable<WatchedScript>> GetAllWatchedScriptsAsync();
    Task<WatchedScript> AddWatchedScriptAsync(WatchedScript watchedScript);
    Task AddWatchedScriptsAsync(IEnumerable<WatchedScript> watchedScripts);

    // UTXO operations
    Task<Utxo?> GetUtxoAsync(string txid, int vout);
    Task<IEnumerable<Utxo>> GetUtxosByAddressAsync(string address);
    Task<IEnumerable<Utxo>> GetUtxosByAddressAndStatusAsync(string address, string status);
    Task<Utxo> UpsertUtxoAsync(Utxo utxo);
    Task UpsertUtxosAsync(IEnumerable<Utxo> utxos);
    Task UpdateUtxoStatusAsync(string txid, int vout, string status, DateTime? spentAt = null, string? spentByTxid = null);
    Task DeleteUtxoAsync(string txid, int vout);

    // IndexProgress operations
    Task<IndexProgress?> GetIndexProgressAsync();
    Task<IndexProgress> UpsertIndexProgressAsync(IndexProgress indexProgress);

    // Transaction operations
    Task<Transaction?> GetTransactionAsync(string txid);
    Task<IEnumerable<Transaction>> GetTransactionsByBlockHeightAsync(int blockHeight);
    Task<Transaction> AddTransactionAsync(Transaction transaction);
    Task AddTransactionsAsync(IEnumerable<Transaction> transactions);

    // TransactionInput operations
    Task<IEnumerable<TransactionInput>> GetTransactionInputsAsync(string txid);
    Task<TransactionInput> AddTransactionInputAsync(TransactionInput input);
    Task AddTransactionInputsAsync(IEnumerable<TransactionInput> inputs);

    // TransactionOutput operations
    Task<IEnumerable<TransactionOutput>> GetTransactionOutputsAsync(string txid);
    Task<TransactionOutput?> GetTransactionOutputAsync(string txid, int vout);
    Task<TransactionOutput> AddTransactionOutputAsync(TransactionOutput output);
    Task AddTransactionOutputsAsync(IEnumerable<TransactionOutput> outputs);
    Task UpdateTransactionOutputSpentAsync(string txid, int vout, string spentByTxid, int spentByInputIndex, int spentAtHeight);

    // BlockInfo operations
    Task<BlockInfo?> GetBlockInfoByHashAsync(string hash);
    Task<BlockInfo?> GetBlockInfoByHeightAsync(int height);
    Task<BlockInfo> AddBlockInfoAsync(BlockInfo blockInfo);
    Task AddBlockInfosAsync(IEnumerable<BlockInfo> blockInfos);

    // AddressMetadata operations
    Task<AddressMetadata?> GetAddressMetadataAsync(string address);
    Task<AddressMetadata> UpsertAddressMetadataAsync(AddressMetadata metadata);

    // TransactionReference operations
    Task<IEnumerable<TransactionReference>> GetTransactionReferencesByAddressAsync(string address);
    Task<TransactionReference> AddTransactionReferenceAsync(TransactionReference reference);
    Task AddTransactionReferencesAsync(IEnumerable<TransactionReference> references);

    // Bulk operations
    Task<int> SaveChangesAsync();
    Task BeginTransactionAsync();
    Task CommitTransactionAsync();
    Task RollbackTransactionAsync();
}
