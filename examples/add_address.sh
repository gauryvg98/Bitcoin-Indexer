#!/bin/bash

# Example script to add a Bitcoin address to the indexer

INDEXER_URL="http://localhost:8080"
ADDRESS="bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"

echo "Adding Bitcoin address to indexer..."

curl -X POST "$INDEXER_URL/v1/btc/watch" \
  -H "Content-Type: application/json" \
  -d "{
    \"kind\": \"address\",
    \"address\": \"$ADDRESS\"
  }"

echo ""
echo "Address added successfully!"
echo "You can now query balances at: $INDEXER_URL/v1/btc/balance/$ADDRESS"
