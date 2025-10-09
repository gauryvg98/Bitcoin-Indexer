#!/bin/bash

# View performance metrics script
# This script displays indexer performance metrics

echo "📊 Bitcoin Indexer Performance Metrics"
echo "====================================="

# Check if metrics file exists
if [ ! -f "indexer_metrics.json" ]; then
    echo "❌ No metrics file found (indexer_metrics.json)"
    echo "   Run the indexer first to generate metrics"
    exit 1
fi

echo "📈 Current Performance Metrics:"
echo ""

# Parse and display metrics
if command -v jq &> /dev/null; then
    echo "📊 Range Metrics:"
    jq -r '.range_metrics | to_entries[] | "   Range \(.key): \(.value.block_count) blocks, \(.value.avg_block_time_ms | tonumber / 1000 | . * 100 | round / 100)s avg, \(.value.blocks_per_second | . * 100 | round / 100) blocks/sec"' indexer_metrics.json
    
    echo ""
    echo "⏱️  Time Breakdown:"
    jq -r '.range_metrics | to_entries[] | "   Range \(.key): RPC=\(.value.rpc_time_percent | . * 100 | round / 100)%, DB=\(.value.db_time_percent | . * 100 | round / 100)%, Parse=\(.value.parse_time_percent | . * 100 | round / 100)%"' indexer_metrics.json
    
    echo ""
    echo "📦 Transaction Stats:"
    jq -r '.range_metrics | to_entries[] | "   Range \(.key): \(.value.total_transactions) txs, \(.value.total_inputs) inputs, \(.value.total_outputs) outputs"' indexer_metrics.json
    
    echo ""
    echo "💰 UTXO Stats:"
    jq -r '.range_metrics | to_entries[] | "   Range \(.key): \(.value.total_new_utxos) new UTXOs, \(.value.total_spent_utxos) spent UTXOs"' indexer_metrics.json
    
else
    echo "⚠️  jq not installed. Installing for better metrics display..."
    if command -v brew &> /dev/null; then
        brew install jq
    elif command -v apt-get &> /dev/null; then
        sudo apt-get install jq
    else
        echo "❌ Please install jq manually for better metrics display"
        echo "   Raw metrics file: indexer_metrics.json"
        cat indexer_metrics.json
    fi
fi

echo ""
echo "📝 Last Updated:"
if command -v jq &> /dev/null; then
    jq -r '.range_metrics | to_entries[] | "   Range \(.key): \(.value.last_updated)"' indexer_metrics.json
else
    echo "   Check indexer_metrics.json for last_updated timestamps"
fi
