#!/bin/bash

# Performance analysis script
# This script analyzes the performance bottlenecks in the indexer

set -e

echo "🔍 Bitcoin Indexer Performance Analysis"
echo "======================================"

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
    echo "🔄 Indexer is running - analyzing performance..."
    echo ""
    
    # Check the last few lines of the log for performance data
    if [ -f "log.txt" ]; then
        echo "📊 Recent Performance Analysis:"
        echo "=============================="
        
        # Get the last 100 lines and analyze them
        tail -100 log.txt > /tmp/recent_log.txt
        
        # Count different types of operations
        TOTAL_BLOCKS=$(grep -c "✅ DB Worker" /tmp/recent_log.txt || echo "0")
        ERROR_COUNT=$(grep -c "❌" /tmp/recent_log.txt || echo "0")
        
        echo "   Total blocks processed: $TOTAL_BLOCKS"
        echo "   Errors: $ERROR_COUNT"
        echo ""
        
        if [ "$TOTAL_BLOCKS" -gt 0 ]; then
            echo "📈 Processing Time Analysis:"
            echo "==========================="
            
            # Extract processing times and calculate statistics
            grep "✅ DB Worker" /tmp/recent_log.txt | tail -10 | while read line; do
                # Extract time from the log line
                time_part=$(echo "$line" | grep -o 'in [0-9.]*[a-z]*' | sed 's/in //')
                block_info=$(echo "$line" | grep -o 'block [0-9]* ([0-9]* txs' | sed 's/block //' | sed 's/ (.*//')
                echo "   Block $block_info: $time_part"
            done
            
            echo ""
            echo "🔍 Detailed Operation Breakdown:"
            echo "==============================="
            
            # Analyze the detailed operation timings
            if grep -q "📊 DB Worker" /tmp/recent_log.txt; then
                echo "   Recent operation timings:"
                grep "📊 DB Worker" /tmp/recent_log.txt | tail -20 | while read line; do
                    echo "     $line"
                done
            else
                echo "   ⚠️  No detailed operation timings found"
                echo "   Start the indexer to see detailed performance breakdown"
            fi
            
            echo ""
            echo "📊 Performance Summary:"
            echo "====================="
            
            # Calculate average processing time
            if [ "$TOTAL_BLOCKS" -gt 0 ]; then
                echo "   Recent blocks processed: $TOTAL_BLOCKS"
                if [ "$ERROR_COUNT" -eq 0 ]; then
                    echo "   ✅ No recent errors"
                else
                    echo "   ⚠️  $ERROR_COUNT recent errors found"
                fi
            fi
            
        else
            echo "⚠️  No blocks processed recently"
        fi
        
        # Clean up temp file
        rm -f /tmp/recent_log.txt
        
    else
        echo "⚠️  No log.txt file found"
    fi
    
    echo ""
    echo "💡 Performance Optimization Tips:"
    echo "================================"
    echo "1. Look for operations taking > 5 seconds"
    echo "2. Check if UTXO operations are the bottleneck"
    echo "3. Monitor transaction processing times"
    echo "4. Watch for commit times > 1 second"
    echo ""
    echo "🔧 If performance is still slow:"
    echo "   - Check database indexes"
    echo "   - Monitor database connections"
    echo "   - Consider increasing batch sizes"
    echo "   - Check for database locks"
    
else
    echo "🚀 Indexer is not running"
    echo ""
    echo "💡 To analyze performance:"
    echo "   1. Start the indexer: ./indexer"
    echo "   2. Run this script again: ./scripts/analyze_performance.sh"
    echo "   3. Monitor the detailed operation timings"
fi

echo ""
echo "📋 Database Performance Check:"
echo "============================="

# Check database performance
echo "   Checking database indexes..."
INDEX_COUNT=$(psql "$DATABASE_URL" -t -c "SELECT COUNT(*) FROM pg_indexes WHERE schemaname = 'public';" 2>/dev/null || echo "0")
echo "   Database indexes: $INDEX_COUNT"

echo "   Checking table sizes..."
UTXO_COUNT=$(psql "$DATABASE_URL" -t -c "SELECT COUNT(*) FROM utxo;" 2>/dev/null || echo "0")
TX_COUNT=$(psql "$DATABASE_URL" -t -c "SELECT COUNT(*) FROM transaction;" 2>/dev/null || echo "0")
echo "   UTXOs: $UTXO_COUNT"
echo "   Transactions: $TX_COUNT"

echo ""
echo "✅ Performance analysis complete"
