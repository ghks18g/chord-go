package smt

import (
	"bytes"
	"errors"

	"github.com/ghks18g/chord-go/pkg/crypto/poseidon"
)

const Depth = 256

// Precomputed zero hashes for each level of the tree (0 to 256)
var zeroHashes [Depth + 1][32]byte

func init() {
	var zero [32]byte
	zeroHashes[0] = zero
	for i := 1; i <= Depth; i++ {
		zeroHashes[i] = poseidon.Hash2(zeroHashes[i-1], zeroHashes[i-1])
	}
}

// Node represents an in-memory node in the SMT.
type Node struct {
	Left  *Node
	Right *Node
	Hash  [32]byte
}

// Tree represents a 256-depth Poseidon-based Sparse Merkle Tree.
type Tree struct {
	root   [32]byte
	values map[[32]byte][32]byte
}

func NewTree() *Tree {
	return &Tree{
		root:   zeroHashes[Depth],
		values: make(map[[32]byte][32]byte),
	}
}

// Root returns the current root hash of the tree.
func (t *Tree) Root() [32]byte {
	return t.root
}

// Get retrieves the value stored at key.
func (t *Tree) Get(key [32]byte) ([32]byte, bool) {
	val, ok := t.values[key]
	return val, ok
}

// Update sets the value for the given key and recomputes the root hash.
func (t *Tree) Update(key [32]byte, value [32]byte) [32]byte {
	if value == [32]byte{} {
		delete(t.values, key)
	} else {
		t.values[key] = value
	}
	t.recomputeRoot()
	return t.root
}

// recomputeRoot recalculates the root hash from the stored key-value pairs.
func (t *Tree) recomputeRoot() {
	if len(t.values) == 0 {
		t.root = zeroHashes[Depth]
		return
	}

	rootNode := &Node{Hash: zeroHashes[Depth]}
	for k, v := range t.values {
		rootNode = insert(rootNode, k, v, Depth)
	}
	t.root = rootNode.Hash
}

func insert(curr *Node, key [32]byte, val [32]byte, depth int) *Node {
	if depth == 0 {
		return &Node{Hash: val}
	}

	bit := getBit(key, depth-1)
	if curr == nil {
		curr = &Node{}
	}

	if bit == 0 {
		left := curr.Left
		if left == nil {
			left = &Node{Hash: zeroHashes[depth-1]}
		}
		curr.Left = insert(left, key, val, depth-1)
		if curr.Right == nil {
			curr.Right = &Node{Hash: zeroHashes[depth-1]}
		}
	} else {
		right := curr.Right
		if right == nil {
			right = &Node{Hash: zeroHashes[depth-1]}
		}
		curr.Right = insert(right, key, val, depth-1)
		if curr.Left == nil {
			curr.Left = &Node{Hash: zeroHashes[depth-1]}
		}
	}

	curr.Hash = poseidon.Hash2(curr.Left.Hash, curr.Right.Hash)
	return curr
}

// getBit returns the n-th bit of a 32-byte array (0 or 1), from bit 0 to 255.
func getBit(data [32]byte, bitIndex int) int {
	byteIndex := 31 - (bitIndex / 8)
	shift := uint(bitIndex % 8)
	return int((data[byteIndex] >> shift) & 1)
}

// Proof represents a Sparse Merkle Tree audit path.
type Proof struct {
	Siblings [][32]byte `json:"siblings"`
}

// Prove generates a Merkle inclusion or non-inclusion proof for a given key.
func (t *Tree) Prove(key [32]byte) (*Proof, error) {
	proof := &Proof{
		Siblings: make([][32]byte, Depth),
	}

	// Build the tree nodes to extract siblings
	rootNode := &Node{Hash: zeroHashes[Depth]}
	for k, v := range t.values {
		rootNode = insert(rootNode, k, v, Depth)
	}

	curr := rootNode
	for d := Depth; d > 0; d-- {
		bit := getBit(key, d-1)
		if curr == nil {
			proof.Siblings[Depth-d] = zeroHashes[d-1]
			continue
		}

		if bit == 0 {
			if curr.Right != nil {
				proof.Siblings[Depth-d] = curr.Right.Hash
			} else {
				proof.Siblings[Depth-d] = zeroHashes[d-1]
			}
			curr = curr.Left
		} else {
			if curr.Left != nil {
				proof.Siblings[Depth-d] = curr.Left.Hash
			} else {
				proof.Siblings[Depth-d] = zeroHashes[d-1]
			}
			curr = curr.Right
		}
	}

	return proof, nil
}

// Verify checks the validity of an inclusion (val != 0) or non-inclusion (val == 0) proof.
func Verify(root [32]byte, key [32]byte, value [32]byte, proof *Proof) bool {
	if proof == nil || len(proof.Siblings) != Depth {
		return false
	}

	currHash := value
	for d := 1; d <= Depth; d++ {
		sibling := proof.Siblings[Depth-d]
		bit := getBit(key, d-1)

		if bit == 0 {
			currHash = poseidon.Hash2(currHash, sibling)
		} else {
			currHash = poseidon.Hash2(sibling, currHash)
		}
	}

	return bytes.Equal(currHash[:], root[:])
}

// ErrKeyNotFound indicates key does not exist
var ErrKeyNotFound = errors.New("key not found in smt")
