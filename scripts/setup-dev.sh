#!/bin/bash
set -e

echo "🚀 Setting up development environment..."
echo ""

echo "📦 Installing development dependencies..."
make setup-envtest
make controller-gen
make kustomize
make golangci-lint

echo "🪝 Setting up pre-commit hooks..."
if command -v pre-commit &> /dev/null; then
    pre-commit install
    echo "✅ Pre-commit hooks installed"
else
    echo "⚠️  pre-commit not found. Install it with: pip install pre-commit"
fi

echo "🧪 Running initial tests to verify setup..."
make test

echo ""
echo "✅ Development environment setup complete!"
echo ""
echo "Next steps:"
echo "  1. Run 'make help' to see available commands"
echo "  2. Run './scripts/quick-test.sh' before committing"
echo "  3. See DEVELOPMENT.md for detailed workflow"
