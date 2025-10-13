using BitcoinIndexer.Core.Configuration;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.Options;

namespace BitcoinIndexer.Infrastructure.Configuration;

/// <summary>
/// Service for loading and validating configuration
/// </summary>
public class ConfigurationService
{
    private readonly IConfiguration _configuration;
    private readonly IOptions<IndexerConfiguration> _options;

    public ConfigurationService(IConfiguration configuration, IOptions<IndexerConfiguration> options)
    {
        _configuration = configuration;
        _options = options;
    }

    /// <summary>
    /// Gets the validated configuration
    /// </summary>
    public IndexerConfiguration GetConfiguration()
    {
        var config = _options.Value;
        ValidateConfiguration(config);
        return config;
    }

    /// <summary>
    /// Validates the configuration
    /// </summary>
    private static void ValidateConfiguration(IndexerConfiguration config)
    {
        if (string.IsNullOrWhiteSpace(config.BitcoinRpcUrl))
            throw new InvalidOperationException("BTC_RPC_URL is required");

        if (string.IsNullOrWhiteSpace(config.BitcoinRpcUsername))
            throw new InvalidOperationException("BTC_RPC_USERNAME is required");

        if (string.IsNullOrWhiteSpace(config.BitcoinRpcPassword))
            throw new InvalidOperationException("BTC_RPC_PASSWORD is required");

        if (string.IsNullOrWhiteSpace(config.DatabaseUrl))
            throw new InvalidOperationException("DATABASE_URL is required");

        if (config.BackfillWorkers < 1)
            throw new InvalidOperationException("BACKFILL_WORKERS must be at least 1");
    }
}
