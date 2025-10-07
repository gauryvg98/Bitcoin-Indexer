#!/bin/bash

# Test script for the enhanced Bitcoin indexer
# This script demonstrates the new functionality

echo "🚀 Testing Enhanced Bitcoin Indexer"
echo "=================================="

# Start the indexer in the background
echo "Starting Bitcoin indexer..."
./bitcoin-indexer &
INDEXER_PID=$!

# Wait for the indexer to start
sleep 5

echo ""
echo "📊 Testing Enhanced Features"
echo "============================"

# Test 1: Connect a wallet instantly
echo "1. Connecting wallet instantly..."
curl -X POST http://localhost:8080/v1/btc/connect \
  -H "Content-Type: application/json" \
  -d '{"address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"}' \
  | jq '.'

echo ""
echo "2. Getting transaction history for the connected wallet..."
curl http://localhost:8080/v1/btc/history/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa?limit=10 \
  | jq '.'

echo ""
echo "3. Getting balance for the connected wallet..."
curl http://localhost:8080/v1/btc/balance/1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa \
  | jq '.'

echo ""
echo "4. Testing with another address..."
curl -X POST http://localhost:8080/v1/btc/connect \
  -H "Content-Type: application/json" \
  -d '{"address": "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"}' \
  | jq '.'

echo ""
echo "5. Getting transaction history for the second wallet..."
curl http://localhost:8080/v1/btc/history/1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2?limit=5 \
  | jq '.'

echo ""
echo "6. Checking indexer status..."
curl http://localhost:8080/v1/btc/status \
  | jq '.'

echo ""
echo "✅ Enhanced Bitcoin Indexer Test Complete!"
echo "=========================================="
echo ""
echo "Key Features Demonstrated:"
echo "• Instant wallet connection with historical data"
echo "• Transaction history with sender/receiver information"
echo "• All UTXOs stored for fast backfill"
echo "• Sender computation only for watched addresses"
echo "• Parallel processing for improved performance"
echo ""

# Clean up
echo "Stopping indexer..."
kill $INDEXER_PID 2>/dev/null
echo "Test completed!"
