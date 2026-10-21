package vm

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"testing"

	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

// Helper to create a 160-byte RIP-7212 payload
func makeRIP7212Input(hash []byte, r, s, x, y *big.Int) []byte {
	input := make([]byte, 160)
	copy(input[0:32], hash)
	r.FillBytes(input[32:64])
	s.FillBytes(input[64:96])
	x.FillBytes(input[96:128])
	y.FillBytes(input[128:160])
	return input
}

// TestRIP7212_ValidSignature verifies that a valid NIST P-256 signature returns 32-byte 0x00...01.
func TestRIP7212_ValidSignature(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate P-256 key: %v", err)
	}

	msg := []byte("CHORD protocol secp256r1 RIP-7212 test vector")
	hash := sha256.Sum256(msg)

	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("failed to sign message: %v", err)
	}

	input := makeRIP7212Input(hash[:], r, s, priv.PublicKey.X, priv.PublicKey.Y)

	precompile := &RIP7212Precompile{}

	// Verify required gas is 3,450
	if precompile.RequiredGas(input) != 3450 {
		t.Fatalf("expected required gas 3450, got %d", precompile.RequiredGas(input))
	}

	out, err := precompile.Run(input)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	expected := make([]byte, 32)
	expected[31] = 1

	if !bytes.Equal(out, expected) {
		t.Fatalf("expected output 0x00...01, got %x", out)
	}
}

// TestRIP7212_InvalidSignatures verifies that invalid signatures, tampered data, or bad coordinates return an empty slice.
func TestRIP7212_InvalidSignatures(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	msg := []byte("CHORD protocol test")
	hash := sha256.Sum256(msg)

	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	precompile := &RIP7212Precompile{}

	// 1. Tampered message hash
	tamperedHash := hash
	tamperedHash[0] ^= 0xff
	inputTamperedHash := makeRIP7212Input(tamperedHash[:], r, s, priv.PublicKey.X, priv.PublicKey.Y)
	out, err := precompile.Run(inputTamperedHash)
	if err != nil || len(out) != 0 {
		t.Fatalf("tampered hash should return empty slice, got: %x, err: %v", out, err)
	}

	// 2. Tampered signature (r + 1)
	badR := new(big.Int).Add(r, big.NewInt(1))
	inputTamperedR := makeRIP7212Input(hash[:], badR, s, priv.PublicKey.X, priv.PublicKey.Y)
	out, err = precompile.Run(inputTamperedR)
	if err != nil || len(out) != 0 {
		t.Fatalf("tampered r should return empty slice, got: %x", out)
	}

	// 3. Point not on curve
	badX := big.NewInt(12345)
	badY := big.NewInt(67890)
	inputOffCurve := makeRIP7212Input(hash[:], r, s, badX, badY)
	out, err = precompile.Run(inputOffCurve)
	if err != nil || len(out) != 0 {
		t.Fatalf("off-curve point should return empty slice, got: %x", out)
	}

	// 4. Zero point (0, 0)
	zeroPointInput := makeRIP7212Input(hash[:], r, s, big.NewInt(0), big.NewInt(0))
	out, err = precompile.Run(zeroPointInput)
	if err != nil || len(out) != 0 {
		t.Fatalf("zero point should return empty slice, got: %x", out)
	}

	// 5. Short input (< 160 bytes)
	out, err = precompile.Run([]byte{0x01, 0x02})
	if err != nil || len(out) != 0 {
		t.Fatalf("short input should return empty slice, got: %x", out)
	}
}

// TestRIP7212_StaticCallE2E verifies that a smart contract can invoke 0x100 via STATICCALL and receive verified result.
func TestRIP7212_StaticCallE2E(t *testing.T) {
	stateDB := NewMemoryStateDB()
	callerAddr := types.HexToAddressMust("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	p256PrecompileAddr := types.HexToAddressMust("0x0000000000000000000000000000000000000100")

	// Generate valid P-256 signature
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	hash := sha256.Sum256([]byte("e2e-staticcall-p256-verify"))
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	payload := makeRIP7212Input(hash[:], r, s, priv.PublicKey.X, priv.PublicKey.Y)

	// Contract Bytecode:
	// 1. Copy calldata to memory[0..160] using CALLDATACOPY(memOff=0, dataOff=0, len=160)
	// 2. STATICCALL(gas=10000, to=0x100, inOff=0, inSize=160, outOff=160, outSize=32)
	// 3. MLOAD(160) -> load 32-byte precompile result
	// 4. SSTORE(key=0x01, val=result)
	var code []byte

	// CALLDATACOPY: [destOffset, offset, length] -> pops destOffset, then offset, then length
	code = append(code, pushUint64(160)...) // length
	code = append(code, pushUint64(0)...)   // offset
	code = append(code, pushUint64(0)...)   // destOffset
	code = append(code, byte(CALLDATACOPY))

	// STATICCALL: [gas, to, inOff, inSize, outOff, outSize]
	code = append(code, pushUint64(32)...)                 // outSize
	code = append(code, pushUint64(160)...)                // outOff
	code = append(code, pushUint64(160)...)                // inSize
	code = append(code, pushUint64(0)...)                  // inOff
	code = append(code, pushAddress(p256PrecompileAddr)...) // to (0x100)
	code = append(code, pushUint64(10000)...)              // gas
	code = append(code, byte(STATICCALL))

	// Stack now has subcall status (1 on success). Store status in storage key 0x00
	code = append(code, pushUint64(0x00)...) // key
	code = append(code, byte(SSTORE))

	// MLOAD(160) -> load verification output (uint256(1))
	code = append(code, pushUint64(160)...) // offset
	code = append(code, byte(MLOAD))

	// Store output in storage key 0x01
	code = append(code, pushUint64(0x01)...) // key
	code = append(code, byte(SSTORE))
	code = append(code, byte(STOP))

	evm := NewEVM(stateDB, Context{
		Caller: callerAddr,
		To:     callerAddr,
		Origin: callerAddr,
	})

	_, _, err = evm.Execute(code, payload, 200000)
	if err != nil {
		t.Fatalf("contract execution failed: %v", err)
	}

	// 1. Check subcall success status at key 0x00: should be 1
	var statusKey types.Hash
	statusVal := stateDB.GetState(callerAddr, statusKey)
	var expectedStatus types.Hash
	expectedStatus[31] = 1
	if statusVal != expectedStatus {
		t.Fatalf("expected STATICCALL success status 1, got %x", statusVal)
	}

	// 2. Check precompile verification result at key 0x01: should be 1
	var resultKey types.Hash
	resultKey[31] = 0x01
	resultVal := stateDB.GetState(callerAddr, resultKey)
	var expectedResult types.Hash
	expectedResult[31] = 1
	if resultVal != expectedResult {
		t.Fatalf("expected precompile result 1, got %x", resultVal)
	}
}

// Suppress uint256 unused import
var _ = uint256.NewInt
