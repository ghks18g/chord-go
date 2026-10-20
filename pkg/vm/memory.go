package vm

import "github.com/holiman/uint256"

// Memory provides dynamic byte addressable storage for EVM execution.
type Memory struct {
	store []byte
}

// NewMemory creates a new memory instance.
func NewMemory() *Memory {
	return &Memory{
		store: make([]byte, 0, 1024),
	}
}

// Len returns the current length of the memory buffer.
func (m *Memory) Len() int {
	return len(m.store)
}

// Resize expands the memory up to size if it is smaller.
// EVM memory expands in 32-byte words.
func (m *Memory) Resize(size uint64) {
	if uint64(len(m.store)) < size {
		// Round up to nearest 32 bytes
		rounded := (size + 31) / 32 * 32
		if uint64(cap(m.store)) < rounded {
			newStore := make([]byte, rounded)
			copy(newStore, m.store)
			m.store = newStore
		} else {
			m.store = m.store[:rounded]
		}
	}
}

// Set writes data of specified length from value to offset in memory.
func (m *Memory) Set(offset, size uint64, value []byte) {
	if size == 0 {
		return
	}
	m.Resize(offset + size)
	copy(m.store[offset:offset+size], value)
}

// Set32 writes a 32-byte 256-bit word to the specified offset.
func (m *Memory) Set32(offset uint64, val *uint256.Int) {
	m.Resize(offset + 32)
	b := val.Bytes32()
	copy(m.store[offset:offset+32], b[:])
}

// SetByte writes a single byte at the specified offset.
func (m *Memory) SetByte(offset uint64, val byte) {
	m.Resize(offset + 1)
	m.store[offset] = val
}

// GetCopy returns a copy of length size starting from offset.
func (m *Memory) GetCopy(offset, size uint64) []byte {
	if size == 0 {
		return []byte{}
	}
	if offset > uint64(len(m.store)) {
		return make([]byte, size)
	}
	m.Resize(offset + size)
	res := make([]byte, size)
	copy(res, m.store[offset:offset+size])
	return res
}

// GetPtr returns a slice view of length size starting from offset.
func (m *Memory) GetPtr(offset, size uint64) []byte {
	if size == 0 {
		return nil
	}
	m.Resize(offset + size)
	return m.store[offset : offset+size]
}
