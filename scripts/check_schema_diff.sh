#!/bin/bash

# Check database schema differences
# This script compares the current database schema with the expected schema

set -e

echo "🔍 Checking Database Schema Differences"
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

# Test database connection
if ! psql "$DATABASE_URL" -c "SELECT 1;" > /dev/null 2>&1; then
    echo "❌ Database connection failed. Please check your DATABASE_URL"
    exit 1
fi

echo "✅ Database connection successful"
echo ""

# Check if schema exists
SCHEMA_EXISTS=$(psql "$DATABASE_URL" -t -c "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'index_progress');")

if [ "$SCHEMA_EXISTS" = " f" ]; then
    echo "❌ Database schema not found. Please run setup_comprehensive.sh first"
    exit 1
fi

echo "✅ Database schema exists"
echo ""

# Check table structure
echo "📋 Current Table Structure:"
echo "=========================="

TABLES=("index_progress" "transactions" "transaction_inputs" "transaction_outputs" "utxos" "watched_scripts" "transaction_references")

for table in "${TABLES[@]}"; do
    echo ""
    echo "📊 Table: $table"
    echo "   Columns:"
    psql "$DATABASE_URL" -c "\d $table" | grep "Column" | sed 's/^/     /'
    
    echo "   Indexes:"
    psql "$DATABASE_URL" -c "\d $table" | grep "Index" | sed 's/^/     /'
    
    echo "   Constraints:"
    psql "$DATABASE_URL" -c "\d $table" | grep "Check\|Foreign\|Primary" | sed 's/^/     /'
done

echo ""
echo "🔍 Schema Validation Complete"
echo ""
echo "💡 If you see any issues:"
echo "   1. Run: scripts/reset_database.sh"
echo "   2. Or manually apply schema: psql \$DATABASE_URL -f internal/database/schema.sql"
