package vm

import (
	"bytes"
	"testing"

	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

func TestCancun_MCOPY(t *testing.T) {
	// Bytecode:
	// PUSH32 0x11223344... (32 bytes)
	// PUSH1 0x00
	// MSTORE
	// PUSH1 0x20 (len = 32)
	// PUSH1 0x00 (src = 0)
	// PUSH1 0x20 (dest = 32)
	// MCOPY
	// PUSH1 0x40 (return size = 64)
	// PUSH1 0x00 (return offset = 0)
	// RETURN
	var code []byte
	// PUSH32 (0x7f) + 32 bytes of 0xaa
	code = append(code, byte(PUSH32))
	for i := 0; i < 32; i++ {
		code = append(code, 0xaa)
	}
	code = append(code, byte(PUSH1), 0x00) // offset 0
	code = append(code, byte(MSTORE))

	// MCOPY: dest=32, src=0, len=32 -> stack pops dest, src, len
	code = append(code, byte(PUSH1), 32) // len = 32
	code = append(code, byte(PUSH1), 0)  // src = 0
	code = append(code, byte(PUSH1), 32) // dest = 32
	code = append(code, byte(MCOPY))

	// RETURN: offset=0, len=64
	code = append(code, byte(PUSH1), 64)
	code = append(code, byte(PUSH1), 0)
	code = append(code, byte(RETURN))

	state := NewMemoryStateDB()
	caller, _ := types.HexToAddress("0x1111111111111111111111111111111111111111")
	evm := NewEVM(state, Context{
		Caller:      caller,
		To:          caller,
		Origin:      caller,
		BlockNumber: 1,
	})

	ret, _, err := evm.Execute(code, nil, 100_000)
	if err != nil {
		t.Fatalf("MCOPY execution failed: %v", err)
	}
	if len(ret) != 64 {
		t.Fatalf("expected 64 bytes returned, got %d", len(ret))
	}
	for i := 0; i < 64; i++ {
		if ret[i] != 0xaa {
			t.Fatalf("byte %d mismatch: expected 0xaa, got 0x%x", i, ret[i])
		}
	}
}

func TestCancun_TransientStorage_TSTORE_TLOAD(t *testing.T) {
	// Bytecode:
	// PUSH32 0xbeef (value)
	// PUSH32 0x01 (key)
	// TSTORE
	// PUSH32 0x01 (key)
	// TLOAD
	// PUSH1 0x00
	// MSTORE
	// PUSH1 0x20
	// PUSH1 0x00
	// RETURN
	var code []byte
	code = append(code, byte(PUSH32))
	var valBytes [32]byte
	valBytes[31] = 0xee
	code = append(code, valBytes[:]...)

	code = append(code, byte(PUSH32))
	var keyBytes [32]byte
	keyBytes[31] = 0x01
	code = append(code, keyBytes[:]...)

	code = append(code, byte(TSTORE))

	// TLOAD key 1
	code = append(code, byte(PUSH32))
	code = append(code, keyBytes[:]...)
	code = append(code, byte(TLOAD))

	// MSTORE at offset 0
	code = append(code, byte(PUSH1), 0x00)
	code = append(code, byte(MSTORE))

	// RETURN offset 0, len 32
	code = append(code, byte(PUSH1), 32)
	code = append(code, byte(PUSH1), 0)
	code = append(code, byte(RETURN))

	state := NewMemoryStateDB()
	addr, _ := types.HexToAddress("0x2222222222222222222222222222222222222222")
	evm := NewEVM(state, Context{
		Caller:      addr,
		To:          addr,
		Origin:      addr,
		BlockNumber: 1,
	})

	ret, _, err := evm.Execute(code, nil, 100_000)
	if err != nil {
		t.Fatalf("TSTORE/TLOAD execution failed: %v", err)
	}
	if len(ret) != 32 || ret[31] != 0xee {
		t.Fatalf("TLOAD returned unexpected data: %x", ret)
	}

	// Verify that persistent StateDB has NOT been modified by transient storage
	if state.GetState(addr, types.BytesToHash(keyBytes[:])) != types.EmptyHash() {
		t.Fatalf("transient storage leaked into permanent StateDB!")
	}
}

func TestCancun_BlobOpcodes_SafeNoOp(t *testing.T) {
	// BLOBHASH(index 0) should return 0
	// BLOBBASEFEE should return 1
	var code []byte
	code = append(code, byte(PUSH1), 0) // index
	code = append(code, byte(BLOBHASH))
	code = append(code, byte(PUSH1), 0)
	code = append(code, byte(MSTORE))

	code = append(code, byte(BLOBBASEFEE))
	code = append(code, byte(PUSH1), 32)
	code = append(code, byte(MSTORE))

	code = append(code, byte(PUSH1), 64)
	code = append(code, byte(PUSH1), 0)
	code = append(code, byte(RETURN))

	state := NewMemoryStateDB()
	addr, _ := types.HexToAddress("0x3333333333333333333333333333333333333333")
	evm := NewEVM(state, Context{
		Caller: addr,
		To:     addr,
	})

	ret, _, err := evm.Execute(code, nil, 100_000)
	if err != nil {
		t.Fatalf("BLOB opcode execution failed: %v", err)
	}
	if len(ret) != 64 {
		t.Fatalf("expected 64 bytes returned")
	}
	blobHashPart := ret[:32]
	if !bytes.Equal(blobHashPart, make([]byte, 32)) {
		t.Fatalf("expected 32 zero bytes for BLOBHASH, got %x", blobHashPart)
	}
	blobBaseFeePart := ret[32:]
	if blobBaseFeePart[31] != 1 {
		t.Fatalf("expected 1 for BLOBBASEFEE, got %x", blobBaseFeePart)
	}
}

func TestCancun_BASEFEE_Context(t *testing.T) {
	var code []byte
	code = append(code, byte(BASEFEE))
	code = append(code, byte(PUSH1), 0)
	code = append(code, byte(MSTORE))
	code = append(code, byte(PUSH1), 32)
	code = append(code, byte(PUSH1), 0)
	code = append(code, byte(RETURN))

	state := NewMemoryStateDB()
	addr, _ := types.HexToAddress("0x4444444444444444444444444444444444444444")
	customBaseFee := uint256.NewInt(2_500_000_000) // 2.5 Gwei
	evm := NewEVM(state, Context{
		Caller:  addr,
		To:      addr,
		BaseFee: customBaseFee,
	})

	ret, _, err := evm.Execute(code, nil, 100_000)
	if err != nil {
		t.Fatalf("BASEFEE execution failed: %v", err)
	}
	resInt := new(uint256.Int).SetBytes(ret)
	if !resInt.Eq(customBaseFee) {
		t.Fatalf("expected basefee %s, got %s", customBaseFee.Dec(), resInt.Dec())
	}
}
