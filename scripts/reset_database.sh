#!/bin/bash

# Reset database script for Bitcoin indexer
# This script drops and recreates the database schema

set -e

# Load configuration
if [ -f ".env" ]; then
    source .env
elif [ -f "config.env.example" ]; then
    echo "⚠️  Using config.env.example as template"
    source config.env.example
else
    echo "❌ No configuration file found. Please create config.env"
    exit 1
fi

echo "🗄️  Resetting Bitcoin indexer database..."
echo "   Database: $DATABASE_URL"

# Confirm before proceeding
read -p "⚠️  This will DROP and recreate the database. Continue? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ Operation cancelled"
    exit 1
fi

# Drop and recreate database
echo "🔄 Dropping existing database..."
psql "$DATABASE_URL" -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

echo "🔄 Creating database schema..."
psql "$DATABASE_URL" -f internal/database/schema.sql

echo "✅ Database reset completed successfully!"
echo "   You can now start the indexer to begin indexing"
