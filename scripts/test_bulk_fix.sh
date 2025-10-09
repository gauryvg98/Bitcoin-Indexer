#!/bin/bash

# Test bulk operations fix
# This script tests the performance improvement from using bulk operations

set -e

echo "⚡ Testing Bulk Operations Fix"
echo "============================="

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

# Test basic connection
if ! psql "$DATABASE_URL" -c "SELECT 1;" > /dev/null 2>&1; then
    echo "❌ Database connection failed"
    exit 1
fi

echo "✅ Database connection successful"
echo ""

# Check if indexer is running
if pgrep -f "./indexer" > /dev/null; then
    echo "🔄 Indexer is running - monitoring performance..."
    echo ""
    echo "📊 Performance Monitoring (Press Ctrl+C to stop):"
    echo "================================================"
    
    # Monitor the log file for performance metrics
    if [ -f "log.txt" ]; then
        tail -f log.txt | grep -E "(✅ DB Worker|📦 RPC Worker)" | while read line; do
            echo "$(date '+%H:%M:%S') $line"
        done
    else
        echo "⚠️  No log.txt file found. Start the indexer to see performance metrics."
    fi
else
    echo "🚀 Starting indexer to test bulk operations fix..."
    echo ""
    echo "💡 The indexer should now process blocks much faster with bulk operations."
    echo "   Previous performance: 14-26 seconds per block"
    echo "   Expected performance: 1-3 seconds per block"
    echo ""
    echo "📊 Starting indexer (Press Ctrl+C to stop):"
    echo "=========================================="
    
    # Start the indexer and monitor its output
    ./indexer 2>&1 | tee log.txt
fi
