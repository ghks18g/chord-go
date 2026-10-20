package bft

import (
	"testing"

	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

func TestBFTConsensus(t *testing.T) {
	// Initialize 4 validators
	validators := make([]*Validator, 4)
	for i := 0; i < 4; i++ {
		var addr types.Address
		addr[19] = byte(i + 1)
		validators[i] = &Validator{
			ID:          i + 1,
			Address:     addr,
			VotingPower: 100,
		}
	}

	engine := NewEngine(validators, false) // createEmpty = false (On-Demand)

	// 1. Verify idle engine does not produce empty block
	_, err := engine.ProposeAndCommit(validators[0].Address)
	if err != ErrNoTransactionsToCommit {
		t.Fatalf("expected ErrNoTransactionsToCommit when mempool is empty, got: %v", err)
	}

	// 2. Add transaction and commit
	tx := &types.Transaction{
		Nonce: 1,
		From:  validators[0].Address,
		Value: uint256.NewInt(500),
	}
	engine.AddTx(tx)

	res, err := engine.ProposeAndCommit(validators[0].Address)
	if err != nil {
		t.Fatalf("consensus failed: %v", err)
	}

	if !res.Finalized || res.Votes < 3 {
		t.Fatalf("expected >= 3 votes and finalized block, got votes: %d", res.Votes)
	}

	if res.Block.Header.Height != 1 || len(res.Block.Transactions) != 1 {
		t.Fatalf("unexpected block content: height %d, txs %d", res.Block.Header.Height, len(res.Block.Transactions))
	}

	// 3. Fault Tolerance test (N=4, f=1): 1 validator crashes (3 active validators)
	crashedEngine := NewEngine(validators[:3], false)
	crashedEngine.AddTx(tx)

	res2, err := crashedEngine.ProposeAndCommit(validators[0].Address)
	if err != nil {
		t.Fatalf("BFT should tolerate 1 node failure (3/4 quorum), but failed: %v", err)
	}
	if res2.Votes != 3 || !res2.Finalized {
		t.Fatalf("expected 3 votes with 1 offline node")
	}

	// 4. Pruning test
	// Produce 10 blocks
	for i := uint64(2); i <= 10; i++ {
		engine.AddTx(tx)
		_, _ = engine.ProposeAndCommit(validators[0].Address)
	}

	// Keep only recent 3 blocks
	prunedCount := engine.PruneBlocks(3)
	if prunedCount == 0 {
		t.Fatalf("expected old blocks to be pruned")
	}

	// Block 1 should no longer exist
	_, ok := engine.GetBlock(1)
	if ok {
		t.Fatalf("block 1 should have been pruned")
	}

	// Block 10 must still exist
	_, ok = engine.GetBlock(10)
	if !ok {
		t.Fatalf("block 10 must be retained")
	}
}

func TestRoundRobinBlockProposer(t *testing.T) {
	validators := make([]*Validator, 4)
	for i := 0; i < 4; i++ {
		var addr types.Address
		addr[19] = byte(i + 1)
		validators[i] = &Validator{
			ID:          i + 1,
			Address:     addr,
			VotingPower: 2500,
		}
	}

	engine := NewEngine(validators, true) // createEmpty = true to easily generate blocks

	expectedProposers := []types.Address{
		validators[0].Address, // Block 1 -> (1-1)%4 = 0
		validators[1].Address, // Block 2 -> (2-1)%4 = 1
		validators[2].Address, // Block 3 -> (3-1)%4 = 2
		validators[3].Address, // Block 4 -> (4-1)%4 = 3
		validators[0].Address, // Block 5 -> (5-1)%4 = 0
		validators[1].Address, // Block 6 -> (6-1)%4 = 1
	}

	for height, expected := range expectedProposers {
		res, err := engine.ProposeAndCommit(types.Address{}) // Empty proposer signals automatic round-robin
		if err != nil {
			t.Fatalf("height %d failed: %v", height+1, err)
		}
		if res.Block.Header.Height != uint64(height+1) {
			t.Fatalf("expected height %d, got %d", height+1, res.Block.Header.Height)
		}
		if res.Block.Header.Proposer != expected {
			t.Fatalf("height %d proposer mismatch: expected %s, got %s", height+1, expected.Hex(), res.Block.Header.Proposer.Hex())
		}
	}
}
