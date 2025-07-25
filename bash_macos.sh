#!/bin/bash
set -e

VERSION="2.0.0"
COMMIT=$(git rev-parse --short HEAD)
PLATFORM="macos"
OUT_DIR="bin/$PLATFORM"

echo "🍎 Building Magickline for macOS version $VERSION ($COMMIT)..."

# Temporarily move icon.syso out of the way (Windows only)
if [ -f icon.syso ]; then
  mv icon.syso icon.syso.bak
fi

# Clean and prepare output folder
rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

# Build macOS amd64 binary
echo "🔧 Building macOS amd64 binary..."
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build \
  -trimpath \
  -ldflags="-X main.Version=$VERSION -X main.Commit=$COMMIT" \
  -o "$OUT_DIR/magickline-macos-amd64"

# Build macOS arm64 binary
echo "🔧 Building macOS arm64 binary..."
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build \
  -trimpath \
  -ldflags="-X main.Version=$VERSION -X main.Commit=$COMMIT" \
  -o "$OUT_DIR/magickline-macos-arm64"

# Restore icon.syso if needed
if [ -f icon.syso.bak ]; then
  mv icon.syso.bak icon.syso
fi

# Copy shared runtime assets
cp appsettings.yaml "$OUT_DIR/"
cp -r data "$OUT_DIR/"

# Write version info
echo "Version: $VERSION" > "$OUT_DIR/version.txt"
echo "Commit: $COMMIT" >> "$OUT_DIR/version.txt"

# Create archive from bin/macos/
TAR_NAME="magickline-${PLATFORM}-v${VERSION}.tar.gz"
tar -czf "$TAR_NAME" -C bin "$PLATFORM"

echo "✅ macOS build complete."
echo "📁 Output in: $OUT_DIR"
echo "📦 Archive: $TAR_NAME"
