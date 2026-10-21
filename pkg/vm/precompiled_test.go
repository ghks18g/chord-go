package vm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"strings"
	"testing"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ghks18g/chord-go/pkg/crypto/poseidon"
	"github.com/ghks18g/chord-go/pkg/crypto/signature"
	"github.com/ghks18g/chord-go/pkg/types"
	"golang.org/x/crypto/ripemd160"
)

func TestPrecompiled_ECRecover_0x1(t *testing.T) {
	privKey, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	expectedAddr := gethcrypto.PubkeyToAddress(privKey.PublicKey)

	msg := []byte("CHORD L1 Native ECRecover Test")
	hash := signature.Keccak256(msg)

	sig, err := gethcrypto.Sign(hash[:], privKey)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}
	// sig is [R || S || V] where V is 0 or 1. Precompile 0x1 expects V as 27 or 28 in byte 63
	v := sig[64] + 27

	// Input: hash[32] || v[32] || r[32] || s[32]
	input := make([]byte, 128)
	copy(input[0:32], hash[:])
	input[63] = v
	copy(input[64:96], sig[0:32])
	copy(input[96:128], sig[32:64])

	p, ok := PrecompiledContracts[types.BytesToAddress([]byte{1})]
	if !ok {
		t.Fatalf("precompile 0x1 not found in registry")
	}

	res, err := p.Run(input)
	if err != nil {
		t.Fatalf("ecrecover failed: %v", err)
	}
	if len(res) != 32 {
		t.Fatalf("expected 32 bytes result, got %d", len(res))
	}

	recoveredAddr := types.BytesToAddress(res[12:])
	if !strings.EqualFold(recoveredAddr.Hex(), expectedAddr.Hex()) {
		t.Fatalf("address mismatch: expected %s, got %s", expectedAddr.Hex(), recoveredAddr.Hex())
	}
}

func TestPrecompiled_SHA256_0x2(t *testing.T) {
	data := []byte("hello CHORD sha256")
	expected := sha256.Sum256(data)

	p := PrecompiledContracts[types.BytesToAddress([]byte{2})]
	res, err := p.Run(data)
	if err != nil {
		t.Fatalf("sha256 failed: %v", err)
	}
	if string(res) != string(expected[:]) {
		t.Fatalf("sha256 mismatch")
	}
}

func TestPrecompiled_RIPEMD160_0x3(t *testing.T) {
	data := []byte("hello CHORD ripemd160")
	hasher := ripemd160.New()
	hasher.Write(data)
	expected := hasher.Sum(nil)

	p := PrecompiledContracts[types.BytesToAddress([]byte{3})]
	res, err := p.Run(data)
	if err != nil {
		t.Fatalf("ripemd160 failed: %v", err)
	}
	if len(res) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(res))
	}
	if string(res[12:]) != string(expected) {
		t.Fatalf("ripemd160 mismatch")
	}
}

func TestPrecompiled_Identity_0x4(t *testing.T) {
	data := []byte("verbatim identity data for chord evm")
	p := PrecompiledContracts[types.BytesToAddress([]byte{4})]
	res, err := p.Run(data)
	if err != nil {
		t.Fatalf("identity failed: %v", err)
	}
	if string(res) != string(data) {
		t.Fatalf("identity mismatch")
	}
}

func TestPrecompiled_ModExp_0x5(t *testing.T) {
	// Compute 3^5 mod 7 = 243 mod 7 = 5
	// Layout: Bsize (32) || Esize (32) || Msize (32) || B || E || M
	input := make([]byte, 96+3)
	input[31] = 1 // Bsize = 1
	input[63] = 1 // Esize = 1
	input[95] = 1 // Msize = 1
	input[96] = 3 // B = 3
	input[97] = 5 // E = 5
	input[98] = 7 // M = 7

	p := PrecompiledContracts[types.BytesToAddress([]byte{5})]
	res, err := p.Run(input)
	if err != nil {
		t.Fatalf("modexp failed: %v", err)
	}
	if len(res) != 1 || res[0] != 5 {
		t.Fatalf("expected 5, got %v", res)
	}
}

func TestPrecompiled_AltBn128_0x6_0x7_0x8(t *testing.T) {
	// Verify alt_bn128Add (0x6), alt_bn128Mul (0x7), alt_bn128Pairing (0x8) exist
	for i := 6; i <= 8; i++ {
		p, ok := PrecompiledContracts[types.BytesToAddress([]byte{byte(i)})]
		if !ok || p == nil {
			t.Fatalf("precompile 0x%x not registered", i)
		}
	}
}

func TestPrecompiled_Blake2F_0x9(t *testing.T) {
	p, ok := PrecompiledContracts[types.BytesToAddress([]byte{9})]
	if !ok || p == nil {
		t.Fatalf("precompile 0x9 (Blake2F) not registered")
	}
}

func TestPrecompiled_RIP7212_0x100(t *testing.T) {
	p := PrecompiledContracts[types.HexToAddressMust("0x0000000000000000000000000000000000000100")]
	if p == nil {
		t.Fatalf("RIP-7212 precompile not registered at 0x100")
	}

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate p256 key: %v", err)
	}
	msgHash := sha256.Sum256([]byte("MSCA WebAuthn Test"))
	r, s, err := ecdsa.Sign(rand.Reader, privKey, msgHash[:])
	if err != nil {
		t.Fatalf("failed to sign p256: %v", err)
	}

	input := make([]byte, 160)
	copy(input[0:32], msgHash[:])
	rBytes := r.Bytes()
	copy(input[32+(32-len(rBytes)):64], rBytes)
	sBytes := s.Bytes()
	copy(input[64+(32-len(sBytes)):96], sBytes)
	xBytes := privKey.PublicKey.X.Bytes()
	copy(input[96+(32-len(xBytes)):128], xBytes)
	yBytes := privKey.PublicKey.Y.Bytes()
	copy(input[128+(32-len(yBytes)):160], yBytes)

	res, err := p.Run(input)
	if err != nil {
		t.Fatalf("RIP-7212 run failed: %v", err)
	}
	if len(res) != 32 || res[31] != 1 {
		t.Fatalf("expected 1 (valid signature), got %v", res)
	}
}

func TestPrecompiled_Poseidon_0x200(t *testing.T) {
	p := PrecompiledContracts[types.HexToAddressMust("0x0000000000000000000000000000000000000200")]
	if p == nil {
		t.Fatalf("Poseidon precompile not registered at 0x200")
	}

	var left, right [32]byte
	copy(left[:], new(big.Int).SetInt64(100).Bytes())
	copy(right[:], new(big.Int).SetInt64(200).Bytes())
	expected := poseidon.Hash2(left, right)

	input := append(left[:], right[:]...)
	res, err := p.Run(input)
	if err != nil {
		t.Fatalf("poseidon run failed: %v", err)
	}
	if len(res) != 32 || string(res) != string(expected[:]) {
		t.Fatalf("poseidon hash mismatch")
	}
}
