#!/bin/bash
set -e

echo "🔍 Running quick development checks..."
echo ""

echo "📝 Formatting code..."
make fmt

echo "🧹 Running go vet..."
make vet

echo "🔍 Running linter..."
make lint

echo "🧪 Running tests..."
make test

echo ""
echo "✅ All checks passed! Ready to commit."
