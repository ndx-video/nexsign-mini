#!/bin/bash
# Start nsm locally for testing

set -e

cd "$(dirname "$0")"

echo "Starting nexSign mini..."

# Check if hosts.json exists, create if not
if [ ! -f hosts.json ]; then
    echo "Creating hosts.json..."
    echo "[]" > hosts.json
fi

# Set environment variables
export PORT=8080

# Run the application
go run main.go
