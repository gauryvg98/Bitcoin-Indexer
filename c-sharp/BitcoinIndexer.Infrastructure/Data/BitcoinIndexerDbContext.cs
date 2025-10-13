using BitcoinIndexer.Core.Models;
using Microsoft.EntityFrameworkCore;

namespace BitcoinIndexer.Infrastructure.Data;

/// <summary>
/// Entity Framework DbContext for the Bitcoin indexer
/// </summary>
public class BitcoinIndexerDbContext : DbContext
{
    public BitcoinIndexerDbContext(DbContextOptions<BitcoinIndexerDbContext> options) : base(options)
    {
    }

    // DbSets for all entities
    public DbSet<WatchTarget> WatchTargets { get; set; } = null!;
    public DbSet<WatchedScript> WatchedScripts { get; set; } = null!;
    public DbSet<Utxo> Utxos { get; set; } = null!;
    public DbSet<IndexProgress> IndexProgress { get; set; } = null!;
    public DbSet<Transaction> Transactions { get; set; } = null!;
    public DbSet<TransactionInput> TransactionInputs { get; set; } = null!;
    public DbSet<TransactionOutput> TransactionOutputs { get; set; } = null!;
    public DbSet<BlockInfo> BlockInfos { get; set; } = null!;
    public DbSet<AddressMetadata> AddressMetadata { get; set; } = null!;
    public DbSet<TransactionReference> TransactionReferences { get; set; } = null!;

    protected override void OnModelCreating(ModelBuilder modelBuilder)
    {
        base.OnModelCreating(modelBuilder);

        // Configure all entities to use snake_case column names to match Go schema
        foreach (var entity in modelBuilder.Model.GetEntityTypes())
        {
            foreach (var property in entity.GetProperties())
            {
                property.SetColumnName(ToSnakeCase(property.GetColumnName()));
            }
        }

        // Configure WatchTarget (table: watch_target)
        modelBuilder.Entity<WatchTarget>(entity =>
        {
            entity.ToTable("watch_target");
            entity.HasKey(e => e.Id);
            entity.Property(e => e.Kind).IsRequired().HasMaxLength(20);
            entity.Property(e => e.Address).HasMaxLength(100);
            entity.Property(e => e.Xpub).HasMaxLength(200);
            entity.Property(e => e.DerivationScheme).HasMaxLength(20);
            entity.HasIndex(e => e.Address).IsUnique().HasFilter("\"address\" IS NOT NULL");
        });

        // Configure WatchedScript (table: watched_script)
        modelBuilder.Entity<WatchedScript>(entity =>
        {
            entity.ToTable("watched_script");
            entity.HasKey(e => e.Id);
            entity.Property(e => e.Address).IsRequired().HasMaxLength(100);
            entity.Property(e => e.ScriptHex).IsRequired().HasMaxLength(10000);
            entity.Property(e => e.Type).HasMaxLength(20);
            entity.HasIndex(e => e.ScriptHex).IsUnique();
            entity.HasIndex(e => e.Address);
            entity.HasIndex(e => e.Type);
            entity.HasIndex(e => e.TargetId);
        });

        // Configure Utxo (table: utxo)
        modelBuilder.Entity<Utxo>(entity =>
        {
            entity.ToTable("utxo");
            entity.HasKey(e => new { e.Txid, e.Vout });
            entity.Property(e => e.Txid).IsRequired().HasMaxLength(64);
            entity.Property(e => e.Address).IsRequired().HasMaxLength(100);
            entity.Property(e => e.ScriptHex).IsRequired().HasMaxLength(10000);
            entity.Property(e => e.Status).IsRequired().HasMaxLength(50);
            entity.Property(e => e.BlockHash).HasMaxLength(64);
            entity.HasIndex(e => e.Address);
            entity.HasIndex(e => e.BlockHash);
            entity.HasIndex(e => e.BlockHeight);
            entity.HasIndex(e => new { e.Address, e.Status, e.BlockHeight });
        });

        // Configure IndexProgress (table: index_progress)
        modelBuilder.Entity<IndexProgress>(entity =>
        {
            entity.ToTable("index_progress");
            entity.HasKey(e => e.Id);
            entity.Property(e => e.LastBlockHash).HasMaxLength(64);
        });

        // Configure Transaction (table: transaction)
        modelBuilder.Entity<Transaction>(entity =>
        {
            entity.ToTable("transaction");
            entity.HasKey(e => e.Txid);
            entity.Property(e => e.Txid).IsRequired().HasMaxLength(64);
            entity.Property(e => e.BlockHash).HasMaxLength(64);
            entity.HasIndex(e => e.BlockHeight);
            entity.HasIndex(e => e.BlockHash);
            entity.HasIndex(e => e.BlockTime);
            entity.HasIndex(e => e.IsCoinbase);
            
            // Ignore navigation properties to prevent foreign key creation
            entity.Ignore(e => e.Inputs);
            entity.Ignore(e => e.Outputs);
        });

        // Configure TransactionInput (table: transaction_input)
        modelBuilder.Entity<TransactionInput>(entity =>
        {
            entity.ToTable("transaction_input");
            entity.HasKey(e => new { e.Txid, e.Vout });
            entity.Property(e => e.Txid).IsRequired().HasMaxLength(64);
            entity.Property(e => e.PrevTxid).HasMaxLength(64);
            entity.Property(e => e.ScriptSig).HasMaxLength(10000);
            entity.Property(e => e.Witness).HasColumnType("text[]");
            entity.HasIndex(e => e.Txid);
            entity.HasIndex(e => new { e.PrevTxid, e.PrevVout });
        });

        // Configure TransactionOutput (table: transaction_output)
        modelBuilder.Entity<TransactionOutput>(entity =>
        {
            entity.ToTable("transaction_output");
            entity.HasKey(e => new { e.Txid, e.Vout });
            entity.Property(e => e.Txid).IsRequired().HasMaxLength(64);
            entity.Property(e => e.Address).IsRequired().HasMaxLength(100);
            entity.Property(e => e.ScriptType).HasMaxLength(50);
            entity.Property(e => e.ScriptHex).HasMaxLength(10000);
            entity.Property(e => e.ScriptAsm).HasMaxLength(10000);
            entity.HasIndex(e => e.Txid);
            entity.HasIndex(e => e.Address);
        });

        // Configure BlockInfo (table: block_info) - matches Go schema
        modelBuilder.Entity<BlockInfo>(entity =>
        {
            entity.ToTable("block_info");
            entity.HasKey(e => e.Height); // Go uses height as primary key
            entity.Property(e => e.Hash).IsRequired().HasMaxLength(64);
            entity.Property(e => e.PreviousHash).HasMaxLength(64);
            entity.HasIndex(e => e.Height);
            entity.HasIndex(e => e.Hash);
            entity.HasIndex(e => e.Timestamp);
        });

        // Configure AddressMetadata (table: address_metadata)
        modelBuilder.Entity<AddressMetadata>(entity =>
        {
            entity.ToTable("address_metadata");
            entity.HasKey(e => e.Address);
            entity.Property(e => e.Address).IsRequired().HasMaxLength(100);
            entity.HasIndex(e => e.IsWatched);
            entity.HasIndex(e => e.LastSeenAt);
        });

        // Configure TransactionReference (table: tx_reference)
        modelBuilder.Entity<TransactionReference>(entity =>
        {
            entity.ToTable("tx_reference");
            entity.HasKey(e => e.Id);
            entity.Property(e => e.Txid).IsRequired().HasMaxLength(64);
            entity.Property(e => e.Address).IsRequired().HasMaxLength(100);
            entity.Property(e => e.Direction).IsRequired().HasMaxLength(10);
            entity.Property(e => e.SenderAddress).HasMaxLength(100);
            entity.Property(e => e.ReceiverAddress).HasMaxLength(100);
            entity.HasIndex(e => e.Txid);
            entity.HasIndex(e => e.Address);
            entity.HasIndex(e => e.BlockHeight);
            entity.HasIndex(e => e.CreatedAt);
            entity.HasIndex(e => new { e.Txid, e.Address, e.Direction }).IsUnique();
        });
    }

    /// <summary>
    /// Converts PascalCase to snake_case to match Go schema
    /// </summary>
    private static string ToSnakeCase(string input)
    {
        if (string.IsNullOrEmpty(input))
            return input;

        var result = new System.Text.StringBuilder();
        result.Append(char.ToLower(input[0]));

        for (int i = 1; i < input.Length; i++)
        {
            if (char.IsUpper(input[i]))
            {
                result.Append('_');
                result.Append(char.ToLower(input[i]));
            }
            else
            {
                result.Append(input[i]);
            }
        }

        return result.ToString();
    }
}
