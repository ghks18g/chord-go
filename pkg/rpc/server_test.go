package rpc

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ghks18g/chord-go/pkg/app"
	"github.com/ghks18g/chord-go/pkg/bft"
	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/ghks18g/chord-go/pkg/vm"
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

func TestRPC_EthCall_RIP7212Precompile(t *testing.T) {
	appl := app.NewApp("chord_7613-1")
	engine := bft.NewEngine(nil, false)
	ts := NewTestServer(appl, engine)
	defer ts.Close()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	hash := sha256.Sum256([]byte("rpc-test-rip7212"))
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	input := make([]byte, 160)
	copy(input[0:32], hash[:])
	r.FillBytes(input[32:64])
	s.FillBytes(input[64:96])
	priv.PublicKey.X.FillBytes(input[96:128])
	priv.PublicKey.Y.FillBytes(input[128:160])

	// 1. Valid signature -> returns 0x00...01
	reqBody, _ := json.Marshal(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_call",
		Params: []interface{}{
			map[string]interface{}{
				"to":   "0x0000000000000000000000000000000000000100",
				"data": "0x" + hex.EncodeToString(input),
			},
			"latest",
		},
		ID: 1,
	})
	resp, err := http.Post(ts.URL+"/", "application/json", bytes.NewBuffer(reqBody))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("eth_call request failed: %v", err)
	}
	var rpcResp JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&rpcResp)
	expected := "0x0000000000000000000000000000000000000000000000000000000000000001"
	if rpcResp.Result != expected {
		t.Fatalf("expected %s, got %v", expected, rpcResp.Result)
	}

	// 2. Tampered signature -> returns "0x"
	input[0] ^= 0xff
	reqBody, _ = json.Marshal(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_call",
		Params: []interface{}{
			map[string]interface{}{
				"to":   "0x0000000000000000000000000000000000000100",
				"data": "0x" + hex.EncodeToString(input),
			},
			"latest",
		},
		ID: 2,
	})
	resp, _ = http.Post(ts.URL+"/", "application/json", bytes.NewBuffer(reqBody))
	var rpcRespBad JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&rpcRespBad)
	if rpcRespBad.Result != "0x" {
		t.Fatalf("expected '0x' for invalid signature, got %v", rpcRespBad.Result)
	}
}

func TestRPC_EthSendTransaction_WithOnChainLogs(t *testing.T) {
	appl := app.NewApp("chord_7613-1")
	engine := bft.NewEngine(nil, false)
	ts := NewTestServer(appl, engine)
	defer ts.Close()

	alice, _ := types.HexToAddress("0x1111111111111111111111111111111111111111")
	appl.SetBalance(alice, uint256.NewInt(1000000))

	// Deploy a contract that emits LOG1
	topicHex := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	topicBytes, _ := hex.DecodeString(topicHex[2:])

	contractCode := []byte{
		byte(vm.PUSH1), 0x2a,
		byte(vm.PUSH1), 0x00,
		byte(vm.MSTORE),
		byte(vm.PUSH32),
	}
	contractCode = append(contractCode, topicBytes...)
	contractCode = append(contractCode,
		byte(vm.PUSH1), 0x20, // size 32
		byte(vm.PUSH1), 0x00, // offset 0
		byte(vm.LOG1),
		byte(vm.STOP),
	)

	contractAddr, err := appl.DeployContract(alice, contractCode, uint256.NewInt(0))
	if err != nil {
		t.Fatalf("failed to deploy contract: %v", err)
	}

	// Send tx to contract
	sendTxBody, _ := json.Marshal(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_sendTransaction",
		Params: []interface{}{
			map[string]interface{}{
				"from":  alice.String(),
				"to":    contractAddr.String(),
				"value": "0x0",
				"data":  "0x",
			},
		},
		ID: 1,
	})
	resp, err := http.Post(ts.URL+"/", "application/json", bytes.NewBuffer(sendTxBody))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("eth_sendTransaction failed: %v", err)
	}
	var sendResp JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&sendResp)
	txHashStr, ok := sendResp.Result.(string)
	if !ok || txHashStr == "" {
		t.Fatalf("expected non-empty tx hash, got %v", sendResp.Result)
	}

	// Query receipt
	receiptBody, _ := json.Marshal(JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "eth_getTransactionReceipt",
		Params:  []interface{}{txHashStr},
		ID:      2,
	})
	resp, _ = http.Post(ts.URL+"/", "application/json", bytes.NewBuffer(receiptBody))
	var receiptResp struct {
		Result TxReceipt `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&receiptResp)

	if len(receiptResp.Result.Logs) != 1 {
		t.Fatalf("expected 1 log in receipt, got %d", len(receiptResp.Result.Logs))
	}
	log := receiptResp.Result.Logs[0]
	if len(log.Topics) != 1 || log.Topics[0] != topicHex {
		t.Fatalf("expected topic %s, got %v", topicHex, log.Topics)
	}
}
