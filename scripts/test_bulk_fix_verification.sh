#!/bin/bash

# Test bulk operations fix verification
# This script verifies that the bulk operations fix is working

set -e

echo "🔧 Verifying Bulk Operations Fix"
echo "==============================="

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
    echo "🔄 Indexer is running - checking for errors..."
    echo ""
    
    # Check the last few lines of the log for errors
    if [ -f "log.txt" ]; then
        echo "📊 Recent Log Analysis:"
        echo "======================"
        
        # Count errors in the last 50 lines
        ERROR_COUNT=$(tail -50 log.txt | grep -c "❌" || true)
        SUCCESS_COUNT=$(tail -50 log.txt | grep -c "✅ DB Worker" || true)
        
        echo "   Recent errors: $ERROR_COUNT"
        echo "   Recent successes: $SUCCESS_COUNT"
        
        if [ "$ERROR_COUNT" -eq 0 ]; then
            echo "   ✅ No recent errors found!"
        else
            echo "   ⚠️  Found $ERROR_COUNT recent errors"
            echo ""
            echo "   Recent errors:"
            tail -50 log.txt | grep "❌" | tail -5
        fi
        
        echo ""
        echo "   Recent processing times:"
        tail -50 log.txt | grep "✅ DB Worker" | tail -5 | while read line; do
            # Extract processing time from the log line
            time_part=$(echo "$line" | grep -o 'in [0-9.]*[a-z]*' || echo "unknown")
            echo "     $time_part"
        done
        
    else
        echo "⚠️  No log.txt file found"
    fi
else
    echo "🚀 Indexer is not running"
    echo ""
    echo "💡 To test the fix:"
    echo "   1. Start the indexer: ./indexer"
    echo "   2. Monitor for errors: tail -f log.txt"
    echo "   3. Look for processing times under 5 seconds per block"
fi

echo ""
echo "🔍 Expected Results After Fix:"
echo "=============================="
echo "✅ No 'pq: could not determine data type of parameter' errors"
echo "✅ Processing times: 1-5 seconds per block (instead of 14-26 seconds)"
echo "✅ Consistent successful block processing"
echo "✅ Bulk operations working correctly"
