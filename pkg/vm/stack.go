package vm

import (
	"errors"
	"fmt"

	"github.com/holiman/uint256"
)

var (
	ErrStackUnderflow = errors.New("stack underflow")
	ErrStackOverflow  = errors.New("stack overflow")
)

const MaxStackDepth = 1024

// Stack represents the EVM 256-bit execution stack.
type Stack struct {
	data []uint256.Int
}

// NewStack creates an empty stack with capacity.
func NewStack() *Stack {
	return &Stack{
		data: make([]uint256.Int, 0, 16),
	}
}

// Len returns the current depth of the stack.
func (s *Stack) Len() int {
	return len(s.data)
}

// Push pushes a 256-bit value onto the stack.
func (s *Stack) Push(val *uint256.Int) error {
	if len(s.data) >= MaxStackDepth {
		return ErrStackOverflow
	}
	s.data = append(s.data, *val)
	return nil
}

// Pop pops the top 256-bit value from the stack.
func (s *Stack) Pop() (*uint256.Int, error) {
	if len(s.data) == 0 {
		return nil, ErrStackUnderflow
	}
	idx := len(s.data) - 1
	val := s.data[idx]
	s.data = s.data[:idx]
	return &val, nil
}

// Peek returns the top value without removing it.
func (s *Stack) Peek() (*uint256.Int, error) {
	if len(s.data) == 0 {
		return nil, ErrStackUnderflow
	}
	return &s.data[len(s.data)-1], nil
}

// Back returns the n-th value from the top (0-indexed: 0 is top).
func (s *Stack) Back(n int) (*uint256.Int, error) {
	if n < 0 || n >= len(s.data) {
		return nil, fmt.Errorf("%w: cannot access index %d with stack size %d", ErrStackUnderflow, n, len(s.data))
	}
	return &s.data[len(s.data)-1-n], nil
}

// Dup duplicates the n-th value from the top onto the top (1-indexed: DUP1 duplicates top).
func (s *Stack) Dup(n int) error {
	if n < 1 || n > len(s.data) {
		return fmt.Errorf("%w: dup %d requires stack depth >= %d", ErrStackUnderflow, n, n)
	}
	if len(s.data) >= MaxStackDepth {
		return ErrStackOverflow
	}
	val := s.data[len(s.data)-n]
	s.data = append(s.data, val)
	return nil
}

// Swap swaps the top item with the n-th item below it (1-indexed: SWAP1 swaps top and second item).
func (s *Stack) Swap(n int) error {
	if n < 1 || n >= len(s.data) {
		return fmt.Errorf("%w: swap %d requires stack depth >= %d", ErrStackUnderflow, n, n+1)
	}
	topIdx := len(s.data) - 1
	targetIdx := len(s.data) - 1 - n
	s.data[topIdx], s.data[targetIdx] = s.data[targetIdx], s.data[topIdx]
	return nil
}
