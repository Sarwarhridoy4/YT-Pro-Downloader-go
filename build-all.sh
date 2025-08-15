#!/bin/bash
# ==============================================
#  Go Cross-Platform Build Script (Linux/macOS)
#  Author: Sarwar Hossain
# ==============================================

# Exit immediately if a command exits with a non-zero status
set -e

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "[ERROR] Go is not installed or not in PATH."
    echo "Download Go from https://go.dev/dl"
    exit 1
fi

# Variables
GOFILE="app.go"
OUTNAME="YT-Pro-Downloader"
BUILD_DIR="build"

# Create build folder
mkdir -p "$BUILD_DIR"

echo "🚀 Starting cross-platform build..."

# ====== Windows 64-bit ======
echo "Building Windows 64-bit..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$BUILD_DIR/${OUTNAME}-windows-amd64.exe" "$GOFILE"

# ====== Windows 32-bit ======
echo "Building Windows 32-bit..."
GOOS=windows GOARCH=386 go build -ldflags="-s -w" -o "$BUILD_DIR/${OUTNAME}-windows-386.exe" "$GOFILE"

# ====== Linux 64-bit ======
echo "Building Linux 64-bit..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$BUILD_DIR/${OUTNAME}-linux-amd64" "$GOFILE"

# ====== Linux 32-bit ======
echo "Building Linux 32-bit..."
GOOS=linux GOARCH=386 go build -ldflags="-s -w" -o "$BUILD_DIR/${OUTNAME}-linux-386" "$GOFILE"

# ====== macOS (Darwin) 64-bit ======
echo "Building macOS 64-bit..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o "$BUILD_DIR/${OUTNAME}-darwin-amd64" "$GOFILE"

# ====== macOS ARM64 (Apple Silicon) ======
echo "Building macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o "$BUILD_DIR/${OUTNAME}-darwin-arm64" "$GOFILE"

echo
echo "✅ All builds completed! Files are in the '$BUILD_DIR' folder."
