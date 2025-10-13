using BitcoinIndexer.Core.Interfaces;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

namespace BitcoinIndexer.Application.Services;

/// <summary>
/// Hosted service that manages the Bitcoin indexer lifecycle
/// </summary>
public class BitcoinIndexerHostedService : BackgroundService
{
    private readonly IServiceProvider _serviceProvider;
    private readonly ILogger<BitcoinIndexerHostedService> _logger;

    public BitcoinIndexerHostedService(
        IServiceProvider serviceProvider,
        ILogger<BitcoinIndexerHostedService> logger)
    {
        _serviceProvider = serviceProvider;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        _logger.LogInformation("Bitcoin Indexer Hosted Service starting...");

        try
        {
            // Wait a bit for the application to fully start
            await Task.Delay(2000, stoppingToken);

            using var scope = _serviceProvider.CreateScope();
            var indexerService = scope.ServiceProvider.GetRequiredService<IBitcoinIndexerService>();

            _logger.LogInformation("Starting Bitcoin indexer...");
            await indexerService.StartAsync(stoppingToken);

            // Keep the service running
            await Task.Delay(Timeout.Infinite, stoppingToken);
        }
        catch (OperationCanceledException)
        {
            _logger.LogInformation("Bitcoin Indexer Hosted Service is stopping...");
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error in Bitcoin Indexer Hosted Service");
        }
    }

    public override async Task StopAsync(CancellationToken cancellationToken)
    {
        _logger.LogInformation("Bitcoin Indexer Hosted Service stopping...");

        try
        {
            using var scope = _serviceProvider.CreateScope();
            var indexerService = scope.ServiceProvider.GetRequiredService<IBitcoinIndexerService>();
            await indexerService.StopAsync(cancellationToken);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error stopping Bitcoin indexer");
        }

        await base.StopAsync(cancellationToken);
    }
}
