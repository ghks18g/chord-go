#!/usr/bin/env bash
# CHORD E2E Integration Test Script
set -e

RPC_URL="${1:-http://127.0.0.1:8545}"
FAUCET_ADDR="0x7613761376137613761376137613761376137613"

echo "=========================================================="
echo " Running CHORD E2E Tests on $RPC_URL"
echo "=========================================================="

echo "1. Checking CometBFT Node Status (/status)..."
STATUS=$(curl -s "$RPC_URL/status")
echo "$STATUS" | grep -q "chord_7613-1" || { echo "❌ Status check failed"; exit 1; }
echo "   ✅ Status OK: Network is chord_7613-1"

echo "2. Querying EVM eth_chainId (JSON-RPC)..."
CHAIN_ID_RESP=$(curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' "$RPC_URL")
echo "$CHAIN_ID_RESP" | grep -q "0x1dbd" || { echo "❌ eth_chainId check failed (expected 0x1dbd / 7613)"; exit 1; }
echo "   ✅ eth_chainId OK: 0x1dbd (Decimal: 7613)"

echo "3. Querying Faucet Balance (eth_getBalance)..."
BAL_RESP=$(curl -s -X POST -H "Content-Type: application/json" --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"$FAUCET_ADDR\"],\"id\":2}" "$RPC_URL")
echo "$BAL_RESP" | grep -q "result" || { echo "❌ Balance query failed"; exit 1; }
echo "   ✅ Faucet Balance OK: $(echo "$BAL_RESP" | grep -o '"result":"[^"]*"')"

echo "4. Querying ZK Poseidon SMT Account Proof (/prove_account)..."
PROOF_RESP=$(curl -s "$RPC_URL/prove_account?address=$FAUCET_ADDR")
echo "$PROOF_RESP" | grep -q "siblings" || { echo "❌ SMT proof query failed"; exit 1; }
STATE_ROOT=$(echo "$PROOF_RESP" | grep -o '"state_root":"[^"]*"' | cut -d'"' -f4)
echo "   ✅ ZK SMT Proof OK: State Root = $STATE_ROOT"

echo "=========================================================="
echo " 🎉 All E2E Integration Checks Passed Successfully!"
echo "=========================================================="
