using BitcoinIndexer.Core.Interfaces;
using BitcoinIndexer.Core.Models;
using Microsoft.AspNetCore.Mvc;

namespace BitcoinIndexer.Api.Controllers;

/// <summary>
/// Bitcoin indexer API controller
/// </summary>
[ApiController]
[Route("v1/btc")]
public class BitcoinController : ControllerBase
{
    private readonly IBitcoinIndexerRepository _repository;
    private readonly IBitcoinIndexerService _indexerService;
    private readonly ILogger<BitcoinController> _logger;

    public BitcoinController(
        IBitcoinIndexerRepository repository,
        IBitcoinIndexerService indexerService,
        ILogger<BitcoinController> logger)
    {
        _repository = repository;
        _indexerService = indexerService;
        _logger = logger;
    }

    /// <summary>
    /// Gets the balance for a specific address
    /// </summary>
    /// <param name="address">Bitcoin address</param>
    /// <param name="confirmations">Number of confirmations required (default: 3)</param>
    /// <returns>Balance information</returns>
    [HttpGet("balance/{address}")]
    public async Task<ActionResult<Balance>> GetBalance(string address, int confirmations = 3)
    {
        try
        {
            if (string.IsNullOrWhiteSpace(address))
            {
                return BadRequest("Address parameter is required");
            }

            var utxos = await _repository.GetUtxosByAddressAsync(address);
            var confirmedUtxos = new List<Utxo>();
            foreach (var utxo in utxos.Where(u => u.Status == "confirmed"))
            {
                if (utxo.BlockHeight == null || await GetConfirmationsAsync(utxo.BlockHeight.Value) >= confirmations)
                {
                    confirmedUtxos.Add(utxo);
                }
            }

            var balance = new Balance
            {
                Address = address,
                ConfirmedSats = confirmedUtxos.Sum(u => u.ValueSats),
                UnconfirmedSats = utxos.Where(u => u.Status == "mempool").Sum(u => u.ValueSats),
                TotalSats = utxos.Sum(u => u.ValueSats),
                UtxoCount = utxos.Count(),
                ConfirmedUtxoCount = confirmedUtxos.Count()
            };

            return Ok(balance);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error getting balance for address {Address}", address);
            return StatusCode(500, "Failed to get balance");
        }
    }

    /// <summary>
    /// Gets UTXOs for a specific address
    /// </summary>
    /// <param name="address">Bitcoin address</param>
    /// <returns>List of UTXOs</returns>
    [HttpGet("utxos/{address}")]
    public async Task<ActionResult<IEnumerable<UtxoInfo>>> GetUtxos(string address)
    {
        try
        {
            if (string.IsNullOrWhiteSpace(address))
            {
                return BadRequest("Address parameter is required");
            }

            var utxos = await _repository.GetUtxosByAddressAsync(address);
            var utxoInfos = new List<UtxoInfo>();

            foreach (var utxo in utxos)
            {
                var utxoInfo = new UtxoInfo
                {
                    Txid = utxo.Txid,
                    Vout = utxo.Vout,
                    ValueSats = utxo.ValueSats,
                    Status = utxo.Status,
                    BlockHeight = utxo.BlockHeight,
                    BlockHash = utxo.BlockHash,
                    FirstSeenAt = utxo.FirstSeenAt.ToString("yyyy-MM-ddTHH:mm:ssZ"),
                    SpentAt = utxo.SpentAt?.ToString("yyyy-MM-ddTHH:mm:ssZ")
                };

                // Calculate confirmations
                if (utxo.BlockHeight.HasValue)
                {
                    utxoInfo.Confirmations = await GetConfirmationsAsync(utxo.BlockHeight.Value);
                }

                utxoInfos.Add(utxoInfo);
            }

            return Ok(utxoInfos);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error getting UTXOs for address {Address}", address);
            return StatusCode(500, "Failed to get UTXOs");
        }
    }

    /// <summary>
    /// Gets transaction information
    /// </summary>
    /// <param name="txid">Transaction ID</param>
    /// <returns>Transaction details</returns>
    [HttpGet("tx/{txid}")]
    public async Task<ActionResult<Transaction>> GetTransaction(string txid)
    {
        try
        {
            if (string.IsNullOrWhiteSpace(txid))
            {
                return BadRequest("Transaction ID parameter is required");
            }

            var transaction = await _repository.GetTransactionAsync(txid);
            if (transaction == null)
            {
                return NotFound("Transaction not found");
            }

            return Ok(transaction);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error getting transaction {Txid}", txid);
            return StatusCode(500, "Failed to get transaction");
        }
    }

    /// <summary>
    /// Gets transaction history for a specific address
    /// </summary>
    /// <param name="address">Bitcoin address</param>
    /// <param name="limit">Maximum number of transactions to return (default: 50)</param>
    /// <returns>Transaction history</returns>
    [HttpGet("history/{address}")]
    public async Task<ActionResult<IEnumerable<TransactionReference>>> GetTransactionHistory(string address, int limit = 50)
    {
        try
        {
            if (string.IsNullOrWhiteSpace(address))
            {
                return BadRequest("Address parameter is required");
            }

            var history = await _repository.GetTransactionReferencesByAddressAsync(address);
            var limitedHistory = history.Take(limit);

            return Ok(limitedHistory);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error getting transaction history for address {Address}", address);
            return StatusCode(500, "Failed to get transaction history");
        }
    }

    /// <summary>
    /// Gets the current indexing status
    /// </summary>
    /// <returns>Indexing status</returns>
    [HttpGet("status")]
    public async Task<ActionResult<object>> GetIndexStatus()
    {
        try
        {
            var progress = await _repository.GetIndexProgressAsync();
            var status = await _indexerService.GetStatusAsync();

            var result = new
            {
                last_height = progress?.LastHeight ?? 0,
                last_block_hash = progress?.LastBlockHash,
                updated_at = progress?.UpdatedAt,
                is_running = status.IsRunning,
                total_processed_blocks = status.TotalProcessedBlocks,
                total_processed_transactions = status.TotalProcessedTransactions,
                error_message = status.ErrorMessage
            };

            return Ok(result);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error getting index status");
            return StatusCode(500, "Failed to get index status");
        }
    }

    /// <summary>
    /// Manually start the indexer
    /// </summary>
    [HttpPost("start")]
    public async Task<IActionResult> StartIndexer()
    {
        try
        {
            await _indexerService.StartAsync(CancellationToken.None);
            return Ok(new { message = "Indexer started successfully" });
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error starting indexer");
            return StatusCode(500, "Failed to start indexer");
        }
    }

    /// <summary>
    /// Manually process blocks from a specific height range
    /// </summary>
    [HttpPost("process-blocks")]
    public async Task<IActionResult> ProcessBlocks([FromQuery] int startHeight = 0, [FromQuery] int endHeight = 10)
    {
        try
        {
            await _indexerService.ProcessBlocksAsync(startHeight, endHeight, CancellationToken.None);
            return Ok(new { message = $"Processed blocks from {startHeight} to {endHeight}" });
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error processing blocks");
            return StatusCode(500, "Failed to process blocks");
        }
    }

    /// <summary>
    /// Test Bitcoin RPC connection
    /// </summary>
    [HttpGet("test-rpc")]
    public async Task<IActionResult> TestRpc()
    {
        try
        {
            var blockCount = await _indexerService.GetBlockCountAsync();
            var blockHash = await _indexerService.GetBlockHashAsync(918825);
            var block = await _indexerService.GetBlockAsync(blockHash);
            
            return Ok(new { 
                blockCount = blockCount,
                blockHash = blockHash,
                blockHeight = block.Height,
                blockTxCount = block.Transactions.Length,
                blockHashFromBlock = block.Hash
            });
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error testing RPC");
            return StatusCode(500, $"RPC test failed: {ex.Message}");
        }
    }

    private async Task<int> GetConfirmationsAsync(int blockHeight)
    {
        try
        {
            var progress = await _repository.GetIndexProgressAsync();
            if (progress?.LastHeight == null)
            {
                return 0;
            }

            return progress.LastHeight - blockHeight + 1;
        }
        catch
        {
            return 0;
        }
    }
}
