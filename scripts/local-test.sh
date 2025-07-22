#!/bin/bash
set -e

echo "🧪 Running comprehensive local tests..."
echo ""

# Function to run command with time tracking
run_with_time() {
    echo "⏱️  Running: $1"
    start_time=$(date +%s)
    eval "$1"
    end_time=$(date +%s)
    duration=$((end_time - start_time))
    echo "✅ Completed in ${duration}s"
    echo ""
}

run_with_time "make fmt"
run_with_time "make vet"
run_with_time "make generate"
run_with_time "make manifests"

# Check if any files were modified by generation
if [[ -n $(git diff --name-only) ]]; then
    echo "⚠️  Generated files were modified. Please commit these changes:"
    git diff --name-only
    echo ""
fi

run_with_time "make lint"
run_with_time "make test"

echo "📊 Test Coverage Summary:"
go tool cover -func=cover.out | tail -1

echo ""
echo "🎯 Running E2E tests (this may take a while)..."
if make test-e2e; then
    echo "✅ E2E tests passed!"
else
    echo "❌ E2E tests failed. Check the output above."
    exit 1
fi

echo ""
echo "🎉 All tests passed! Your code is ready for commit."
