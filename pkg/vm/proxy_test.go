package vm

import (
	"encoding/hex"
	"testing"

	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

func TestProxy_DELEGATECALL_StorageIsolation(t *testing.T) {
	state := NewMemoryStateDB()
	logicAddr, _ := types.HexToAddress("0x1010101010101010101010101010101010101010")
	proxyAddr, _ := types.HexToAddress("0x2020202020202020202020202020202020202020")
	userAddr, _ := types.HexToAddress("0x3030303030303030303030303030303030303030")

	// Logic Contract Bytecode:
	// CALLDATALOAD(0) -> val
	// PUSH1 0x00 (slot 0)
	// SSTORE
	// PUSH1 0x01 (push 1 to return)
	// PUSH1 0x00
	// MSTORE
	// PUSH1 0x20
	// PUSH1 0x00
	// RETURN
	var logicCode []byte
	logicCode = append(logicCode, byte(PUSH1), 0)
	logicCode = append(logicCode, byte(CALLDATALOAD)) // stack: val
	logicCode = append(logicCode, byte(PUSH1), 0)    // stack: val, 0
	logicCode = append(logicCode, byte(SSTORE))       // storage[0] = val
	logicCode = append(logicCode, byte(PUSH1), 1)
	logicCode = append(logicCode, byte(PUSH1), 0)
	logicCode = append(logicCode, byte(MSTORE))
	logicCode = append(logicCode, byte(PUSH1), 32)
	logicCode = append(logicCode, byte(PUSH1), 0)
	logicCode = append(logicCode, byte(RETURN))

	state.SetCode(logicAddr, logicCode)

	// Proxy Contract Bytecode:
	// Copies all calldata to memory[0]
	// DELEGATECALL(gas=100000, target=logicAddr, inOff=0, inSize=CALLDATASIZE, outOff=0, outSize=32)
	// RETURN(0, RETURNDATASIZE)
	var proxyCode []byte
	proxyCode = append(proxyCode, byte(CALLDATASIZE))
	proxyCode = append(proxyCode, byte(PUSH1), 0)
	proxyCode = append(proxyCode, byte(PUSH1), 0)
	proxyCode = append(proxyCode, byte(CALLDATACOPY)) // memory[0..size] = calldata

	// DELEGATECALL stack: gas, to, inOff, inSize, outOff, outSize
	proxyCode = append(proxyCode, byte(PUSH1), 32)             // outSize
	proxyCode = append(proxyCode, byte(PUSH1), 0)              // outOff
	proxyCode = append(proxyCode, byte(CALLDATASIZE))          // inSize
	proxyCode = append(proxyCode, byte(PUSH1), 0)              // inOff
	proxyCode = append(proxyCode, byte(PUSH32))
	var logicPadded [32]byte
	copy(logicPadded[12:], logicAddr[:])
	proxyCode = append(proxyCode, logicPadded[:]...)           // to = logicAddr
	proxyCode = append(proxyCode, byte(GAS))                   // gas
	proxyCode = append(proxyCode, byte(DELEGATECALL))

	// Return returndata
	proxyCode = append(proxyCode, byte(RETURNDATASIZE))
	proxyCode = append(proxyCode, byte(PUSH1), 0)
	proxyCode = append(proxyCode, byte(RETURN))

	state.SetCode(proxyAddr, proxyCode)

	// User calls Proxy with calldata = 0x9999...99
	evm := NewEVM(state, Context{
		Caller:      userAddr,
		To:          proxyAddr,
		Origin:      userAddr,
		BlockNumber: 1,
	})

	inputVal := uint256.NewInt(999999)
	inputData := inputVal.Bytes32()

	ret, _, err := evm.Execute(proxyCode, inputData[:], 1_000_000)
	if err != nil {
		t.Fatalf("proxy call failed: %v", err)
	}
	if len(ret) != 32 || ret[31] != 1 {
		t.Fatalf("expected return 1 from logic, got %x", ret)
	}

	// CRITICAL VERIFICATION:
	// Storage slot 0 in PROXY must be set to 999999!
	var slot0 types.Hash
	proxyVal := state.GetState(proxyAddr, slot0)
	proxyValInt := new(uint256.Int).SetBytes32(proxyVal.Bytes())
	if !proxyValInt.Eq(inputVal) {
		t.Fatalf("proxy storage slot 0 mismatch: expected %s, got %s", inputVal.Dec(), proxyValInt.Dec())
	}

	// Storage slot 0 in LOGIC must be EMPTY (isolation guarantee)!
	logicVal := state.GetState(logicAddr, slot0)
	if logicVal != types.EmptyHash() {
		t.Fatalf("logic contract storage was contaminated! Expected empty, got %x", logicVal)
	}
}

func TestProxy_EIP1167_MinimalProxy(t *testing.T) {
	state := NewMemoryStateDB()
	targetAddr, _ := types.HexToAddress("0x9999999999999999999999999999999999999999")
	userAddr, _ := types.HexToAddress("0x4444444444444444444444444444444444444444")
	cloneAddr, _ := types.HexToAddress("0x5555555555555555555555555555555555555555")

	// Target Contract: returns 0x12345678 in 32 bytes
	var targetCode []byte
	targetCode = append(targetCode, byte(PUSH32))
	var targetVal [32]byte
	targetVal[28] = 0x12
	targetVal[29] = 0x34
	targetVal[30] = 0x56
	targetVal[31] = 0x78
	targetCode = append(targetCode, targetVal[:]...)
	targetCode = append(targetCode, byte(PUSH1), 0)
	targetCode = append(targetCode, byte(MSTORE))
	targetCode = append(targetCode, byte(PUSH1), 32)
	targetCode = append(targetCode, byte(PUSH1), 0)
	targetCode = append(targetCode, byte(RETURN))

	state.SetCode(targetAddr, targetCode)

	// Standard EIP-1167 Minimal Proxy Bytecode (45 bytes):
	// 363d3d373d3d3d363d73 <20-byte target> 5af43d82803e903d91602b57fd5bf3
	prefix, _ := hex.DecodeString("363d3d373d3d3d363d73")
	suffix, _ := hex.DecodeString("5af43d82803e903d91602b57fd5bf3")
	cloneCode := append(prefix, targetAddr[:]...)
	cloneCode = append(cloneCode, suffix...)

	if len(cloneCode) != 45 {
		t.Fatalf("expected 45 bytes clone bytecode, got %d", len(cloneCode))
	}
	state.SetCode(cloneAddr, cloneCode)

	evm := NewEVM(state, Context{
		Caller:      userAddr,
		To:          cloneAddr,
		Origin:      userAddr,
		BlockNumber: 1,
	})

	ret, _, err := evm.Execute(cloneCode, nil, 100_000)
	if err != nil {
		t.Fatalf("clone execution failed: %v", err)
	}
	if len(ret) != 32 || ret[31] != 0x78 || ret[28] != 0x12 {
		t.Fatalf("unexpected clone return: %x", ret)
	}
}
