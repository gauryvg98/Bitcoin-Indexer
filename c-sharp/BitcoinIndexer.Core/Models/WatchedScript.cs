using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace BitcoinIndexer.Core.Models;

/// <summary>
/// Represents derived scripts we monitor
/// </summary>
public class WatchedScript
{
    [Key]
    [DatabaseGenerated(DatabaseGeneratedOption.Identity)]
    public long Id { get; set; }

    /// <summary>
    /// Bitcoin address associated with this script
    /// </summary>
    [Required]
    [MaxLength(100)]
    public string Address { get; set; } = string.Empty;

    /// <summary>
    /// Script hex that we're monitoring
    /// </summary>
    [Required]
    [MaxLength(1000)]
    public string ScriptHex { get; set; } = string.Empty;

    /// <summary>
    /// Script type (p2wpkh, p2tr, etc.)
    /// </summary>
    [MaxLength(20)]
    public string? Type { get; set; }

    /// <summary>
    /// Foreign key to the watch target
    /// </summary>
    [Required]
    public long TargetId { get; set; }

    /// <summary>
    /// Navigation property to the watch target
    /// </summary>
    [ForeignKey(nameof(TargetId))]
    public WatchTarget Target { get; set; } = null!;
}
