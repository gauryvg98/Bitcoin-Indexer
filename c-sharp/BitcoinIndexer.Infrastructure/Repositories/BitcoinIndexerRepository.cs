using BitcoinIndexer.Core.Interfaces;
using BitcoinIndexer.Core.Models;
using BitcoinIndexer.Infrastructure.Data;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Storage;
using Npgsql;

namespace BitcoinIndexer.Infrastructure.Repositories;

/// <summary>
/// Repository implementation for Bitcoin indexer specific operations
/// </summary>
public class BitcoinIndexerRepository : IBitcoinIndexerRepository
{
    private readonly BitcoinIndexerDbContext _context;
    private IDbContextTransaction? _transaction;

    public BitcoinIndexerRepository(BitcoinIndexerDbContext context)
    {
        _context = context;
    }

    // WatchTarget operations
    public async Task<WatchTarget?> GetWatchTargetByAddressAsync(string address)
    {
        return await _context.WatchTargets
            .FirstOrDefaultAsync(wt => wt.Address == address);
    }

    public async Task<WatchTarget?> GetWatchTargetByXpubAsync(string xpub)
    {
        return await _context.WatchTargets
            .FirstOrDefaultAsync(wt => wt.Xpub == xpub);
    }

    public async Task<IEnumerable<WatchTarget>> GetAllWatchTargetsAsync()
    {
        return await _context.WatchTargets.ToListAsync();
    }

    public async Task<WatchTarget> AddWatchTargetAsync(WatchTarget watchTarget)
    {
        await _context.WatchTargets.AddAsync(watchTarget);
        return watchTarget;
    }

    public Task UpdateWatchTargetAsync(WatchTarget watchTarget)
    {
        _context.WatchTargets.Update(watchTarget);
        return Task.CompletedTask;
    }

    public async Task DeleteWatchTargetAsync(int id)
    {
        var watchTarget = await _context.WatchTargets.FindAsync(id);
        if (watchTarget != null)
        {
            _context.WatchTargets.Remove(watchTarget);
        }
    }

    // WatchedScript operations
    public async Task<WatchedScript?> GetWatchedScriptByScriptHexAsync(string scriptHex)
    {
        return await _context.WatchedScripts
            .FirstOrDefaultAsync(ws => ws.ScriptHex == scriptHex);
    }

    public async Task<IEnumerable<WatchedScript>> GetWatchedScriptsByAddressAsync(string address)
    {
        return await _context.WatchedScripts
            .Where(ws => ws.Address == address)
            .ToListAsync();
    }

    public async Task<IEnumerable<WatchedScript>> GetAllWatchedScriptsAsync()
    {
        return await _context.WatchedScripts.ToListAsync();
    }

    public async Task<WatchedScript> AddWatchedScriptAsync(WatchedScript watchedScript)
    {
        await _context.WatchedScripts.AddAsync(watchedScript);
        return watchedScript;
    }

    public async Task AddWatchedScriptsAsync(IEnumerable<WatchedScript> watchedScripts)
    {
        await _context.WatchedScripts.AddRangeAsync(watchedScripts);
    }

    // UTXO operations
    public async Task<Utxo?> GetUtxoAsync(string txid, int vout)
    {
        return await _context.Utxos
            .FirstOrDefaultAsync(u => u.Txid == txid && u.Vout == vout);
    }

    public async Task<IEnumerable<Utxo>> GetUtxosByAddressAsync(string address)
    {
        return await _context.Utxos
            .Where(u => u.Address == address)
            .ToListAsync();
    }

    public async Task<IEnumerable<Utxo>> GetUtxosByAddressAndStatusAsync(string address, string status)
    {
        return await _context.Utxos
            .Where(u => u.Address == address && u.Status == status)
            .ToListAsync();
    }

    public async Task<Utxo> UpsertUtxoAsync(Utxo utxo)
    {
        var existing = await GetUtxoAsync(utxo.Txid, utxo.Vout);
        if (existing != null)
        {
            existing.Status = utxo.Status;
            existing.BlockHeight = utxo.BlockHeight;
            existing.BlockHash = utxo.BlockHash;
            existing.SpentAt = utxo.SpentAt;
            _context.Utxos.Update(existing);
            return existing;
        }
        else
        {
            await _context.Utxos.AddAsync(utxo);
            return utxo;
        }
    }

    public async Task UpsertUtxosAsync(IEnumerable<Utxo> utxos)
    {
        foreach (var utxo in utxos)
        {
            await UpsertUtxoAsync(utxo);
        }
    }

    public async Task UpdateUtxoStatusAsync(string txid, int vout, string status, DateTime? spentAt = null, string? spentByTxid = null)
    {
        var utxo = await GetUtxoAsync(txid, vout);
        if (utxo != null)
        {
            utxo.Status = status;
            utxo.SpentAt = spentAt;
            utxo.SpentByTxid = spentByTxid;
            _context.Utxos.Update(utxo);
        }
    }

    public async Task DeleteUtxoAsync(string txid, int vout)
    {
        var utxo = await GetUtxoAsync(txid, vout);
        if (utxo != null)
        {
            _context.Utxos.Remove(utxo);
        }
    }

    // IndexProgress operations
    public async Task<IndexProgress?> GetIndexProgressAsync()
    {
        return await _context.IndexProgress.AsNoTracking().FirstOrDefaultAsync();
    }

    public async Task<IndexProgress> UpsertIndexProgressAsync(IndexProgress indexProgress)
    {
        // Use raw SQL to avoid tracking issues
        var sql = @"
            INSERT INTO index_progress (id, last_height, last_block_hash, updated_at)
            VALUES (true, @lastHeight, @lastBlockHash, @updatedAt)
            ON CONFLICT (id) DO UPDATE SET
                last_height = @lastHeight,
                last_block_hash = @lastBlockHash,
                updated_at = @updatedAt";
        
        var parameters = new[]
        {
            new Npgsql.NpgsqlParameter("@lastHeight", indexProgress.LastHeight),
            new Npgsql.NpgsqlParameter("@lastBlockHash", indexProgress.LastBlockHash ?? (object)DBNull.Value),
            new Npgsql.NpgsqlParameter("@updatedAt", DateTime.UtcNow)
        };
        
        await _context.Database.ExecuteSqlRawAsync(sql, parameters);
        
        // Return the updated progress
        return new IndexProgress
        {
            Id = true,
            LastHeight = indexProgress.LastHeight,
            LastBlockHash = indexProgress.LastBlockHash,
            UpdatedAt = DateTime.UtcNow
        };
    }

    // Transaction operations
    public async Task<Transaction?> GetTransactionAsync(string txid)
    {
        return await _context.Transactions
            .Include(t => t.Inputs)
            .Include(t => t.Outputs)
            .FirstOrDefaultAsync(t => t.Txid == txid);
    }

    public async Task<IEnumerable<Transaction>> GetTransactionsByBlockHeightAsync(int blockHeight)
    {
        return await _context.Transactions
            .Where(t => t.BlockHeight == blockHeight)
            .ToListAsync();
    }

    public async Task<Transaction> AddTransactionAsync(Transaction transaction)
    {
        await _context.Transactions.AddAsync(transaction);
        return transaction;
    }

    public async Task AddTransactionsAsync(IEnumerable<Transaction> transactions)
    {
        await _context.Transactions.AddRangeAsync(transactions);
    }

    // TransactionInput operations
    public async Task<IEnumerable<TransactionInput>> GetTransactionInputsAsync(string txid)
    {
        return await _context.TransactionInputs
            .Where(ti => ti.Txid == txid)
            .ToListAsync();
    }

    public async Task<TransactionInput> AddTransactionInputAsync(TransactionInput input)
    {
        await _context.TransactionInputs.AddAsync(input);
        return input;
    }

    public async Task AddTransactionInputsAsync(IEnumerable<TransactionInput> inputs)
    {
        await _context.TransactionInputs.AddRangeAsync(inputs);
    }

    // TransactionOutput operations
    public async Task<IEnumerable<TransactionOutput>> GetTransactionOutputsAsync(string txid)
    {
        return await _context.TransactionOutputs
            .Where(to => to.Txid == txid)
            .ToListAsync();
    }

    public async Task<TransactionOutput?> GetTransactionOutputAsync(string txid, int vout)
    {
        return await _context.TransactionOutputs
            .FirstOrDefaultAsync(to => to.Txid == txid && to.Vout == vout);
    }

    public async Task<TransactionOutput> AddTransactionOutputAsync(TransactionOutput output)
    {
        await _context.TransactionOutputs.AddAsync(output);
        return output;
    }

    public async Task AddTransactionOutputsAsync(IEnumerable<TransactionOutput> outputs)
    {
        await _context.TransactionOutputs.AddRangeAsync(outputs);
    }

    public async Task UpdateTransactionOutputSpentAsync(string txid, int vout, string spentByTxid, int spentByInputIndex, int spentAtHeight)
    {
        var output = await GetTransactionOutputAsync(txid, vout);
        if (output != null)
        {
            // Note: TransactionOutput in Go schema doesn't track spent status
            // This is handled by the UTXO table instead
            _context.TransactionOutputs.Update(output);
        }
    }

    // BlockInfo operations
    public async Task<BlockInfo?> GetBlockInfoByHashAsync(string hash)
    {
        return await _context.BlockInfos
            .FirstOrDefaultAsync(b => b.Hash == hash);
    }

    public async Task<BlockInfo?> GetBlockInfoByHeightAsync(int height)
    {
        return await _context.BlockInfos
            .FirstOrDefaultAsync(b => b.Height == height);
    }

    public async Task<BlockInfo> AddBlockInfoAsync(BlockInfo blockInfo)
    {
        await _context.BlockInfos.AddAsync(blockInfo);
        return blockInfo;
    }

    public async Task AddBlockInfosAsync(IEnumerable<BlockInfo> blockInfos)
    {
        await _context.BlockInfos.AddRangeAsync(blockInfos);
    }

    // AddressMetadata operations
    public async Task<AddressMetadata?> GetAddressMetadataAsync(string address)
    {
        return await _context.AddressMetadata
            .FirstOrDefaultAsync(am => am.Address == address);
    }

    public async Task<AddressMetadata> UpsertAddressMetadataAsync(AddressMetadata metadata)
    {
        var existing = await GetAddressMetadataAsync(metadata.Address);
        if (existing != null)
        {
            existing.FirstSeenAt = metadata.FirstSeenAt;
            existing.LastSeenAt = metadata.LastSeenAt;
            existing.TotalReceivedSats = metadata.TotalReceivedSats;
            existing.TotalSentSats = metadata.TotalSentSats;
            existing.TransactionCount = metadata.TransactionCount;
            existing.IsWatched = metadata.IsWatched;
            existing.UpdatedAt = DateTime.UtcNow;
            _context.AddressMetadata.Update(existing);
            return existing;
        }
        else
        {
            await _context.AddressMetadata.AddAsync(metadata);
            return metadata;
        }
    }

    // TransactionReference operations
    public async Task<IEnumerable<TransactionReference>> GetTransactionReferencesByAddressAsync(string address)
    {
        return await _context.TransactionReferences
            .Where(tr => tr.Address == address)
            .OrderByDescending(tr => tr.CreatedAt)
            .ToListAsync();
    }

    public async Task<TransactionReference> AddTransactionReferenceAsync(TransactionReference reference)
    {
        await _context.TransactionReferences.AddAsync(reference);
        return reference;
    }

    public async Task AddTransactionReferencesAsync(IEnumerable<TransactionReference> references)
    {
        await _context.TransactionReferences.AddRangeAsync(references);
    }

    // Bulk operations
    public async Task<int> SaveChangesAsync()
    {
        return await _context.SaveChangesAsync();
    }

    public async Task BeginTransactionAsync()
    {
        _transaction = await _context.Database.BeginTransactionAsync();
    }

    public async Task CommitTransactionAsync()
    {
        if (_transaction != null)
        {
            await _transaction.CommitAsync();
            await _transaction.DisposeAsync();
            _transaction = null;
        }
    }

    public async Task RollbackTransactionAsync()
    {
        if (_transaction != null)
        {
            await _transaction.RollbackAsync();
            await _transaction.DisposeAsync();
            _transaction = null;
        }
    }
}
