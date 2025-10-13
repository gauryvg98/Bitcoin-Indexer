#!/bin/bash

# Bitcoin Indexer C# Setup Script
# This script helps set up the Bitcoin Indexer C# application

set -e

echo "🚀 Bitcoin Indexer C# Setup"
echo "=========================="

# Check if .NET 9 is installed
if ! command -v dotnet &> /dev/null; then
    echo "❌ .NET SDK is not installed. Please install .NET 9.0 SDK first."
    echo "   Visit: https://dotnet.microsoft.com/download/dotnet/9.0"
    exit 1
fi

# Check .NET version
DOTNET_VERSION=$(dotnet --version)
echo "✅ .NET version: $DOTNET_VERSION"

# Check if PostgreSQL is available
if ! command -v psql &> /dev/null; then
    echo "⚠️  PostgreSQL client not found. Please ensure PostgreSQL is installed and accessible."
    echo "   Visit: https://www.postgresql.org/download/"
fi

# Restore dependencies
echo "📦 Restoring NuGet packages..."
dotnet restore

# Build the solution
echo "🔨 Building the solution..."
dotnet build

# Check if database connection string is configured
if [ ! -f "BitcoinIndexer.Api/appsettings.Production.json" ]; then
    echo "📝 Creating production configuration..."
    cp BitcoinIndexer.Api/appsettings.Development.json BitcoinIndexer.Api/appsettings.Production.json
    echo "✅ Created appsettings.Production.json"
    echo "⚠️  Please update the configuration with your database and Bitcoin RPC settings"
fi

# Check if Entity Framework tools are available
if ! dotnet ef --version &> /dev/null; then
    echo "📦 Installing Entity Framework tools..."
    dotnet tool install --global dotnet-ef
fi

echo ""
echo "🎉 Setup completed successfully!"
echo ""
echo "Next steps:"
echo "1. Update BitcoinIndexer.Api/appsettings.Production.json with your configuration"
echo "2. Create a PostgreSQL database: CREATE DATABASE bitcoin_indexer;"
echo "3. Run database migrations: dotnet ef database update --project BitcoinIndexer.Infrastructure --startup-project BitcoinIndexer.Api"
echo "4. Start the application: dotnet run --project BitcoinIndexer.Api"
echo ""
echo "For more information, see the README.md file."
