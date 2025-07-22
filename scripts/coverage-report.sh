#!/bin/bash
set -e

echo "📊 Generating test coverage report..."
echo ""

make test

echo "🌐 Creating HTML coverage report..."
go tool cover -html=cover.out -o coverage.html

echo "📋 Coverage summary:"
go tool cover -func=cover.out | tail -1

echo ""
echo "✅ Coverage report generated: coverage.html"

# Try to open the report in browser (macOS/Linux)
if command -v open &> /dev/null; then
    echo "🌐 Opening coverage report in browser..."
    open coverage.html
elif command -v xdg-open &> /dev/null; then
    echo "🌐 Opening coverage report in browser..."
    xdg-open coverage.html
else
    echo "💡 To view the report, open coverage.html in your browser"
fi
