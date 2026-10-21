package vm

import (
	"bytes"
	"testing"

	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

// TestLog0_Basic verifies that LOG0 emits memory data with no topics.
func TestLog0_Basic(t *testing.T) {
	stateDB := NewMemoryStateDB()
	addr := types.HexToAddressMust("0x1234567890123456789012345678901234567890")

	// Bytecode:
	// 1. PUSH32 0xdeadbeef... -> MSTORE at 0x00
	// 2. LOG0(offset=0, size=32)
	var code []byte
	var dataHash types.Hash
	dataHash[0] = 0xde
	dataHash[1] = 0xad
	dataHash[2] = 0xbe
	dataHash[3] = 0xef

	code = append(code, pushHash(dataHash)...)
	code = append(code, pushUint64(0)...)
	code = append(code, byte(MSTORE))

	// LOG0 stack: [offset, size]
	code = append(code, pushUint64(32)...) // size
	code = append(code, pushUint64(0)...)  // offset
	code = append(code, byte(LOG0))
	code = append(code, byte(STOP))

	evm := NewEVM(stateDB, Context{
		To:          addr,
		BlockNumber: 42,
	})

	_, _, err := evm.Execute(code, nil, 100000)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	logs := stateDB.GetLogs()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}

	l := logs[0]
	if l.Address != addr {
		t.Fatalf("expected log address %s, got %s", addr.Hex(), l.Address.Hex())
	}
	if len(l.Topics) != 0 {
		t.Fatalf("expected 0 topics for LOG0, got %d", len(l.Topics))
	}
	if l.BlockNumber != 42 {
		t.Fatalf("expected block number 42, got %d", l.BlockNumber)
	}
	if !bytes.Equal(l.Data, dataHash[:]) {
		t.Fatalf("expected log data %x, got %x", dataHash[:], l.Data)
	}
}

// TestLog1To4_TopicsAndData verifies that LOG1 through LOG4 store 1 to 4 topics and payload.
func TestLog1To4_TopicsAndData(t *testing.T) {
	for nTopics := 1; nTopics <= 4; nTopics++ {
		stateDB := NewMemoryStateDB()
		addr := types.HexToAddressMust("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")

		// Prepare N topics
		topics := make([]types.Hash, nTopics)
		for i := 0; i < nTopics; i++ {
			topics[i][31] = byte(i + 1)
		}

		// Prepare memory data (16 bytes)
		payload := []byte("chord-log-event!")

		var code []byte
		// MSTORE at offset 0
		var word [32]byte
		copy(word[:], payload)
		code = append(code, pushHash(word)...)
		code = append(code, pushUint64(0)...)
		code = append(code, byte(MSTORE))

		// Push topics in reverse order so topics[0] is popped first
		for i := nTopics - 1; i >= 0; i-- {
			code = append(code, pushHash(topics[i])...)
		}

		// Push size and offset
		code = append(code, pushUint64(16)...) // size = 16 bytes
		code = append(code, pushUint64(0)...)  // offset = 0

		logOp := OpCode(byte(LOG0) + byte(nTopics))
		code = append(code, byte(logOp))
		code = append(code, byte(STOP))

		evm := NewEVM(stateDB, Context{
			To:          addr,
			BlockNumber: 100,
		})

		_, _, err := evm.Execute(code, nil, 100000)
		if err != nil {
			t.Fatalf("execute LOG%d failed: %v", nTopics, err)
		}

		logs := stateDB.GetLogs()
		if len(logs) != 1 {
			t.Fatalf("LOG%d: expected 1 log, got %d", nTopics, len(logs))
		}

		l := logs[0]
		if l.Address != addr {
			t.Fatalf("LOG%d: expected address %s, got %s", nTopics, addr.Hex(), l.Address.Hex())
		}
		if len(l.Topics) != nTopics {
			t.Fatalf("LOG%d: expected %d topics, got %d", nTopics, nTopics, len(l.Topics))
		}
		for i := 0; i < nTopics; i++ {
			if l.Topics[i] != topics[i] {
				t.Fatalf("LOG%d: topic %d mismatch: expected %x, got %x", nTopics, i, topics[i], l.Topics[i])
			}
		}
		if !bytes.Equal(l.Data, payload) {
			t.Fatalf("LOG%d: payload mismatch: expected %s, got %s", nTopics, string(payload), string(l.Data))
		}
	}
}

// TestLog_StaticWriteProtection verifies that emitting a log inside a static context returns ErrWriteProtection.
func TestLog_StaticWriteProtection(t *testing.T) {
	stateDB := NewMemoryStateDB()
	addr := types.HexToAddressMust("0x1111111111111111111111111111111111111111")

	var topic types.Hash
	topic[31] = 0x01

	var code []byte
	code = append(code, pushHash(topic)...)
	code = append(code, pushUint64(0)...) // size
	code = append(code, pushUint64(0)...) // offset
	code = append(code, byte(LOG1))
	code = append(code, byte(STOP))

	evm := NewEVM(stateDB, Context{
		To: addr,
	})
	evm.isStatic = true // In static context

	_, _, err := evm.Execute(code, nil, 50000)
	if err == nil || err != ErrWriteProtection {
		t.Fatalf("expected ErrWriteProtection when logging in static mode, got %v", err)
	}

	if len(stateDB.GetLogs()) != 0 {
		t.Fatalf("no logs should be emitted upon write protection failure")
	}
}

// Suppress uint256 unused import
var _ = uint256.NewInt
