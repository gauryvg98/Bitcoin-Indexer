#!/bin/bash

# Comprehensive Bitcoin Indexer Setup Script
# This script sets up the complete data storage system with the correct DATABASE_URL

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Bitcoin Indexer Comprehensive Setup${NC}"
echo "=============================================="

# Set the correct DATABASE_URL
export DATABASE_URL="postgres://yashvardhangaur@localhost/bitcoin_indexer?sslmode=disable"

echo -e "${YELLOW}📋 Configuration:${NC}"
echo "  DATABASE_URL: $DATABASE_URL"
echo ""

# Function to check if PostgreSQL is available
check_postgres() {
    echo -e "${YELLOW}📡 Checking PostgreSQL connection...${NC}"
    
    if ! command -v psql &> /dev/null; then
        echo -e "${RED}❌ PostgreSQL client (psql) not found. Please install PostgreSQL.${NC}"
        exit 1
    fi
    
    # Test connection using the DATABASE_URL
    if ! psql "$DATABASE_URL" -c "SELECT 1;" &> /dev/null; then
        echo -e "${RED}❌ Cannot connect to PostgreSQL database.${NC}"
        echo "Please check your database configuration:"
        echo "  DATABASE_URL: $DATABASE_URL"
        echo ""
        echo "Make sure PostgreSQL is running and the database exists:"
        echo "  createdb bitcoin_indexer"
        exit 1
    fi
    
    echo -e "${GREEN}✅ PostgreSQL connection successful${NC}"
}

# Function to drop all tables
drop_all_tables() {
    echo -e "${YELLOW}🗑️  Dropping all tables...${NC}"
    
    # Drop all tables in the correct order (respecting foreign key constraints)
    psql "$DATABASE_URL" -c "
        DROP TABLE IF EXISTS tx_reference CASCADE;
        DROP TABLE IF EXISTS address_metadata CASCADE;
        DROP TABLE IF EXISTS block_info CASCADE;
        DROP TABLE IF EXISTS utxo CASCADE;
        DROP TABLE IF EXISTS transaction_output CASCADE;
        DROP TABLE IF EXISTS transaction_input CASCADE;
        DROP TABLE IF EXISTS transaction CASCADE;
        DROP TABLE IF EXISTS watched_script CASCADE;
        DROP TABLE IF EXISTS watch_target CASCADE;
        DROP TABLE IF EXISTS index_progress CASCADE;
    "
    
    echo -e "${GREEN}✅ All tables dropped successfully${NC}"
}

# Function to apply comprehensive schema
apply_schema() {
    echo -e "${YELLOW}📊 Applying comprehensive schema...${NC}"
    
    # Apply the comprehensive schema
    psql "$DATABASE_URL" -f internal/database/schema.sql
    
    echo -e "${GREEN}✅ Comprehensive schema applied${NC}"
}

# Function to show database status
show_status() {
    echo -e "${YELLOW}📊 Database Status:${NC}"
    echo "=================="
    
    psql "$DATABASE_URL" -c "
        SELECT 
            schemaname,
            relname as tablename,
            n_tup_ins as inserts,
            n_tup_upd as updates,
            n_tup_del as deletes
        FROM pg_stat_user_tables 
        ORDER BY relname;
    "
}

# Function to show next steps
show_next_steps() {
    echo -e "${GREEN}🎉 Comprehensive setup completed successfully!${NC}"
    echo ""
    echo -e "${YELLOW}📋 Next Steps:${NC}"
    echo "1. Start the Bitcoin indexer:"
    echo "   go run ./cmd/indexer"
    echo ""
    echo "2. Or build and run:"
    echo "   go build -o bitcoin-indexer ./cmd/indexer"
    echo "   ./bitcoin-indexer"
    echo ""
    echo "3. Add addresses to watch:"
    echo "   curl -X POST http://localhost:8080/api/watch-targets \\"
    echo "     -H 'Content-Type: application/json' \\"
    echo "     -d '{\"kind\":\"address\",\"address\":\"bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh\"}'"
    echo ""
    echo "4. Query transaction data:"
    echo "   curl http://localhost:8080/api/transactions/bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
    echo ""
    echo -e "${BLUE}💡 The indexer will now store ALL transaction data comprehensively!${NC}"
}

# Main execution
main() {
    check_postgres
    drop_all_tables
    apply_schema
    show_status
    show_next_steps
}

# Run main function
main
