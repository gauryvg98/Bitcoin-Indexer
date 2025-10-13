using BitcoinIndexer.Core.Configuration;
using BitcoinIndexer.Core.Interfaces;
using BitcoinIndexer.Infrastructure.Bitcoin;
using BitcoinIndexer.Infrastructure.Configuration;
using BitcoinIndexer.Infrastructure.Data;
using BitcoinIndexer.Infrastructure.Repositories;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;

namespace BitcoinIndexer.Infrastructure.Extensions;

/// <summary>
/// Extension methods for configuring services
/// </summary>
public static class ServiceCollectionExtensions
{
    /// <summary>
    /// Adds configuration services to the dependency injection container
    /// </summary>
    public static IServiceCollection AddIndexerConfiguration(this IServiceCollection services, IConfiguration configuration)
    {
        // Configure the IndexerConfiguration from appsettings.json and environment variables
        services.Configure<IndexerConfiguration>(configuration.GetSection("Indexer"));

        // Register the configuration service
        services.AddSingleton<ConfigurationService>();

        return services;
    }

    /// <summary>
    /// Adds database services to the dependency injection container
    /// </summary>
    public static IServiceCollection AddIndexerDatabase(this IServiceCollection services, IConfiguration configuration)
    {
        // Get database connection string from configuration
        var connectionString = configuration.GetConnectionString("DefaultConnection") 
            ?? configuration["Indexer:DatabaseUrl"];

        if (string.IsNullOrEmpty(connectionString))
        {
            throw new InvalidOperationException("Database connection string is required");
        }

        // Add Entity Framework DbContext
        services.AddDbContext<BitcoinIndexerDbContext>(options =>
        {
            options.UseNpgsql(connectionString, npgsqlOptions =>
            {
                npgsqlOptions.EnableRetryOnFailure(
                    maxRetryCount: 3,
                    maxRetryDelay: TimeSpan.FromSeconds(30),
                    errorCodesToAdd: null);
                // Use no tracking to avoid concurrency issues
                npgsqlOptions.UseQuerySplittingBehavior(QuerySplittingBehavior.SingleQuery);
            });
            // Use NoTracking by default to avoid concurrency issues during block processing
            options.UseQueryTrackingBehavior(QueryTrackingBehavior.NoTracking);
            options.EnableSensitiveDataLogging();
            options.EnableDetailedErrors();
        });

        // Register repositories
        services.AddScoped<IBitcoinIndexerRepository, BitcoinIndexerRepository>();

        return services;
    }

    /// <summary>
    /// Adds Bitcoin RPC services to the dependency injection container
    /// </summary>
    public static IServiceCollection AddBitcoinRpc(this IServiceCollection services)
    {
        // Register HTTP client for Bitcoin RPC
        services.AddHttpClient<IBitcoinRpcClient, BitcoinRpcClient>();

        return services;
    }

}
