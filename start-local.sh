#!/bin/bash
# Start nsm locally for testing

set -e

cd "$(dirname "$0")"

go mod tidy
go build

echo "Starting nexSign mini..."


# Set environment variables
export PORT=8080

# Check if port is already in use
if lsof -Pi :$PORT -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    echo ""
    echo "⚠️  Port $PORT is already in use!"
    echo ""
    
    # Get process information
    PID=$(lsof -Pi :$PORT -sTCP:LISTEN -t)
    PROCESS_INFO=$(ps -p $PID -o pid,comm,args | tail -n 1)
    
    echo "Process using port $PORT:"
    echo "  PID: $PID"
    echo "  $PROCESS_INFO"
    echo ""
    
    # Ask user if they want to terminate the process
    read -p "Do you want to terminate this process? (y/N): " -n 1 -r
    echo ""
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Terminating process $PID..."
        kill $PID
        
        # Wait a moment for the process to terminate
        sleep 1
        
        # Check if process is still running
        if ps -p $PID > /dev/null 2>&1; then
            echo "Process didn't terminate gracefully, forcing kill..."
            kill -9 $PID
            sleep 1
        fi
        
        echo "✓ Process terminated successfully"
        echo ""
    else
        echo "Cancelled. Port $PORT is still in use."
        exit 1
    fi
fi

# Run the application
go run -ldflags="-X 'nexsign.mini/nsm/internal/types.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" main.go
