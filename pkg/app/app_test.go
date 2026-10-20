package app

import (
	"testing"

	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

func TestAppExecutionAndSMT(t *testing.T) {
	app := NewApp("chord_7613-1")

	alice, _ := types.HexToAddress("0x1111111111111111111111111111111111111111")
	bob, _ := types.HexToAddress("0x2222222222222222222222222222222222222222")

	// Set initial balance
	initBal := uint256.NewInt(1000)
	app.SetBalance(alice, initBal)

	if app.GetBalance(alice).Cmp(initBal) != 0 {
		t.Fatalf("alice balance mismatch")
	}

	// 1. Prove Alice balance with SMT
	proof, err := app.ProveAccount(alice)
	if err != nil {
		t.Fatalf("failed to prove alice account: %v", err)
	}

	if !app.VerifyAccountProof(alice, initBal, proof) {
		t.Fatalf("alice account proof verification failed")
	}

	// 2. Transfer 400 from Alice to Bob
	transferAmt := uint256.NewInt(400)
	tx := &types.Transaction{
		Nonce:  1,
		From:   alice,
		To:     &bob,
		Value:  transferAmt,
		TxType: 0,
	}

	err = app.ExecuteTransaction(tx)
	if err != nil {
		t.Fatalf("execute transaction failed: %v", err)
	}

	expectedAlice := uint256.NewInt(600)
	expectedBob := uint256.NewInt(400)

	if app.GetBalance(alice).Cmp(expectedAlice) != 0 {
		t.Fatalf("alice post-tx balance expected 600, got: %s", app.GetBalance(alice).String())
	}
	if app.GetBalance(bob).Cmp(expectedBob) != 0 {
		t.Fatalf("bob post-tx balance expected 400, got: %s", app.GetBalance(bob).String())
	}

	// 3. Commit block and check height
	root := app.Commit()
	if root == [32]byte{} {
		t.Fatalf("commit root should be non-zero")
	}
	if app.Height() != 1 {
		t.Fatalf("expected height 1, got %d", app.Height())
	}

	// 4. Verify Bob's updated account proof
	bobProof, err := app.ProveAccount(bob)
	if err != nil {
		t.Fatalf("failed to prove bob account: %v", err)
	}
	if !app.VerifyAccountProof(bob, expectedBob, bobProof) {
		t.Fatalf("bob account proof verification failed")
	}
}
