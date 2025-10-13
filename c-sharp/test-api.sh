#!/bin/bash

# Bitcoin Indexer C# API Test Script
# This script tests the main API endpoints

set -e

API_BASE_URL="http://localhost:8080"
TEST_ADDRESS="1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"  # Genesis block address

echo "🧪 Bitcoin Indexer C# API Tests"
echo "==============================="

# Test health check
echo "1. Testing health check..."
curl -s "$API_BASE_URL/health" | jq '.' || echo "❌ Health check failed"

echo ""

# Test indexer status
echo "2. Testing indexer status..."
curl -s "$API_BASE_URL/v1/btc/status" | jq '.' || echo "❌ Status check failed"

echo ""

# Test adding a watch target
echo "3. Testing add watch target..."
curl -s -X POST "$API_BASE_URL/v1/btc/watch" \
  -H "Content-Type: application/json" \
  -d "{\"kind\": \"address\", \"address\": \"$TEST_ADDRESS\"}" | jq '.' || echo "❌ Add watch target failed"

echo ""

# Test getting watch targets
echo "4. Testing get watch targets..."
curl -s "$API_BASE_URL/v1/btc/watch" | jq '.' || echo "❌ Get watch targets failed"

echo ""

# Test getting balance
echo "5. Testing get balance..."
curl -s "$API_BASE_URL/v1/btc/balance/$TEST_ADDRESS" | jq '.' || echo "❌ Get balance failed"

echo ""

# Test getting UTXOs
echo "6. Testing get UTXOs..."
curl -s "$API_BASE_URL/v1/btc/utxos/$TEST_ADDRESS" | jq '.' || echo "❌ Get UTXOs failed"

echo ""

# Test connecting a wallet
echo "7. Testing connect wallet..."
curl -s -X POST "$API_BASE_URL/v1/btc/connect" \
  -H "Content-Type: application/json" \
  -d "{\"address\": \"$TEST_ADDRESS\", \"label\": \"Test Wallet\"}" | jq '.' || echo "❌ Connect wallet failed"

echo ""

# Test getting transaction history
echo "8. Testing get transaction history..."
curl -s "$API_BASE_URL/v1/btc/history/$TEST_ADDRESS" | jq '.' || echo "❌ Get transaction history failed"

echo ""
echo "✅ API tests completed!"
echo ""
echo "Note: Some tests may fail if the indexer hasn't processed any blocks yet."
echo "Make sure the Bitcoin RPC connection is configured correctly."
