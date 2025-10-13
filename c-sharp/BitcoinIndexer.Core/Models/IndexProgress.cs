using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace BitcoinIndexer.Core.Models;

/// <summary>
/// Tracks indexing state
/// </summary>
public class IndexProgress
{
    /// <summary>
    /// Primary key (always true for singleton)
    /// </summary>
    [Key]
    public bool Id { get; set; } = true;

    /// <summary>
    /// Last processed block height
    /// </summary>
    public int LastHeight { get; set; }

    /// <summary>
    /// Hash of the last processed block
    /// </summary>
    [MaxLength(64)]
    public string LastBlockHash { get; set; } = string.Empty;

    /// <summary>
    /// When the progress was last updated
    /// </summary>
    public DateTime UpdatedAt { get; set; } = DateTime.UtcNow;
}
