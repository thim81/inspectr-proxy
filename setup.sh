#!/bin/bash

# Set some helpful variables
PROJECT_NAME="inspectr"  # Replace with your project name
FRONTEND_DIR="frontend"
APP_DIR="app"

# Check if Go is installed
if ! command -v go &> /dev/null; then
  echo "Error: Go is not installed. Please install Go from https://go.dev/dl/"
  exit 1
fi

# Check if npm is installed
if ! command -v npm &> /dev/null; then
  echo "Error: npm is not installed. Please install Node.js (which includes npm)."
  exit 1
fi

# Run go mod tidy and vendor
echo "Running go mod tidy and vendor..."
go mod tidy
go mod vendor

# Create dist directory if it doesn't exist
mkdir -p "$FRONTEND_DIR"
rm -Rf "$APP_DIR"

# Run npm install and build
echo "Running npm install..."
cp package.json "$FRONTEND_DIR/package.json"
cd "$FRONTEND_DIR"
npm install
mv "node_modules/@inspectr/app/dist" "../app"
cd ..

rm -Rf "$FRONTEND_DIR"

# Build the Go binary
echo "Building Go binary..."
go build -o "$PROJECT_NAME"

echo "Build complete!  Run with ./$PROJECT_NAME" # Updated message

# Optional: Run tests
# go test ./...

# Optional: Add versioning information to the binary
# go build -ldflags "-X main.version=$(git describe --tags)" -o "$PROJECT_NAME"