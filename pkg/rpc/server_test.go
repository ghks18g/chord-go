package rpc

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ghks18g/chord-go/pkg/app"
	"github.com/ghks18g/chord-go/pkg/bft"
	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

func TestRPCServer(t *testing.T) {
	appl := app.NewApp("chord_7613-1")
	engine := bft.NewEngine(nil, false)
	ts := NewTestServer(appl, engine)
	defer ts.Close()

	// 1. Status endpoint
	resp, err := http.Get(ts.URL + "/status")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to fetch /status")
	}

	// 2. EVM JSON-RPC: eth_chainId
	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_chainId",
		ID:      1,
	}
	body, _ := json.Marshal(rpcReq)
	resp, err = http.Post(ts.URL+"/", "application/json", bytes.NewBuffer(body))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("eth_chainId failed")
	}

	var rpcResp JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&rpcResp)
	if rpcResp.Result != "0x1dbd" { // 7613 in hex
		t.Fatalf("expected chainId 0x1dbd, got: %v", rpcResp.Result)
	}

	// 3. SMT ZK Proof Query: /prove_account
	alice, _ := types.HexToAddress("0x1111111111111111111111111111111111111111")
	appl.SetBalance(alice, uint256.NewInt(1000))

	resp, err = http.Get(ts.URL + "/prove_account?address=" + alice.String())
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("/prove_account failed")
	}

	var proofResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&proofResult)
	if proofResult["address"] != alice.String() {
		t.Fatalf("address mismatch in proof result")
	}
	if proofResult["state_root"] == "" {
		t.Fatalf("expected non-empty state_root in proof result")
	}
}
