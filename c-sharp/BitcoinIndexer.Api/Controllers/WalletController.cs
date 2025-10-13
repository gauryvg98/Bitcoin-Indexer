using BitcoinIndexer.Core.Interfaces;
using BitcoinIndexer.Core.Models;
using Microsoft.AspNetCore.Mvc;

namespace BitcoinIndexer.Api.Controllers;

/// <summary>
/// Wallet connection controller
/// </summary>
[ApiController]
[Route("v1/btc")]
public class WalletController : ControllerBase
{
    private readonly IBitcoinIndexerRepository _repository;
    private readonly ILogger<WalletController> _logger;

    public WalletController(
        IBitcoinIndexerRepository repository,
        ILogger<WalletController> logger)
    {
        _repository = repository;
        _logger = logger;
    }

    /// <summary>
    /// Connects a wallet by adding it to the watchlist
    /// </summary>
    /// <param name="request">Wallet connection request</param>
    /// <returns>Connection status and transaction history</returns>
    [HttpPost("connect")]
    public async Task<ActionResult<object>> ConnectWallet([FromBody] ConnectWalletRequest request)
    {
        try
        {
            if (string.IsNullOrWhiteSpace(request.Address))
            {
                return BadRequest("Address is required");
            }

            // Create watch target
            var watchTarget = new WatchTarget
            {
                Kind = "address",
                Address = request.Address,
                GapLimit = 20,
                CreatedAt = DateTime.UtcNow
            };

            await _repository.AddWatchTargetAsync(watchTarget);
            await _repository.SaveChangesAsync();

            // Get transaction history for the newly connected wallet
            var txHistory = await _repository.GetTransactionReferencesByAddressAsync(request.Address);
            var limitedHistory = txHistory.Take(50);

            var response = new
            {
                status = "connected",
                address = request.Address,
                target_id = watchTarget.Id,
                transaction_history = limitedHistory,
                message = "Wallet connected successfully. Historical data is available immediately."
            };

            return Ok(response);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error connecting wallet for address {Address}", request.Address);
            return StatusCode(500, "Failed to connect wallet");
        }
    }
}

/// <summary>
/// Request model for connecting a wallet
/// </summary>
public class ConnectWalletRequest
{
    /// <summary>
    /// Bitcoin address to connect
    /// </summary>
    public string Address { get; set; } = string.Empty;

    /// <summary>
    /// Optional label for the wallet
    /// </summary>
    public string? Label { get; set; }
}
