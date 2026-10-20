#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "=================================================================="
echo " [CHORD] Complete Ledger Reset & Genesis Template Setup"
echo "=================================================================="

# 1. Stop existing nodes
echo -e "\n[1/5] Stopping CHORD validator nodes..."
cd "${ROOT_DIR}"
docker-compose -f docker/docker-compose.yml down || true

# 2. Compile latest chordd & chordcli for linux/arm64
echo -e "\n[2/5] Building latest static binaries for linux/arm64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/linux/chordd ./cmd/chordd
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/linux/chordcli ./cmd/chordcli

# 3. Start fresh 4-node cluster
echo -e "\n[3/5] Starting fresh 4-node BFT cluster (Clean Genesis Block #0)..."
docker-compose -f docker/docker-compose.yml up -d --build

# Wait for RPC to become ready
echo "Waiting for CHORD RPC (http://127.0.0.1:8545) to respond..."
until curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://127.0.0.1:8545 | grep -q "result"; do
    sleep 0.5
done
echo " -> CHORD Node RPC is ready at http://127.0.0.1:8545"

# 4. Truncate Indexer Database
echo -e "\n[4/5] Resetting ChordScan PostgreSQL Indexer database..."
if docker ps --format '{{.Names}}' | grep -q "^chordscan-postgres$"; then
  docker exec -i chordscan-postgres psql -U chord -d chordscan -c \
    "TRUNCATE TABLE transactions, blocks, contracts, contract_verifications, address_tags CASCADE;" > /dev/null 2>&1 || true
  echo " -> PostgreSQL tables truncated (Clean state)."
else
  echo " -> Note: chordscan-postgres container not running, skipping db truncate."
fi

# 5. Deploy MultiSig, Faucet, and Fund
echo -e "\n[5/5] Deploying Genesis Governance & Faucet Contracts..."
node "${SCRIPT_DIR}/setup-genesis-contracts.js"

echo -e "\n=================================================================="
echo " [SUCCESS] CHORD Ledger is freshly initialized and seeded!"
echo " Explorer: http://localhost:3001"
echo " RPC:      http://127.0.0.1:8545"
echo "=================================================================="
