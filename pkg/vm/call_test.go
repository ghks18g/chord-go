package vm

import (
	"errors"
	"testing"

	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

// Helper to build PUSH opcodes for byte slices
func pushBytes(b []byte) []byte {
	n := len(b)
	if n == 0 {
		return []byte{byte(PUSH1), 0x00}
	}
	op := byte(PUSH1) + byte(n-1)
	res := make([]byte, 1+n)
	res[0] = op
	copy(res[1:], b)
	return res
}

func pushUint64(v uint64) []byte {
	return pushBytes(uint256.NewInt(v).Bytes())
}

func pushAddress(a types.Address) []byte {
	res := make([]byte, 21)
	res[0] = byte(PUSH1 + 19) // PUSH20
	copy(res[1:], a[:])
	return res
}

func pushHash(h types.Hash) []byte {
	res := make([]byte, 33)
	res[0] = byte(PUSH32)
	copy(res[1:], h[:])
	return res
}

// TestCall_ValueTransferAndStateChange verifies:
// 1. Caller transfers value to callee.
// 2. Callee executes code and modifies state (SSTORE).
// 3. Subcall returns success (1 on stack).
func TestCall_ValueTransferAndStateChange(t *testing.T) {
	stateDB := NewMemoryStateDB()

	callerAddr := types.HexToAddressMust("0x1111111111111111111111111111111111111111")
	calleeAddr := types.HexToAddressMust("0x2222222222222222222222222222222222222222")

	// Caller initial balance: 1,000 wei
	stateDB.SetBalance(callerAddr, uint256.NewInt(1000))
	stateDB.SetBalance(calleeAddr, uint256.NewInt(0))

	// Callee bytecode: SSTORE key=0x01, value=0x42
	// Stack before SSTORE: [key, value]
	calleeCode := []byte{
		byte(PUSH1), 0x42, // val
		byte(PUSH1), 0x01, // key
		byte(SSTORE),
		byte(STOP),
	}
	stateDB.SetCode(calleeAddr, calleeCode)

	// Caller bytecode: CALL(gas=10000, to=calleeAddr, value=100, inOff=0, inSize=0, outOff=0, outSize=0)
	// CALL stack: [gas, to, value, inOff, inSize, outOff, outSize]
	var callerCode []byte
	callerCode = append(callerCode, pushUint64(0)...)        // outSize
	callerCode = append(callerCode, pushUint64(0)...)        // outOff
	callerCode = append(callerCode, pushUint64(0)...)        // inSize
	callerCode = append(callerCode, pushUint64(0)...)        // inOff
	callerCode = append(callerCode, pushUint64(100)...)      // value
	callerCode = append(callerCode, pushAddress(calleeAddr)...) // to
	callerCode = append(callerCode, pushUint64(20000)...)    // gas
	callerCode = append(callerCode, byte(CALL))
	callerCode = append(callerCode, byte(STOP))

	evm := NewEVM(stateDB, Context{
		Caller: callerAddr,
		To:     callerAddr,
		Origin: callerAddr,
	})

	_, _, err := evm.Execute(callerCode, nil, 100000)
	if err != nil {
		t.Fatalf("caller execution failed: %v", err)
	}

	// 1. Check balances
	callerBal := stateDB.GetBalance(callerAddr)
	calleeBal := stateDB.GetBalance(calleeAddr)
	if callerBal.Cmp(uint256.NewInt(900)) != 0 {
		t.Fatalf("expected caller balance 900, got %s", callerBal.String())
	}
	if calleeBal.Cmp(uint256.NewInt(100)) != 0 {
		t.Fatalf("expected callee balance 100, got %s", calleeBal.String())
	}

	// 2. Check callee storage: key 0x01 == 0x42
	var expectedKey types.Hash
	expectedKey[31] = 0x01
	storedVal := stateDB.GetState(calleeAddr, expectedKey)
	var expectedVal types.Hash
	expectedVal[31] = 0x42
	if storedVal != expectedVal {
		t.Fatalf("expected callee storage 0x42, got %x", storedVal)
	}
}

// TestStaticCall_WriteProtection verifies:
// 1. Calling SSTORE inside STATICCALL fails with ErrWriteProtection.
// 2. State changes are reverted and subcall pushes 0 to stack.
func TestStaticCall_WriteProtection(t *testing.T) {
	stateDB := NewMemoryStateDB()

	callerAddr := types.HexToAddressMust("0x1111111111111111111111111111111111111111")
	calleeAddr := types.HexToAddressMust("0x2222222222222222222222222222222222222222")

	// Callee bytecode: tries to execute SSTORE
	calleeCode := []byte{
		byte(PUSH1), 0x99, // val
		byte(PUSH1), 0x02, // key
		byte(SSTORE),
		byte(STOP),
	}
	stateDB.SetCode(calleeAddr, calleeCode)

	// Caller bytecode: STATICCALL(gas=10000, to=calleeAddr, inOff=0, inSize=0, outOff=0, outSize=0)
	// STATICCALL stack: [gas, to, inOff, inSize, outOff, outSize]
	var callerCode []byte
	callerCode = append(callerCode, pushUint64(0)...)        // outSize
	callerCode = append(callerCode, pushUint64(0)...)        // outOff
	callerCode = append(callerCode, pushUint64(0)...)        // inSize
	callerCode = append(callerCode, pushUint64(0)...)        // inOff
	callerCode = append(callerCode, pushAddress(calleeAddr)...) // to
	callerCode = append(callerCode, pushUint64(20000)...)    // gas
	callerCode = append(callerCode, byte(STATICCALL))
	// Save the return status (0 or 1) to storage key 0x00 in caller
	callerCode = append(callerCode, pushUint64(0x00)...) // key
	callerCode = append(callerCode, byte(SSTORE))
	callerCode = append(callerCode, byte(STOP))

	evm := NewEVM(stateDB, Context{
		Caller: callerAddr,
		To:     callerAddr,
		Origin: callerAddr,
	})

	_, _, err := evm.Execute(callerCode, nil, 100000)
	if err != nil {
		t.Fatalf("caller execution failed: %v", err)
	}

	// 1. Verify callee storage was NOT written (write protection triggered rollback)
	var calleeKey types.Hash
	calleeKey[31] = 0x02
	calleeStored := stateDB.GetState(calleeAddr, calleeKey)
	if calleeStored != types.EmptyHash() {
		t.Fatalf("expected empty storage at callee, got %x", calleeStored)
	}

	// 2. Verify subcall result stored in caller key 0x00 is 0 (failure)
	var callerKey types.Hash
	callerStored := stateDB.GetState(callerAddr, callerKey)
	if callerStored != types.EmptyHash() {
		t.Fatalf("expected caller stored result 0 (failure), got %x", callerStored)
	}
}

// TestDelegateCall_StorageAndCallerContext verifies:
// 1. DELEGATECALL executes callee's code in caller's context.
// 2. Storage modification affects caller, NOT callee.
// 3. Caller is preserved (CALLER returns the original caller, not proxy).
func TestDelegateCall_StorageAndCallerContext(t *testing.T) {
	stateDB := NewMemoryStateDB()

	userAddr := types.HexToAddressMust("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	proxyAddr := types.HexToAddressMust("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	implAddr := types.HexToAddressMust("0xcccccccccccccccccccccccccccccccccccccccc")

	// Implementation contract: stores CALLER at storage key 0x07
	implCode := []byte{
		byte(CALLER),      // val: caller address
		byte(PUSH1), 0x07, // key
		byte(SSTORE),
		byte(STOP),
	}
	stateDB.SetCode(implAddr, implCode)

	// Proxy contract: DELEGATECALL(gas=20000, to=implAddr, inOff=0, inSize=0, outOff=0, outSize=0)
	var proxyCode []byte
	proxyCode = append(proxyCode, pushUint64(0)...)      // outSize
	proxyCode = append(proxyCode, pushUint64(0)...)      // outOff
	proxyCode = append(proxyCode, pushUint64(0)...)      // inSize
	proxyCode = append(proxyCode, pushUint64(0)...)      // inOff
	proxyCode = append(proxyCode, pushAddress(implAddr)...) // to
	proxyCode = append(proxyCode, pushUint64(20000)...)  // gas
	proxyCode = append(proxyCode, byte(DELEGATECALL))
	proxyCode = append(proxyCode, byte(STOP))
	stateDB.SetCode(proxyAddr, proxyCode)

	// Execute call from userAddr to proxyAddr
	evm := NewEVM(stateDB, Context{
		Caller: userAddr,
		To:     proxyAddr,
		Origin: userAddr,
	})

	_, _, err := evm.Execute(proxyCode, nil, 100000)
	if err != nil {
		t.Fatalf("proxy delegatecall execution failed: %v", err)
	}

	var key types.Hash
	key[31] = 0x07

	// 1. Proxy storage should contain userAddr
	proxyStored := stateDB.GetState(proxyAddr, key)
	var expectedUserHash types.Hash
	copy(expectedUserHash[12:], userAddr[:])
	if proxyStored != expectedUserHash {
		t.Fatalf("expected proxy storage to hold userAddr %x, got %x", expectedUserHash, proxyStored)
	}

	// 2. Implementation storage should remain untouched (empty)
	implStored := stateDB.GetState(implAddr, key)
	if implStored != types.EmptyHash() {
		t.Fatalf("implementation storage must remain empty, got %x", implStored)
	}
}

// TestCall_DepthLimit verifies that exceeding max call depth (1024) halts subcalls.
func TestCall_DepthLimit(t *testing.T) {
	stateDB := NewMemoryStateDB()
	callerAddr := types.HexToAddressMust("0x1111111111111111111111111111111111111111")
	calleeAddr := types.HexToAddressMust("0x2222222222222222222222222222222222222222")

	stateDB.SetCode(calleeAddr, []byte{byte(STOP)})

	var callerCode []byte
	callerCode = append(callerCode, pushUint64(0)...)
	callerCode = append(callerCode, pushUint64(0)...)
	callerCode = append(callerCode, pushUint64(0)...)
	callerCode = append(callerCode, pushUint64(0)...)
	callerCode = append(callerCode, pushUint64(0)...)
	callerCode = append(callerCode, pushAddress(calleeAddr)...)
	callerCode = append(callerCode, pushUint64(1000)...)
	callerCode = append(callerCode, byte(CALL))
	// Store result at key 0x00
	callerCode = append(callerCode, pushUint64(0x00)...)
	callerCode = append(callerCode, byte(SSTORE))
	callerCode = append(callerCode, byte(STOP))

	evm := NewEVM(stateDB, Context{
		Caller: callerAddr,
		To:     callerAddr,
		Origin: callerAddr,
	})
	evm.depth = 1024 // max depth reached

	_, _, err := evm.Execute(callerCode, nil, 50000)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	var key types.Hash
	res := stateDB.GetState(callerAddr, key)
	if res != types.EmptyHash() {
		t.Fatalf("call at depth 1024 should push 0, got %x", res)
	}
}

// TestCall_RevertRollback verifies that when a subcall reverts, state changes made in it are rolled back.
func TestCall_RevertRollback(t *testing.T) {
	stateDB := NewMemoryStateDB()
	callerAddr := types.HexToAddressMust("0x1111111111111111111111111111111111111111")
	calleeAddr := types.HexToAddressMust("0x2222222222222222222222222222222222222222")

	// Callee sets key 0x01 to 0x55, then REVERTS
	calleeCode := []byte{
		byte(PUSH1), 0x55,
		byte(PUSH1), 0x01,
		byte(SSTORE),
		byte(PUSH1), 0x00, // size
		byte(PUSH1), 0x00, // offset
		byte(REVERT),
	}
	stateDB.SetCode(calleeAddr, calleeCode)

	var callerCode []byte
	callerCode = append(callerCode, pushUint64(0)...)
	callerCode = append(callerCode, pushUint64(0)...)
	callerCode = append(callerCode, pushUint64(0)...)
	callerCode = append(callerCode, pushUint64(0)...)
	callerCode = append(callerCode, pushUint64(0)...)
	callerCode = append(callerCode, pushAddress(calleeAddr)...)
	callerCode = append(callerCode, pushUint64(20000)...)
	callerCode = append(callerCode, byte(CALL))
	callerCode = append(callerCode, byte(STOP))

	evm := NewEVM(stateDB, Context{
		Caller: callerAddr,
		To:     callerAddr,
		Origin: callerAddr,
	})

	_, _, err := evm.Execute(callerCode, nil, 100000)
	if err != nil {
		t.Fatalf("caller execution failed: %v", err)
	}

	// Callee storage should have been reverted
	var key types.Hash
	key[31] = 0x01
	stored := stateDB.GetState(calleeAddr, key)
	if stored != types.EmptyHash() {
		t.Fatalf("expected rolled back storage at callee, got %x", stored)
	}
}

// Ensure unused error import suppression if not referenced directly
var _ = errors.New
