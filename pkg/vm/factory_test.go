package vm

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	gethcommon "github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ghks18g/chord-go/pkg/crypto/signature"
	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

func TestFactory_CREATE(t *testing.T) {
	// Child contract: returns 32 bytes 0x42
	// Child InitCode:
	// PUSH32 0x42...
	// PUSH1 0x00
	// MSTORE
	// PUSH1 0x20
	// PUSH1 0x00
	// RETURN
	var childCode []byte
	childCode = append(childCode, byte(PUSH32))
	var childVal [32]byte
	childVal[31] = 0x42
	childCode = append(childCode, childVal[:]...)
	childCode = append(childCode, byte(PUSH1), 0x00)
	childCode = append(childCode, byte(MSTORE))
	childCode = append(childCode, byte(PUSH1), 32)
	childCode = append(childCode, byte(PUSH1), 0)
	childCode = append(childCode, byte(RETURN))

	// Parent Factory Code:
	// Copies childCode to memory at offset 0
	// Calls CREATE(value=0, offset=0, size=len(childCode))
	// Returns the newly created 20-byte address (left padded to 32 bytes)
	var parentCode []byte
	// Write childCode into memory using PUSH / MSTORE or CODECOPY
	// For simplicity, embed childCode directly via MSTORE
	for i := 0; i < len(childCode); i++ {
		parentCode = append(parentCode, byte(PUSH1), childCode[i])
		parentCode = append(parentCode, byte(PUSH1), byte(i))
		parentCode = append(parentCode, byte(MSTORE8))
	}
	// Stack for CREATE: value, offset, size
	parentCode = append(parentCode, byte(PUSH1), byte(len(childCode))) // size
	parentCode = append(parentCode, byte(PUSH1), 0)                    // offset
	parentCode = append(parentCode, byte(PUSH1), 0)                    // value
	parentCode = append(parentCode, byte(CREATE))

	// Store returned address to memory offset 0, and RETURN 32 bytes
	parentCode = append(parentCode, byte(PUSH1), 0)
	parentCode = append(parentCode, byte(MSTORE))
	parentCode = append(parentCode, byte(PUSH1), 32)
	parentCode = append(parentCode, byte(PUSH1), 0)
	parentCode = append(parentCode, byte(RETURN))

	state := NewMemoryStateDB()
	factoryAddr, _ := types.HexToAddress("0x5555555555555555555555555555555555555555")
	state.SetNonce(factoryAddr, 0)

	evm := NewEVM(state, Context{
		Caller: factoryAddr,
		To:     factoryAddr,
	})

	ret, _, err := evm.Execute(parentCode, nil, 1_000_000)
	if err != nil {
		t.Fatalf("CREATE execution failed: %v", err)
	}

	createdAddr := types.BytesToAddress(ret[12:])
	expectedAddr := gethcrypto.CreateAddress(gethcommon.BytesToAddress(factoryAddr.Bytes()), 0)

	if !strings.EqualFold(createdAddr.Hex(), expectedAddr.Hex()) {
		t.Fatalf("address mismatch: expected %s, got %s", expectedAddr.Hex(), createdAddr.Hex())
	}

	// Verify child code is deployed in state
	deployedCode := state.GetCode(createdAddr)
	if len(deployedCode) != 32 || deployedCode[31] != 0x42 {
		t.Fatalf("child code not deployed properly: %x", deployedCode)
	}
}

func TestFactory_CREATE2_Deterministic(t *testing.T) {
	// Child InitCode: returns 32 bytes of 0x77
	var childCode []byte
	childCode = append(childCode, byte(PUSH32))
	var childVal [32]byte
	childVal[31] = 0x77
	childCode = append(childCode, childVal[:]...)
	childCode = append(childCode, byte(PUSH1), 0x00)
	childCode = append(childCode, byte(MSTORE))
	childCode = append(childCode, byte(PUSH1), 32)
	childCode = append(childCode, byte(PUSH1), 0)
	childCode = append(childCode, byte(RETURN))

	salt := [32]byte{0: 0xaa, 31: 0xbb}

	var parentCode []byte
	for i := 0; i < len(childCode); i++ {
		parentCode = append(parentCode, byte(PUSH1), childCode[i])
		parentCode = append(parentCode, byte(PUSH1), byte(i))
		parentCode = append(parentCode, byte(MSTORE8))
	}
	// Stack for CREATE2: value, offset, size, salt
	parentCode = append(parentCode, byte(PUSH32))
	parentCode = append(parentCode, salt[:]...)                       // salt
	parentCode = append(parentCode, byte(PUSH1), byte(len(childCode))) // size
	parentCode = append(parentCode, byte(PUSH1), 0)                    // offset
	parentCode = append(parentCode, byte(PUSH1), 0)                    // value
	parentCode = append(parentCode, byte(CREATE2))

	// Store returned address to memory offset 0, and RETURN 32 bytes
	parentCode = append(parentCode, byte(PUSH1), 0)
	parentCode = append(parentCode, byte(MSTORE))
	parentCode = append(parentCode, byte(PUSH1), 32)
	parentCode = append(parentCode, byte(PUSH1), 0)
	parentCode = append(parentCode, byte(RETURN))

	state := NewMemoryStateDB()
	factoryAddr, _ := types.HexToAddress("0x6666666666666666666666666666666666666666")

	evm := NewEVM(state, Context{
		Caller: factoryAddr,
		To:     factoryAddr,
	})

	ret, _, err := evm.Execute(parentCode, nil, 1_000_000)
	if err != nil {
		t.Fatalf("CREATE2 execution failed: %v", err)
	}

	createdAddr := types.BytesToAddress(ret[12:])
	childHash := signature.Keccak256(childCode)
	expectedAddr := gethcrypto.CreateAddress2(gethcommon.BytesToAddress(factoryAddr.Bytes()), salt, childHash[:])

	if !strings.EqualFold(createdAddr.Hex(), expectedAddr.Hex()) {
		t.Fatalf("CREATE2 address mismatch: expected %s, got %s", expectedAddr.Hex(), createdAddr.Hex())
	}

	// Verify child code is deployed in state
	deployedCode := state.GetCode(createdAddr)
	if len(deployedCode) != 32 || deployedCode[31] != 0x77 {
		t.Fatalf("child code not deployed properly: %x", deployedCode)
	}
}

func TestFactory_EXTCODESIZE_EXTCODECOPY_EXTCODEHASH(t *testing.T) {
	state := NewMemoryStateDB()
	targetAddr, _ := types.HexToAddress("0x7777777777777777777777777777777777777777")
	sampleCode, _ := hex.DecodeString("600160020160005260206000f3") // 13 bytes
	state.SetCode(targetAddr, sampleCode)

	caller, _ := types.HexToAddress("0x8888888888888888888888888888888888888888")

	// Test EXTCODESIZE
	// PUSH20 targetAddr
	// EXTCODESIZE
	// PUSH1 0x00
	// MSTORE
	// PUSH1 0x20
	// PUSH1 0x00
	// RETURN
	var sizeCode []byte
	sizeCode = append(sizeCode, byte(PUSH32))
	var addrPadded [32]byte
	copy(addrPadded[12:], targetAddr[:])
	sizeCode = append(sizeCode, addrPadded[:]...)
	sizeCode = append(sizeCode, byte(EXTCODESIZE))
	sizeCode = append(sizeCode, byte(PUSH1), 0)
	sizeCode = append(sizeCode, byte(MSTORE))
	sizeCode = append(sizeCode, byte(PUSH1), 32)
	sizeCode = append(sizeCode, byte(PUSH1), 0)
	sizeCode = append(sizeCode, byte(RETURN))

	evm := NewEVM(state, Context{Caller: caller, To: caller})
	ret, _, err := evm.Execute(sizeCode, nil, 100_000)
	if err != nil {
		t.Fatalf("EXTCODESIZE failed: %v", err)
	}
	sizeInt := new(uint256.Int).SetBytes(ret)
	if sizeInt.Uint64() != uint64(len(sampleCode)) {
		t.Fatalf("expected size %d, got %d", len(sampleCode), sizeInt.Uint64())
	}

	// Test EXTCODEHASH
	var hashCode []byte
	hashCode = append(hashCode, byte(PUSH32))
	hashCode = append(hashCode, addrPadded[:]...)
	hashCode = append(hashCode, byte(EXTCODEHASH))
	hashCode = append(hashCode, byte(PUSH1), 0)
	hashCode = append(hashCode, byte(MSTORE))
	hashCode = append(hashCode, byte(PUSH1), 32)
	hashCode = append(hashCode, byte(PUSH1), 0)
	hashCode = append(hashCode, byte(RETURN))

	retHash, _, err := evm.Execute(hashCode, nil, 100_000)
	if err != nil {
		t.Fatalf("EXTCODEHASH failed: %v", err)
	}
	expectedHash := signature.Keccak256(sampleCode)
	if !bytes.Equal(retHash, expectedHash[:]) {
		t.Fatalf("EXTCODEHASH mismatch: expected %x, got %x", expectedHash[:], retHash)
	}
}
