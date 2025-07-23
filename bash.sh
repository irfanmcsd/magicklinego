#!/bin/bash

VERSION="2.0.0"
COMMIT=$(git rev-parse --short HEAD)

echo "Building Magickline version $VERSION ($COMMIT)..."

mkdir -p bin

# Build for macOS (Intel + ARM)
GOOS=darwin GOARCH=amd64 go build -ldflags="-X main.Version=$VERSION -X main.Commit=$COMMIT" -o bin/magickline-macos
GOOS=darwin GOARCH=arm64 go build -ldflags="-X main.Version=$VERSION -X main.Commit=$COMMIT" -o bin/magickline-macos-arm64

# Build for Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-X main.Version=$VERSION -X main.Commit=$COMMIT" -o bin/magickline-linux

# Build for Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-X main.Version=$VERSION -X main.Commit=$COMMIT" -o bin/magickline-win.exe

# Copy config and data files
cp appsettings.yaml bin/
cp -r data bin/

# Optional: create version info
echo "Version: $VERSION" > bin/version.txt
echo "Commit: $COMMIT" >> bin/version.txt

echo "✅ Build complete. Files are in /bin"