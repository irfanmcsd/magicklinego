#!/bin/bash
set -e

VERSION="2.0.0"
COMMIT=$(git rev-parse --short HEAD)
PLATFORM="linux"
OUT_DIR="bin/$PLATFORM"

echo "🐧 Building Magickline for Linux version $VERSION ($COMMIT)..."

# Clean and prepare platform-specific output folder
rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

# Build the binary into /bin/linux/
GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags="-X main.Version=$VERSION -X main.Commit=$COMMIT" \
  -o "./$OUT_DIR/magickline-linux"

# Copy config and assets
cp appsettings.yaml "$OUT_DIR/"
cp -r data "$OUT_DIR/"

# Write version info
echo "Version: $VERSION" > "$OUT_DIR/version.txt"
echo "Commit: $COMMIT" >> "$OUT_DIR/version.txt"

# Create archive from bin/linux/
TAR_NAME="magickline-${PLATFORM}-v${VERSION}.tar.gz"
tar -czf "$TAR_NAME" -C bin "$PLATFORM"

echo "✅ Linux build complete."
echo "📁 Output in: $OUT_DIR"
echo "📦 Archive: $TAR_NAME"
