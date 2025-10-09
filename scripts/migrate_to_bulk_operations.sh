#!/bin/bash

# Migration script to bulk operations
# This script helps migrate from individual operations to bulk operations

set -e

echo "🔄 Migrating to Bulk Operations"
echo "=============================="

# Load configuration
if [ -f "config.env" ]; then
    source config.env
elif [ -f "config.env.example" ]; then
    echo "⚠️  Using config.env.example as template"
    source config.env.example
else
    echo "❌ No configuration file found. Please create config.env"
    exit 1
fi

echo "🗄️  Database: $DATABASE_URL"
echo ""

# Test database connection
if ! psql "$DATABASE_URL" -c "SELECT 1;" > /dev/null 2>&1; then
    echo "❌ Database connection failed. Please check your DATABASE_URL"
    exit 1
fi

echo "✅ Database connection successful"
echo ""

# Check current indexer status
echo "📊 Current Indexer Status:"
echo "========================="

LATEST_HEIGHT=$(psql "$DATABASE_URL" -t -c "SELECT COALESCE(MAX(height), 0) FROM index_progress;")
TOTAL_TRANSACTIONS=$(psql "$DATABASE_URL" -t -c "SELECT COUNT(*) FROM transactions;")
TOTAL_UTXOS=$(psql "$DATABASE_URL" -t -c "SELECT COUNT(*) FROM utxos WHERE is_spent = false;")

echo "   Latest Block Height: $LATEST_HEIGHT"
echo "   Total Transactions: $TOTAL_TRANSACTIONS"
echo "   Total UTXOs: $TOTAL_UTXOS"
echo ""

# Check for performance issues
echo "🔍 Performance Analysis:"
echo "======================="

# Check for slow queries (if pg_stat_statements is available)
if psql "$DATABASE_URL" -c "SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements';" | grep -q "1 row"; then
    echo "📈 Top Slow Queries:"
    psql "$DATABASE_URL" -c "
        SELECT 
            query,
            calls,
            total_time,
            mean_time,
            rows
        FROM pg_stat_statements 
        WHERE query LIKE '%transactions%' OR query LIKE '%utxos%'
        ORDER BY mean_time DESC 
        LIMIT 5;
    " | sed 's/^/     /'
else
    echo "⚠️  pg_stat_statements extension not available for detailed analysis"
fi

echo ""
echo "💡 Migration Recommendations:"
echo "============================"

if [ "$LATEST_HEIGHT" -lt 1000 ]; then
    echo "✅ Small dataset - migration should be quick"
    echo "   Recommended: Use bulk operations for new indexing"
elif [ "$LATEST_HEIGHT" -lt 100000 ]; then
    echo "⚠️  Medium dataset - consider bulk operations for better performance"
    echo "   Recommended: Enable bulk operations in indexer configuration"
else
    echo "🚨 Large dataset - bulk operations are essential"
    echo "   Recommended: Enable bulk operations and consider batch processing"
fi

echo ""
echo "🔧 Next Steps:"
echo "=============="
echo "1. Update indexer configuration to use bulk operations"
echo "2. Restart indexer to apply new settings"
echo "3. Monitor performance with: scripts/view_metrics.sh"
echo "4. Check schema with: scripts/check_schema_diff.sh"

echo ""
echo "✅ Migration analysis complete"
