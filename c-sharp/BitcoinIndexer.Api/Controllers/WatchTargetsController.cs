using BitcoinIndexer.Core.Interfaces;
using BitcoinIndexer.Core.Models;
using Microsoft.AspNetCore.Mvc;

namespace BitcoinIndexer.Api.Controllers;

/// <summary>
/// Watch targets management controller
/// </summary>
[ApiController]
[Route("v1/btc/watch")]
public class WatchTargetsController : ControllerBase
{
    private readonly IBitcoinIndexerRepository _repository;
    private readonly ILogger<WatchTargetsController> _logger;

    public WatchTargetsController(
        IBitcoinIndexerRepository repository,
        ILogger<WatchTargetsController> logger)
    {
        _repository = repository;
        _logger = logger;
    }

    /// <summary>
    /// Adds a new watch target (address or xpub)
    /// </summary>
    /// <param name="request">Watch target request</param>
    /// <returns>Created watch target</returns>
    [HttpPost]
    public async Task<ActionResult<WatchTarget>> AddWatchTarget([FromBody] AddWatchTargetRequest request)
    {
        try
        {
            if (string.IsNullOrWhiteSpace(request.Kind))
            {
                return BadRequest("Kind is required");
            }

            if (request.Kind != "address" && request.Kind != "xpub")
            {
                return BadRequest("Kind must be 'address' or 'xpub'");
            }

            if (request.Kind == "address" && string.IsNullOrWhiteSpace(request.Address))
            {
                return BadRequest("Address is required for kind 'address'");
            }

            if (request.Kind == "xpub" && string.IsNullOrWhiteSpace(request.Xpub))
            {
                return BadRequest("Xpub is required for kind 'xpub'");
            }

            var watchTarget = new WatchTarget
            {
                Kind = request.Kind,
                Address = request.Address,
                Xpub = request.Xpub,
                DerivationScheme = request.DerivationScheme,
                Account = request.Account,
                GapLimit = request.GapLimit > 0 ? request.GapLimit : 20,
                CreatedAt = DateTime.UtcNow
            };

            await _repository.AddWatchTargetAsync(watchTarget);
            await _repository.SaveChangesAsync();

            return CreatedAtAction(nameof(GetWatchTargets), new { id = watchTarget.Id }, watchTarget);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error adding watch target");
            return StatusCode(500, "Failed to add watch target");
        }
    }

    /// <summary>
    /// Gets all watch targets
    /// </summary>
    /// <returns>List of watch targets</returns>
    [HttpGet]
    public async Task<ActionResult<IEnumerable<WatchTarget>>> GetWatchTargets()
    {
        try
        {
            var targets = await _repository.GetAllWatchTargetsAsync();
            return Ok(targets);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error getting watch targets");
            return StatusCode(500, "Failed to get watch targets");
        }
    }

    /// <summary>
    /// Deletes a watch target
    /// </summary>
    /// <param name="id">Watch target ID</param>
    /// <returns>No content on success</returns>
    [HttpDelete("{id}")]
    public async Task<ActionResult> DeleteWatchTarget(int id)
    {
        try
        {
            await _repository.DeleteWatchTargetAsync(id);
            await _repository.SaveChangesAsync();

            return NoContent();
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error deleting watch target {Id}", id);
            return StatusCode(500, "Failed to delete watch target");
        }
    }
}

/// <summary>
/// Request model for adding a watch target
/// </summary>
public class AddWatchTargetRequest
{
    /// <summary>
    /// Kind of watch target (address or xpub)
    /// </summary>
    public string Kind { get; set; } = string.Empty;

    /// <summary>
    /// Bitcoin address (for kind 'address')
    /// </summary>
    public string? Address { get; set; }

    /// <summary>
    /// Extended public key (for kind 'xpub')
    /// </summary>
    public string? Xpub { get; set; }

    /// <summary>
    /// Derivation scheme (e.g., 'bip84')
    /// </summary>
    public string? DerivationScheme { get; set; }

    /// <summary>
    /// Account number
    /// </summary>
    public int? Account { get; set; }

    /// <summary>
    /// Gap limit (default: 20)
    /// </summary>
    public int GapLimit { get; set; } = 20;
}
