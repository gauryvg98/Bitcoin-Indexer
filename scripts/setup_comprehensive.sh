#!/bin/bash

# Comprehensive setup script for Bitcoin indexer
# This script sets up the complete environment

set -e

echo "🚀 Bitcoin Indexer Comprehensive Setup"
echo "====================================="

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go first."
    exit 1
fi

echo "✅ Go is installed: $(go version)"

# Check if PostgreSQL is installed
if ! command -v psql &> /dev/null; then
    echo "❌ PostgreSQL is not installed. Please install PostgreSQL first."
    exit 1
fi

echo "✅ PostgreSQL is installed: $(psql --version)"

# Create config file if it doesn't exist
if [ ! -f "config.env" ]; then
    echo "📝 Creating config.env from template..."
    cp config.env.example config.env
    echo "⚠️  Please edit config.env with your database and Bitcoin RPC settings"
fi

# Install Go dependencies
echo "📦 Installing Go dependencies..."
go mod tidy

# Build the indexer
echo "🔨 Building indexer..."
go build -o indexer cmd/indexer/main.go

# Setup database
echo "🗄️  Setting up database..."
if [ -f "config.env" ]; then
    source config.env
    echo "   Database: $DATABASE_URL"
    
    # Test database connection
    if psql "$DATABASE_URL" -c "SELECT 1;" > /dev/null 2>&1; then
        echo "✅ Database connection successful"
        
        # Create schema
        echo "🔄 Creating database schema..."
        psql "$DATABASE_URL" -f internal/database/schema.sql
        echo "✅ Database schema created"
    else
        echo "❌ Database connection failed. Please check your DATABASE_URL in config.env"
        exit 1
    fi
else
    echo "⚠️  config.env not found. Please create it from config.env.example"
fi

echo ""
echo "🎉 Setup completed successfully!"
echo ""
echo "📋 Next steps:"
echo "   1. Edit config.env with your Bitcoin RPC settings"
echo "   2. Run: ./indexer"
echo "   3. Or use: go run cmd/indexer/main.go"
echo ""
echo "📚 Available scripts:"
echo "   - scripts/reset_database.sh    # Reset database"
echo "   - scripts/view_metrics.sh      # View performance metrics"
echo "   - examples/add_address.sh      # Add Bitcoin address to watch"
