#!/usr/bin/env bash
# ==============================================================================
# CHORD Chain & ChordScan VPS Automated Production Deployment Script
# Target: Forge VPS (linux/amd64, 76.13.184.200)
# ==============================================================================
set -euo pipefail

# Support passing host as 1st argument or via FORGE_VPS env, fallback to forge-vps
TARGET_INPUT="${1:-${FORGE_VPS:-}}"
DEFAULT_IP="root@76.13.184.200"

if [ -n "$TARGET_INPUT" ]; then
    HOST="$TARGET_INPUT"
else
    # Try forge-vps first, fallback to verified IP if unresolvable
    if ssh -o ConnectTimeout=3 -q -G "forge-vps" > /dev/null 2>&1 && ssh -o ConnectTimeout=3 "forge-vps" "true" 2>/dev/null; then
        HOST="forge-vps"
    else
        HOST="$DEFAULT_IP"
    fi
fi

REMOTE_DIR="${REMOTE_CHORD_DIR:-/opt/chord}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHORD_GO_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
CHORDSCAN_DIR="$(cd "$CHORD_GO_DIR/../chordscan" && pwd)"

echo "=================================================================="
echo "🚀 CHORD Network & ChordScan Production Deployment"
echo "Target Host : $HOST"
echo "Remote Path : $REMOTE_DIR"
echo "Domain      : chordchain.space (RPC: rpc.chordchain.space)"
echo "Architecture: linux/amd64 (4-Node BFT Round-Robin)"
echo "=================================================================="

# 1. SSH Connectivity Check
echo "🔍 [1/6] Checking SSH connection to $HOST..."
if ! VPS_ARCH="$(ssh -o ConnectTimeout=8 "$HOST" "uname -m" 2>&1)"; then
    echo "❌ SSH connection to $HOST failed. Details:"
    echo "$VPS_ARCH"
    echo ""
    echo "💡 How to fix:"
    echo "  1) If your VPS username/IP is different, pass it directly:"
    echo "     ./deploy.sh <username>@<vps-ip>"
    echo "     예: ./deploy.sh root@76.13.184.200"
    echo ""
    echo "  2) Or add 'forge-vps' to your ~/.ssh/config:"
    echo "     Host forge-vps"
    echo "       HostName 76.13.184.200"
    echo "       User root"
    exit 1
fi
VPS_ARCH="$(echo "$VPS_ARCH" | tr -d '[:space:]')"
echo "✅ SSH connection verified. Target Machine Architecture: $VPS_ARCH"

# 2. Verify / Build Static AMD64 Binaries
echo "🔨 [2/6] Verifying static linux/amd64 binaries..."
mkdir -p "$CHORD_GO_DIR/bin/linux/amd64"
if [ ! -f "$CHORD_GO_DIR/bin/linux/amd64/chordd" ] || [ ! -f "$CHORD_GO_DIR/bin/linux/amd64/chordcli" ]; then
    echo "📦 Building static linux/amd64 binaries..."
    (cd "$CHORD_GO_DIR" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/linux/amd64/chordd ./cmd/chordd)
    (cd "$CHORD_GO_DIR" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/linux/amd64/chordcli ./cmd/chordcli)
fi
cp -f "$CHORD_GO_DIR/bin/linux/amd64/chordd" "$CHORD_GO_DIR/bin/linux/chordd" 2>/dev/null || true
cp -f "$CHORD_GO_DIR/bin/linux/amd64/chordcli" "$CHORD_GO_DIR/bin/linux/chordcli" 2>/dev/null || true
echo "✅ linux/amd64 binaries verified:"
file "$CHORD_GO_DIR/bin/linux/amd64/chordd"

# 3. Create Remote Directories
echo "📁 [3/6] Initializing remote directories on $HOST..."
ssh "$HOST" "mkdir -p $REMOTE_DIR/chord-go $REMOTE_DIR/chordscan"

# 4. Synchronize Sources
echo "🔄 [4/6] Synchronizing chord-go and chordscan source repositories..."
rsync -az --delete \
    --exclude '.git' \
    --exclude '.turbo' \
    --exclude 'tmp' \
    "$CHORD_GO_DIR/" "$HOST:$REMOTE_DIR/chord-go/"

rsync -az --delete \
    --exclude '.git' \
    --exclude 'node_modules' \
    --exclude '.next' \
    --exclude '.turbo' \
    --exclude 'dist' \
    "$CHORDSCAN_DIR/" "$HOST:$REMOTE_DIR/chordscan/"

# 5. Remote Environment & Docker Compose Up
echo "🐳 [5/6] Building containers and starting production stack on $HOST..."
ssh "$HOST" "
    set -eu
    cd $REMOTE_DIR/chord-go/deploy/prod
    if [ ! -f .env ]; then
        cp .env.prod.example .env
    fi
    docker compose -f docker-compose.prod.yml pull || true
    docker compose -f docker-compose.prod.yml up -d --build
    docker network connect chord-production_chord-prod-net freddie-forge-prod-caddy-1 2>/dev/null || true
"

# 6. Verify Deployment Health
echo "🩺 [6/6] Verifying service health..."
ssh "$HOST" "cd $REMOTE_DIR/chord-go/deploy/prod && docker compose -f docker-compose.prod.yml ps"

echo ""
echo "=================================================================="
echo "🎉 CHORD Network & ChordScan deployed successfully to $HOST!"
echo "• Explorer UI : https://chordchain.space"
echo "• Sub-Explorer: https://scan.chordchain.space"
echo "• EVM RPC     : https://rpc.chordchain.space"
echo "=================================================================="
