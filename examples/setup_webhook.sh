#!/bin/bash

# Example script to setup webhook notifications

INDEXER_URL="http://localhost:8080"
ADDRESS="bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
WEBHOOK_URL="https://your-service.com/webhook"
WEBHOOK_SECRET="your-webhook-secret"

echo "Setting up webhook for address: $ADDRESS"

curl -X POST "$INDEXER_URL/v1/btc/webhooks" \
  -H "Content-Type: application/json" \
  -d "{
    \"address\": \"$ADDRESS\",
    \"callback_url\": \"$WEBHOOK_URL\",
    \"secret\": \"$WEBHOOK_SECRET\"
  }"

echo ""
echo "Webhook setup complete!"
echo "The indexer will now send notifications to: $WEBHOOK_URL"
