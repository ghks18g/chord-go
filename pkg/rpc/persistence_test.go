package rpc

import (
	"os"
	"testing"

	"github.com/ghks18g/chord-go/pkg/app"
	"github.com/ghks18g/chord-go/pkg/bft"
	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

func TestPersistence_SaveAndRestore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "chord_persist_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	chainID := "chord_7613-1"
	appl := app.NewApp(chainID)
	validators := []*bft.Validator{
		{ID: 1, Address: types.HexToAddressMust("0xa983D897D00482FEEf3000Ad2BEEf358B9fA06bC"), VotingPower: 2500},
	}
	engine := bft.NewEngine(validators, false)

	server := NewServer(appl, engine)
	server.SetDataDir(tempDir)

	// Set initial balances and height
	user := types.HexToAddressMust("0x1111111111111111111111111111111111111111")
	userBal := uint256.NewInt(500000)
	appl.SetBalance(user, userBal)
	appl.SetHeight(5)

	// Save state and data
	if err := server.Persist(); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	// Create new instances simulating node restart
	appl2 := app.NewApp(chainID)
	engine2 := bft.NewEngine(validators, false)
	server2 := NewServer(appl2, engine2)
	server2.SetDataDir(tempDir)

	restored, err := server2.Restore()
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	if !restored {
		t.Fatalf("expected restored = true")
	}

	// Verify restored height and balance
	if appl2.Height() != 5 {
		t.Fatalf("expected restored height 5, got %d", appl2.Height())
	}
	bal := appl2.GetBalance(user)
	if bal == nil || !bal.Eq(userBal) {
		t.Fatalf("expected restored balance %s, got %v", userBal.Dec(), bal)
	}
}
