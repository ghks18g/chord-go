package vm

import (
	"bytes"
	"testing"

	"github.com/ghks18g/chord-go/pkg/types"

	"github.com/holiman/uint256"
)

func TestStackOperations(t *testing.T) {
	s := NewStack()
	if s.Len() != 0 {
		t.Fatalf("expected len 0, got %d", s.Len())
	}

	val1 := uint256.NewInt(42)
	val2 := uint256.NewInt(100)

	if err := s.Push(val1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := s.Push(val2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Len() != 2 {
		t.Fatalf("expected len 2, got %d", s.Len())
	}

	// Peek
	peekVal, err := s.Peek()
	if err != nil || !peekVal.Eq(val2) {
		t.Fatalf("peek mismatch: %v, %v", peekVal, err)
	}

	// Dup1
	if err := s.Dup(1); err != nil {
		t.Fatalf("dup error: %v", err)
	}
	if s.Len() != 3 {
		t.Fatalf("expected len 3, got %d", s.Len())
	}

	// Swap1
	if err := s.Push(uint256.NewInt(1)); err != nil {
		t.Fatalf("push error: %v", err)
	}
	if err := s.Swap(1); err != nil {
		t.Fatalf("swap error: %v", err)
	}

	// Pop all
	for s.Len() > 0 {
		if _, err := s.Pop(); err != nil {
			t.Fatalf("pop error: %v", err)
		}
	}

	// Underflow test
	if _, err := s.Pop(); err != ErrStackUnderflow {
		t.Fatalf("expected underflow, got %v", err)
	}
}

func TestMemoryOperations(t *testing.T) {
	m := NewMemory()
	val := uint256.NewInt(0x12345678)
	m.Set32(10, val)

	res := m.GetCopy(10, 32)
	expected := val.Bytes32()
	if !bytes.Equal(res, expected[:]) {
		t.Fatalf("memory mismatch: expected %x, got %x", expected, res)
	}
}

func TestEVM_AddAndReturn(t *testing.T) {
	// Bytecode:
	// PUSH1 0x05 (60 05)
	// PUSH1 0x03 (60 03)
	// ADD        (01) -> 8
	// PUSH1 0x00 (60 00)
	// MSTORE     (52) -> mem[0:32] = 8
	// PUSH1 0x20 (60 20) -> size 32
	// PUSH1 0x00 (60 00) -> offset 0
	// RETURN     (f3)
	code := []byte{
		0x60, 0x05,
		0x60, 0x03,
		0x01,
		0x60, 0x00,
		0x52,
		0x60, 0x20,
		0x60, 0x00,
		0xf3,
	}

	stateDB := NewMemoryStateDB()
	evm := NewEVM(stateDB, Context{})

	ret, gasLeft, err := evm.Execute(code, nil, 100000)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if gasLeft >= 100000 {
		t.Fatalf("gas was not deducted: %d", gasLeft)
	}

	if len(ret) != 32 {
		t.Fatalf("expected 32 bytes return, got %d", len(ret))
	}

	resInt := new(uint256.Int).SetBytes(ret)
	if resInt.Uint64() != 8 {
		t.Fatalf("expected return value 8, got %d", resInt.Uint64())
	}
}

func TestEVM_StorageOperations(t *testing.T) {
	// Bytecode:
	// PUSH1 0x99 (val)
	// PUSH1 0x01 (key)
	// SSTORE
	// PUSH1 0x01 (key)
	// SLOAD
	// PUSH1 0x00 (offset)
	// MSTORE
	// PUSH1 0x20
	// PUSH1 0x00
	// RETURN
	code := []byte{
		0x60, 0x99,
		0x60, 0x01,
		0x55,
		0x60, 0x01,
		0x54,
		0x60, 0x00,
		0x52,
		0x60, 0x20,
		0x60, 0x00,
		0xf3,
	}

	to := types.BytesToAddress([]byte{0x01, 0x02, 0x03})
	stateDB := NewMemoryStateDB()
	evm := NewEVM(stateDB, Context{To: to})

	ret, _, err := evm.Execute(code, nil, 100000)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	resInt := new(uint256.Int).SetBytes(ret)
	if resInt.Uint64() != 0x99 {
		t.Fatalf("expected return value 0x99, got 0x%x", resInt.Uint64())
	}

	// Verify state in stateDB directly
	var keySlot types.Hash
	keySlot[31] = 0x01
	valHash := stateDB.GetState(to, keySlot)
	var expectedHash types.Hash
	expectedHash[31] = 0x99
	if valHash != expectedHash {
		t.Fatalf("expected state %x, got %x", expectedHash, valHash)
	}
}

func TestEVM_JumpAndControlFlow(t *testing.T) {
	// Bytecode:
	// PUSH1 0x07 (dest)
	// JUMP
	// STOP (skipped)
	// PUSH1 0x11 (skipped)
	// JUMPDEST (pc 7)
	// PUSH1 0x42
	// PUSH1 0x00
	// MSTORE
	// PUSH1 0x20
	// PUSH1 0x00
	// RETURN
	code := []byte{
		0x60, 0x07, // pc 0, 1
		0x56,       // pc 2 (JUMP)
		0x00,       // pc 3 (STOP)
		0x60, 0x11, // pc 4, 5 (PUSH1 0x11)
		0x00,       // pc 6 (STOP)
		0x5b,       // pc 7 (JUMPDEST)
		0x60, 0x42, // pc 8, 9
		0x60, 0x00, // pc 10, 11
		0x52,       // pc 12 (MSTORE)
		0x60, 0x20, // pc 13, 14
		0x60, 0x00, // pc 15, 16
		0xf3,       // pc 17 (RETURN)
	}

	stateDB := NewMemoryStateDB()
	evm := NewEVM(stateDB, Context{})

	ret, _, err := evm.Execute(code, nil, 100000)
	if err != nil {
		t.Fatalf("jump execute failed: %v", err)
	}

	res := new(uint256.Int).SetBytes(ret)
	if res.Uint64() != 0x42 {
		t.Fatalf("expected 0x42, got 0x%x", res.Uint64())
	}
}

func TestEVM_Revert(t *testing.T) {
	// Bytecode:
	// PUSH1 0xaa
	// PUSH1 0x00
	// MSTORE
	// PUSH1 0x20
	// PUSH1 0x00
	// REVERT
	code := []byte{
		0x60, 0xaa,
		0x60, 0x00,
		0x52,
		0x60, 0x20,
		0x60, 0x00,
		0xfd,
	}

	stateDB := NewMemoryStateDB()
	evm := NewEVM(stateDB, Context{})

	ret, _, err := evm.Execute(code, nil, 100000)
	if err != ErrExecutionReverted {
		t.Fatalf("expected ErrExecutionReverted, got %v", err)
	}
	res := new(uint256.Int).SetBytes(ret)
	if res.Uint64() != 0xaa {
		t.Fatalf("expected revert data 0xaa, got 0x%x", res.Uint64())
	}
}

func TestEVM_OutOfGas(t *testing.T) {
	code := []byte{
		0x60, 0x01,
		0x60, 0x02,
		0x01,
	}
	stateDB := NewMemoryStateDB()
	evm := NewEVM(stateDB, Context{})

	// Very low gas limit
	_, _, err := evm.Execute(code, nil, 2)
	if err != ErrOutOfGas {
		t.Fatalf("expected ErrOutOfGas, got %v", err)
	}
}
