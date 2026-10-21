package bft

import (
	"errors"
	"sync"
	"time"

	"github.com/ghks18g/chord-go/pkg/types"
)

// Validator represents a consensus validator node.
type Validator struct {
	ID      int
	Address types.Address
	VotingPower int64
}

// Engine implements an on-demand BFT consensus engine for CHORD.
type Engine struct {
	mu            sync.Mutex
	validators    []*Validator
	mempool       []*types.Transaction
	currentHeight uint64
	lastBlockHash types.Hash
	createEmpty   bool // false for on-demand Snowman-like consensus
	blocks        map[uint64]*types.Block
}

func NewEngine(validators []*Validator, createEmpty bool) *Engine {
	return &Engine{
		validators:  validators,
		mempool:     make([]*types.Transaction, 0),
		createEmpty: createEmpty,
		blocks:      make(map[uint64]*types.Block),
	}
}

// AddTx adds a transaction to the mempool and triggers on-demand block generation if needed.
func (e *Engine) AddTx(tx *types.Transaction) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.mempool = append(e.mempool, tx)
}

// MempoolSize returns the number of pending transactions.
func (e *Engine) MempoolSize() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.mempool)
}

// RoundResult represents the outcome of a BFT consensus round.
type RoundResult struct {
	Block     *types.Block
	Votes     int
	Finalized bool
	Duration  time.Duration
}

// GetProposerForHeight returns the designated validator proposer for a block height using Round-Robin.
func (e *Engine) GetProposerForHeight(height uint64) types.Address {
	if len(e.validators) == 0 {
		addr, _ := types.HexToAddress("0xa983D897D00482FEEf3000Ad2BEEf358B9fA06bC")
		return addr
	}
	idx := int((height - 1) % uint64(len(e.validators)))
	return e.validators[idx].Address
}

// Validators returns the list of consensus validators.
func (e *Engine) Validators() []*Validator {
	return e.validators
}

func (e *Engine) isValidator(addr types.Address) bool {
	for _, val := range e.validators {
		if val != nil && val.Address == addr {
			return true
		}
	}
	return false
}

// ProposeAndCommit performs a consensus round among the 4 validators with Round-Robin proposal.
// Returns ErrNoTransactionsToCommit if createEmpty is false and mempool is empty.
func (e *Engine) ProposeAndCommit(explicitProposer types.Address) (*RoundResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// On-Demand Rule: If no transactions and createEmpty is false, do not produce block.
	if !e.createEmpty && len(e.mempool) == 0 {
		return nil, ErrNoTransactionsToCommit
	}

	start := time.Now()
	e.currentHeight++

	// Determine proposer: Use round-robin validator across 4 nodes
	proposer := explicitProposer
	if proposer == (types.Address{}) || !e.isValidator(proposer) {
		proposer = e.GetProposerForHeight(e.currentHeight)
	}

	txs := make([]*types.Transaction, len(e.mempool))
	copy(txs, e.mempool)
	e.mempool = e.mempool[:0] // drain mempool

	block := &types.Block{
		Header: types.Header{
			ChainID:   "chord_7613-1",
			Height:    e.currentHeight,
			PrevHash:  e.lastBlockHash,
			Timestamp: time.Now(),
			Proposer:  proposer,
		},
		Transactions: txs,
	}

	// 4-Node BFT Quorum: Need >= 2f + 1 votes where N=4, f=1 => >= 3 votes
	votes := 0
	totalValidators := len(e.validators)
	requiredQuorum := (totalValidators * 2 / 3) + 1 // 4 * 2 / 3 + 1 = 3

	// Phase 1: Prevote & Precommit Simulation
	for _, val := range e.validators {
		// Mock: all active validators verify block and vote yes
		if val != nil {
			votes++
		}
	}

	if votes < requiredQuorum {
		return nil, errors.New("bft quorum not reached")
	}

	blockHash := block.Hash()
	e.lastBlockHash = blockHash
	e.blocks[e.currentHeight] = block

	return &RoundResult{
		Block:     block,
		Votes:     votes,
		Finalized: true,
		Duration:  time.Since(start),
	}, nil
}

// GetBlock returns a block by height.
func (e *Engine) GetBlock(height uint64) (*types.Block, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	b, ok := e.blocks[height]
	return b, ok
}

// PruneBlocks removes blocks older than retainHeight to enforce disk limits.
func (e *Engine) PruneBlocks(retainHeight uint64) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	pruned := 0
	for h := range e.blocks {
		if e.currentHeight > retainHeight && h <= e.currentHeight-retainHeight {
			delete(e.blocks, h)
			pruned++
		}
	}
	return pruned
}

var ErrNoTransactionsToCommit = errors.New("mempool empty and create_empty_blocks is false")

// ExportBlocks returns all stored blocks.
func (e *Engine) ExportBlocks() []*types.Block {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]*types.Block, 0, len(e.blocks))
	for _, b := range e.blocks {
		out = append(out, b)
	}
	return out
}

// RestoreBlocks restores blocks and advances engine height.
func (e *Engine) RestoreBlocks(blocks []*types.Block) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, b := range blocks {
		if b == nil {
			continue
		}
		e.blocks[b.Header.Height] = b
		if b.Header.Height > e.currentHeight {
			e.currentHeight = b.Header.Height
			e.lastBlockHash = b.Hash()
		}
	}
}
