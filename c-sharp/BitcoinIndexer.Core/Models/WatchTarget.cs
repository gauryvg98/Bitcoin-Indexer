using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace BitcoinIndexer.Core.Models;

/// <summary>
/// Represents addresses or HD wallets to monitor
/// </summary>
public class WatchTarget
{
    [Key]
    [DatabaseGenerated(DatabaseGeneratedOption.Identity)]
    public long Id { get; set; }

    /// <summary>
    /// Type of watch target - "address" or "xpub"
    /// </summary>
    [Required]
    [MaxLength(10)]
    public string Kind { get; set; } = string.Empty;

    /// <summary>
    /// Bitcoin address to watch (for kind="address")
    /// </summary>
    [MaxLength(100)]
    public string? Address { get; set; }

    /// <summary>
    /// Extended public key to watch (for kind="xpub")
    /// </summary>
    [MaxLength(200)]
    public string? Xpub { get; set; }

    /// <summary>
    /// Derivation scheme (e.g., 'bip84')
    /// </summary>
    [MaxLength(20)]
    public string? DerivationScheme { get; set; }

    /// <summary>
    /// Account number for HD wallet derivation
    /// </summary>
    public int? Account { get; set; }

    /// <summary>
    /// Gap limit for address derivation
    /// </summary>
    public int GapLimit { get; set; } = 20;

    /// <summary>
    /// When this watch target was created
    /// </summary>
    public DateTime CreatedAt { get; set; } = DateTime.UtcNow;
}
