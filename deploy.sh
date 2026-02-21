#!/bin/bash
set -e

REMOTE_ALIAS="racknerd"
REMOTE_DIR="~/workdesk/mantis"
BINARY_NAME="mantis"

echo "🔨 Building Mantis (AMD64)..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $BINARY_NAME main.go

echo "📤 Deploying to $REMOTE_ALIAS..."

ssh $REMOTE_ALIAS "mkdir -p $REMOTE_DIR"
scp $BINARY_NAME config.yaml $REMOTE_ALIAS:$REMOTE_DIR/

echo "✅ Success! To run:"
echo "ssh $REMOTE_ALIAS 'cd $REMOTE_DIR && ./$BINARY_NAME'"