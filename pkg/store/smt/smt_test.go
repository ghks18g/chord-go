package smt

import (
	"bytes"
	"testing"
)

func TestSMT(t *testing.T) {
	tree := NewTree()

	// 1. Initial empty root
	emptyRoot := tree.Root()
	if emptyRoot != zeroHashes[Depth] {
		t.Fatalf("expected empty root to match precomputed zeroHashes[256]")
	}

	var key1, val1 [32]byte
	key1[31] = 0xAA
	val1[31] = 0xBB

	// 2. Insert key1
	newRoot := tree.Update(key1, val1)
	if newRoot == emptyRoot {
		t.Fatalf("root must change after insertion")
	}

	gotVal, ok := tree.Get(key1)
	if !ok || gotVal != val1 {
		t.Fatalf("expected to get val1 for key1")
	}

	// 3. Merkle Inclusion Proof
	proof, err := tree.Prove(key1)
	if err != nil {
		t.Fatalf("failed to generate proof: %v", err)
	}

	if !Verify(tree.Root(), key1, val1, proof) {
		t.Fatalf("merkle inclusion proof verification failed")
	}

	// 4. Tampered value verification must fail
	var fakeVal [32]byte
	fakeVal[31] = 0xCC
	if Verify(tree.Root(), key1, fakeVal, proof) {
		t.Fatalf("tampered value verification must fail")
	}

	// 5. Non-membership Proof for non-existent key
	var nonExistentKey [32]byte
	nonExistentKey[31] = 0xEE
	nonProof, err := tree.Prove(nonExistentKey)
	if err != nil {
		t.Fatalf("failed to generate non-inclusion proof: %v", err)
	}

	var zeroVal [32]byte
	if !Verify(tree.Root(), nonExistentKey, zeroVal, nonProof) {
		t.Fatalf("non-membership proof verification failed")
	}

	// 6. Delete key1 and verify root returns to empty
	delRoot := tree.Update(key1, [32]byte{})
	if !bytes.Equal(delRoot[:], emptyRoot[:]) {
		t.Fatalf("root should return to emptyRoot after deleting all keys")
	}
}
