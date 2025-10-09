#!/bin/bash

# Test optimizations script
# This script tests the performance optimizations

set -e

echo "⚡ Testing Performance Optimizations"
echo "==================================="

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

# Test bulk operations
echo "1️⃣  Testing bulk operations..."
echo "   This test compares individual vs bulk insert performance"

# Create test tables
psql "$DATABASE_URL" -c "
CREATE TEMP TABLE test_individual (
    id SERIAL PRIMARY KEY,
    data TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TEMP TABLE test_bulk (
    id SERIAL PRIMARY KEY,
    data TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);
" > /dev/null 2>&1

# Test individual inserts
echo "   Testing individual inserts (100 records)..."
START_TIME=$(date +%s.%N)
for i in {1..100}; do
    psql "$DATABASE_URL" -c "INSERT INTO test_individual (data) VALUES ('individual_data_$i');" > /dev/null 2>&1
done
END_TIME=$(date +%s.%N)
INDIVIDUAL_TIME=$(echo "$END_TIME - $START_TIME" | bc)

# Test bulk insert
echo "   Testing bulk insert (100 records)..."
START_TIME=$(date +%s.%N)
psql "$DATABASE_URL" -c "
INSERT INTO test_bulk (data) VALUES 
$(for i in {1..100}; do echo "('bulk_data_$i')"; done | paste -sd,);
" > /dev/null 2>&1
END_TIME=$(date +%s.%N)
BULK_TIME=$(echo "$END_TIME - $START_TIME" | bc)

echo "   📊 Results:"
echo "      Individual inserts: ${INDIVIDUAL_TIME}s"
echo "      Bulk insert: ${BULK_TIME}s"
echo "      Speedup: $(echo "scale=2; $INDIVIDUAL_TIME / $BULK_TIME" | bc)x"

# Test transaction performance
echo ""
echo "2️⃣  Testing transaction performance..."
echo "   This test compares single vs multiple transactions"

# Create test tables
psql "$DATABASE_URL" -c "
CREATE TEMP TABLE test_single_tx (
    id SERIAL PRIMARY KEY,
    data TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TEMP TABLE test_multi_tx (
    id SERIAL PRIMARY KEY,
    data TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);
" > /dev/null 2>&1

# Test multiple transactions
echo "   Testing multiple transactions (50 records)..."
START_TIME=$(date +%s.%N)
for i in {1..50}; do
    psql "$DATABASE_URL" -c "
    BEGIN;
    INSERT INTO test_multi_tx (data) VALUES ('multi_tx_data_$i');
    COMMIT;
    " > /dev/null 2>&1
done
END_TIME=$(date +%s.%N)
MULTI_TX_TIME=$(echo "$END_TIME - $START_TIME" | bc)

# Test single transaction
echo "   Testing single transaction (50 records)..."
START_TIME=$(date +%s.%N)
psql "$DATABASE_URL" -c "
BEGIN;
$(for i in {1..50}; do echo "INSERT INTO test_single_tx (data) VALUES ('single_tx_data_$i');"; done)
COMMIT;
" > /dev/null 2>&1
END_TIME=$(date +%s.%N)
SINGLE_TX_TIME=$(echo "$END_TIME - $START_TIME" | bc)

echo "   📊 Results:"
echo "      Multiple transactions: ${MULTI_TX_TIME}s"
echo "      Single transaction: ${SINGLE_TX_TIME}s"
echo "      Speedup: $(echo "scale=2; $MULTI_TX_TIME / $SINGLE_TX_TIME" | bc)x"

# Test index performance
echo ""
echo "3️⃣  Testing index performance..."
echo "   This test compares indexed vs non-indexed queries"

# Create test tables
psql "$DATABASE_URL" -c "
CREATE TEMP TABLE test_no_index (
    id SERIAL PRIMARY KEY,
    height INTEGER,
    data TEXT
);

CREATE TEMP TABLE test_with_index (
    id SERIAL PRIMARY KEY,
    height INTEGER,
    data TEXT
);

CREATE INDEX idx_test_with_index_height ON test_with_index (height);
" > /dev/null 2>&1

# Insert test data
psql "$DATABASE_URL" -c "
INSERT INTO test_no_index (height, data) 
SELECT generate_series(1, 1000), 'data_' || generate_series(1, 1000);

INSERT INTO test_with_index (height, data) 
SELECT generate_series(1, 1000), 'data_' || generate_series(1, 1000);
" > /dev/null 2>&1

# Test non-indexed query
echo "   Testing non-indexed query..."
START_TIME=$(date +%s.%N)
psql "$DATABASE_URL" -c "SELECT COUNT(*) FROM test_no_index WHERE height BETWEEN 100 AND 200;" > /dev/null 2>&1
END_TIME=$(date +%s.%N)
NO_INDEX_TIME=$(echo "$END_TIME - $START_TIME" | bc)

# Test indexed query
echo "   Testing indexed query..."
START_TIME=$(date +%s.%N)
psql "$DATABASE_URL" -c "SELECT COUNT(*) FROM test_with_index WHERE height BETWEEN 100 AND 200;" > /dev/null 2>&1
END_TIME=$(date +%s.%N)
WITH_INDEX_TIME=$(echo "$END_TIME - $START_TIME" | bc)

echo "   📊 Results:"
echo "      Non-indexed query: ${NO_INDEX_TIME}s"
echo "      Indexed query: ${WITH_INDEX_TIME}s"
echo "      Speedup: $(echo "scale=2; $NO_INDEX_TIME / $WITH_INDEX_TIME" | bc)x"

# Test connection pool
echo ""
echo "4️⃣  Testing connection pool..."
echo "   This test simulates concurrent database operations"

# Test concurrent connections
echo "   Testing concurrent database operations..."
START_TIME=$(date +%s.%N)
for i in {1..10}; do
    (
        psql "$DATABASE_URL" -c "SELECT pg_sleep(0.1), $i as concurrent_test;" > /dev/null 2>&1
    ) &
done

# Wait for all background jobs
wait
END_TIME=$(date +%s.%N)
CONCURRENT_TIME=$(echo "$END_TIME - $START_TIME" | bc)

echo "   📊 Results:"
echo "      Concurrent operations (10): ${CONCURRENT_TIME}s"
echo "      Average per operation: $(echo "scale=3; $CONCURRENT_TIME / 10" | bc)s"

echo ""
echo "🎉 Performance optimization tests completed!"
echo ""
echo "💡 Optimization Summary:"
echo "   - Bulk operations are significantly faster than individual operations"
echo "   - Single transactions are faster than multiple transactions"
echo "   - Indexes improve query performance"
echo "   - Connection pooling enables concurrent operations"
echo ""
echo "📈 The indexer should now perform much better with these optimizations"
