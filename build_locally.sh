#!/bin/bash

# Script to test GoReleaser locally in snapshot mode
# This creates a test build without publishing

set -e

echo "Running GoReleaser in snapshot mode..."
echo "This will create test builds without publishing to GitHub."
echo ""

# Check if goreleaser is installed
if ! command -v goreleaser &> /dev/null; then
    echo "GoReleaser is not installed."
    echo "Install it with: go install github.com/goreleaser/goreleaser@latest"
    echo "Or see: https://goreleaser.com/install/"
    exit 1
fi

# Clean previous builds
echo "Cleaning previous builds..."
rm -rf dist/

# Run goreleaser in snapshot mode
echo "Building with GoReleaser..."
goreleaser release --snapshot --clean --skip=publish

echo ""
echo "Extracting binaries with proper names..."

# Get version from git
VERSION=$(git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
if [ -z "$VERSION" ]; then
    VERSION="dev"
fi

# Copy binaries from subdirectories to dist root with proper names
cp dist/windows-amd64_windows_amd64_v1/androidqf.exe "dist/androidqf_windows_amd64_${VERSION}.exe" 2>/dev/null || true
cp dist/linux-amd64_linux_amd64_v1/androidqf "dist/androidqf_linux_amd64_${VERSION}" 2>/dev/null || true
cp dist/linux-arm64_linux_arm64_v8.0/androidqf "dist/androidqf_linux_arm64_${VERSION}" 2>/dev/null || true
cp dist/darwin-universal_darwin_all/androidqf "dist/androidqf_macos_universal_${VERSION}" 2>/dev/null || true

# Remove build subdirectories
rm -rf dist/*_*_*/

# Remove metadata files
rm -f dist/artifacts.json dist/metadata.json dist/config.yaml

echo ""
echo "Build complete! Binaries are in the dist/ directory:"
ls -lh dist/androidqf_* 2>/dev/null || echo "No binaries found"
