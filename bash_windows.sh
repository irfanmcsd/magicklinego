#!/bin/bash
set -e

VERSION="2.0.0"
COMMIT=$(git rev-parse --short HEAD)
PLATFORM="windows"
OUT_DIR="bin/$PLATFORM"

echo "🚀 Building Magickline for Windows version $VERSION ($COMMIT)..."

# Clean and prepare platform-specific output folder
rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"
rm -f icon.syso

# Compile icon resource
echo "🖼  Compiling Windows icon..."
windres icon.rc -O coff -o icon.syso

# Build the binary into /bin/windows/
echo "🪟 Building for Windows..."
export CGO_ENABLED=0
GOOS=windows GOARCH=amd64 go build \
  -ldflags="-H=windowsgui -X main.Version=$VERSION -X main.Commit=$COMMIT" \
  -o "./$OUT_DIR/magickline-win.exe"

# Copy config and assets
cp appsettings.yaml "$OUT_DIR/"
cp -r data "$OUT_DIR/"

# Write version info
echo "Version: $VERSION" > "$OUT_DIR/version.txt"
echo "Commit: $COMMIT" >> "$OUT_DIR/version.txt"

# Create archive from bin/windows/
TAR_NAME="magickline-${PLATFORM}-v${VERSION}.tar.gz"
tar -czf "$TAR_NAME" -C bin "$PLATFORM"

echo "✅ Windows build complete."
echo "📁 Output in: $OUT_DIR"
echo "📦 Archive: $TAR_NAME"