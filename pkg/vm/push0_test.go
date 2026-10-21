package vm

import (
	"testing"

	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

func TestEVM_PUSH0(t *testing.T) {
	state := NewMemoryStateDB()
	addr, _ := types.HexToAddress("0x1111111111111111111111111111111111111111")

	evm := NewEVM(state, Context{
		Caller:      addr,
		To:          addr,
		Origin:      addr,
		BlockNumber: 1,
		Value:       uint256.NewInt(0),
	})

	// Bytecode: PUSH0 (0x5f), PUSH1 0x05, ADD, PUSH0, MSTORE, PUSH1 0x20, PUSH0, RETURN
	// This pushes 0, 5, adds them -> 5, stores 5 at memory 0, and returns 32 bytes from offset 0
	code := []byte{
		byte(PUSH0),
		byte(PUSH1), 0x05,
		byte(ADD),
		byte(PUSH0),
		byte(MSTORE),
		byte(PUSH1), 0x20,
		byte(PUSH0),
		byte(RETURN),
	}

	ret, gasLeft, err := evm.Execute(code, nil, 100000)
	if err != nil {
		t.Fatalf("unexpected error executing PUSH0: %v", err)
	}

	if len(ret) != 32 {
		t.Fatalf("expected return length 32, got %d", len(ret))
	}

	val := new(uint256.Int).SetBytes(ret)
	if val.Uint64() != 5 {
		t.Fatalf("expected return value 5, got %d", val.Uint64())
	}

	if gasLeft >= 100000 {
		t.Fatalf("gas was not consumed, gasLeft: %d", gasLeft)
	}
}
