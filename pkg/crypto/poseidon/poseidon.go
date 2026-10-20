package poseidon

import (
	"crypto/sha256"
	"math/big"
)

// BN254 Scalar Field Modulus:
// q = 21888242871839275222246405745257275088548364400416034343698204186575808495617
var Q, _ = new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)

// Parameters for t=3 (2 inputs + 1 capacity)
const (
	T          = 3
	R_F        = 8  // Full rounds
	R_P        = 56 // Partial rounds
	TotalRounds = R_F + R_P
)

// Poseidon holds precomputed round constants and MDS matrix for t=3
type Poseidon struct {
	C [TotalRounds * T]*big.Int
	M [T][T]*big.Int
}

var defaultPoseidon *Poseidon

func init() {
	defaultPoseidon = newPoseidon()
}

func newPoseidon() *Poseidon {
	p := &Poseidon{}
	// Deterministically derive round constants using SHA-256 for testnet/development reproducibility
	seed := []byte("chord_poseidon_bn254_t3_seed")
	for i := 0; i < TotalRounds*T; i++ {
		h := sha256.Sum256(append(seed, byte(i), byte(i>>8)))
		val := new(big.Int).SetBytes(h[:])
		p.C[i] = new(big.Int).Mod(val, Q)
	}

	// MDS matrix initialization (Cauchy-style positive non-singular matrix mod Q)
	for i := 0; i < T; i++ {
		for j := 0; j < T; j++ {
			x := int64(i + 1)
			y := int64(j + T + 1)
			denom := new(big.Int).SetInt64(x + y)
			inv := new(big.Int).ModInverse(denom, Q)
			p.M[i][j] = inv
		}
	}
	return p
}

// exp5 computes x^5 mod Q (S-box)
func exp5(x *big.Int) *big.Int {
	res := new(big.Int).Mul(x, x)
	res.Mod(res, Q)
	res.Mul(res, res)
	res.Mod(res, Q)
	res.Mul(res, x)
	res.Mod(res, Q)
	return res
}

// Hash2 computes Poseidon hash of two 32-byte elements: H(left, right) -> [32]byte
func Hash2(left, right [32]byte) [32]byte {
	return defaultPoseidon.Hash2(left, right)
}

func (p *Poseidon) Hash2(left, right [32]byte) [32]byte {
	state := [T]*big.Int{
		new(big.Int).Mod(new(big.Int).SetBytes(left[:]), Q),
		new(big.Int).Mod(new(big.Int).SetBytes(right[:]), Q),
		big.NewInt(0), // capacity
	}

	rcIdx := 0
	halfRF := R_F / 2

	// First full rounds
	for r := 0; r < halfRF; r++ {
		p.round(state[:], rcIdx, true)
		rcIdx += T
	}

	// Partial rounds
	for r := 0; r < R_P; r++ {
		p.round(state[:], rcIdx, false)
		rcIdx += T
	}

	// Last full rounds
	for r := 0; r < halfRF; r++ {
		p.round(state[:], rcIdx, true)
		rcIdx += T
	}

	// Squeeze first element
	outBytes := state[0].Bytes()
	var out [32]byte
	copy(out[32-len(outBytes):], outBytes)
	return out
}

func (p *Poseidon) round(state []*big.Int, rcOffset int, full bool) {
	// Add round constants
	for i := 0; i < T; i++ {
		state[i].Add(state[i], p.C[rcOffset+i])
		state[i].Mod(state[i], Q)
	}

	// S-box
	if full {
		for i := 0; i < T; i++ {
			state[i] = exp5(state[i])
		}
	} else {
		state[0] = exp5(state[0])
	}

	// Mix with MDS matrix
	newState := [T]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0)}
	for i := 0; i < T; i++ {
		sum := big.NewInt(0)
		for j := 0; j < T; j++ {
			term := new(big.Int).Mul(p.M[i][j], state[j])
			sum.Add(sum, term)
		}
		newState[i] = new(big.Int).Mod(sum, Q)
	}

	for i := 0; i < T; i++ {
		state[i] = newState[i]
	}
}
