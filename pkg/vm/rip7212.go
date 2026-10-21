package vm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"
)

// RIP7212Precompile implements the secp256r1 P256VERIFY precompile as defined in RIP-7212.
type RIP7212Precompile struct{}

// RIP7212Gas is the fixed gas cost for P256VERIFY (3,450 gas).
const RIP7212Gas uint64 = 3450

// RequiredGas returns the gas cost for executing P256VERIFY.
func (p *RIP7212Precompile) RequiredGas(input []byte) uint64 {
	return RIP7212Gas
}

// Run executes P256VERIFY.
// Input format (160 bytes):
// hash[32] || r[32] || s[32] || x[32] || y[32]
// Returns 0x00...01 (32 bytes) on successful verification, or empty slice on failure.
func (p *RIP7212Precompile) Run(input []byte) ([]byte, error) {
	if len(input) < 160 {
		return []byte{}, nil
	}

	hash := input[:32]
	r := new(big.Int).SetBytes(input[32:64])
	s := new(big.Int).SetBytes(input[64:96])
	x := new(big.Int).SetBytes(input[96:128])
	y := new(big.Int).SetBytes(input[128:160])

	curve := elliptic.P256()
	params := curve.Params()

	// Verify (x, y) is on curve and not at infinity
	if x.Sign() == 0 && y.Sign() == 0 {
		return []byte{}, nil
	}
	if !curve.IsOnCurve(x, y) {
		return []byte{}, nil
	}

	// Verify r, s are in [1, n-1]
	zero := big.NewInt(0)
	if r.Cmp(zero) <= 0 || r.Cmp(params.N) >= 0 {
		return []byte{}, nil
	}
	if s.Cmp(zero) <= 0 || s.Cmp(params.N) >= 0 {
		return []byte{}, nil
	}

	pubKey := &ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}

	if !ecdsa.Verify(pubKey, hash, r, s) {
		return []byte{}, nil
	}

	// Verification succeeded: return 32-byte 0x00...01
	out := make([]byte, 32)
	out[31] = 1
	return out, nil
}
