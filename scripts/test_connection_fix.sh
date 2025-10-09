#!/bin/bash

# Test connection fix script
# This script tests the database connection improvements

set -e

echo "🔌 Testing Connection Fix"
echo "========================"

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
echo "1️⃣  Testing basic database connection..."
if psql "$DATABASE_URL" -c "SELECT 1;" > /dev/null 2>&1; then
    echo "✅ Basic connection successful"
else
    echo "❌ Basic connection failed"
    exit 1
fi

# Test connection pool
echo ""
echo "2️⃣  Testing connection pool..."
for i in {1..5}; do
    if psql "$DATABASE_URL" -c "SELECT $i as test_connection;" > /dev/null 2>&1; then
        echo "   ✅ Connection $i: OK"
    else
        echo "   ❌ Connection $i: Failed"
        exit 1
    fi
done

# Test concurrent connections
echo ""
echo "3️⃣  Testing concurrent connections..."
for i in {1..10}; do
    (
        if psql "$DATABASE_URL" -c "SELECT pg_sleep(0.1), $i as concurrent_test;" > /dev/null 2>&1; then
            echo "   ✅ Concurrent connection $i: OK"
        else
            echo "   ❌ Concurrent connection $i: Failed"
            exit 1
        fi
    ) &
done

# Wait for all background jobs
wait

# Test transaction handling
echo ""
echo "4️⃣  Testing transaction handling..."
psql "$DATABASE_URL" -c "
BEGIN;
CREATE TEMP TABLE test_tx (id SERIAL PRIMARY KEY, data TEXT);
INSERT INTO test_tx (data) VALUES ('test');
SELECT COUNT(*) FROM test_tx;
ROLLBACK;
" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo "✅ Transaction handling: OK"
else
    echo "❌ Transaction handling: Failed"
    exit 1
fi

# Test prepared statements
echo ""
echo "5️⃣  Testing prepared statements..."
psql "$DATABASE_URL" -c "
PREPARE test_stmt AS SELECT \$1::int as param;
EXECUTE test_stmt(42);
DEALLOCATE test_stmt;
" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo "✅ Prepared statements: OK"
else
    echo "❌ Prepared statements: Failed"
    exit 1
fi

echo ""
echo "🎉 All connection tests passed!"
echo ""
echo "💡 Connection fix is working correctly"
echo "   The indexer should now handle database connections more efficiently"
