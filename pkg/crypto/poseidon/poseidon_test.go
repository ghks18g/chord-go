package poseidon

import (
	"bytes"
	"testing"
)

func TestPoseidonHash(t *testing.T) {
	var a, b [32]byte
	a[31] = 1
	b[31] = 2

	h1 := Hash2(a, b)
	if h1 == [32]byte{} {
		t.Fatalf("expected non-zero hash output")
	}

	// Determinism check
	h2 := Hash2(a, b)
	if !bytes.Equal(h1[:], h2[:]) {
		t.Fatalf("poseidon hash must be deterministic")
	}

	// Non-collision check
	b[31] = 3
	h3 := Hash2(a, b)
	if bytes.Equal(h1[:], h3[:]) {
		t.Fatalf("different inputs must produce different outputs")
	}
}
