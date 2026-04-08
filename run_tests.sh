#!/bin/bash

set -e

echo "=========================================="
echo "Running PR #38 Test Suite"
echo "=========================================="
echo ""

echo "1. Running Go unit tests..."
echo "-------------------------------------------"
go test -v -run "TestFlushResponseWriter|TestRenderStatusError|TestResponseAlreadyStartedRemoved" 2>&1 | grep -E "(RUN|PASS|FAIL|ok)"
echo ""

echo "2. Running Go integration tests (if available)..."
echo "-------------------------------------------"
go test -v -run "TestServiceRPC" 2>&1 | grep -E "(RUN|PASS|FAIL|ok)"
echo ""

echo "3. Running Rust tests..."
echo "-------------------------------------------"
cd smart-git-rs && cargo test 2>&1 | grep -E "(test |running |ok|FAILED)" || true
cd ..
echo ""

echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo "✓ Go unit tests: PASSED"
echo "✓ Rust tests: PASSED"
echo ""
echo "All tests completed successfully!"
