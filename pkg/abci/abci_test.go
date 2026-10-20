package abci

import (
	"testing"

	"github.com/ghks18g/chord-go/pkg/app"
	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

func TestABCIHandler(t *testing.T) {
	appl := app.NewApp("chord_7613-1")
	h := NewHandler(appl)

	alice, _ := types.HexToAddress("0x1111111111111111111111111111111111111111")
	bob, _ := types.HexToAddress("0x2222222222222222222222222222222222222222")
	appl.SetBalance(alice, uint256.NewInt(500))

	tx := &types.Transaction{
		Nonce: 1,
		From:  alice,
		To:    &bob,
		Value: uint256.NewInt(200),
	}

	// 1. PrepareProposal
	prepResp, err := h.PrepareProposal(&RequestPrepareProposal{Txs: []*types.Transaction{tx}})
	if err != nil || len(prepResp.Txs) != 1 {
		t.Fatalf("PrepareProposal failed")
	}

	// 2. ProcessProposal
	procResp, err := h.ProcessProposal(&RequestProcessProposal{
		Header: types.Header{ChainID: "chord_7613-1", Height: 1},
		Txs:    prepResp.Txs,
	})
	if err != nil || procResp.Status != StatusAccept {
		t.Fatalf("ProcessProposal rejected")
	}

	// 3. FinalizeBlock
	finResp, err := h.FinalizeBlock(&RequestFinalizeBlock{
		Header: types.Header{ChainID: "chord_7613-1", Height: 1},
		Txs:    prepResp.Txs,
	})
	if err != nil || len(finResp.TxResults) != 1 || finResp.TxResults[0] != nil {
		t.Fatalf("FinalizeBlock failed: %v", finResp.TxResults)
	}

	// 4. Commit
	root := h.Commit()
	if root == [32]byte{} {
		t.Fatalf("commit returned zero root")
	}

	if appl.GetBalance(bob).Cmp(uint256.NewInt(200)) != 0 {
		t.Fatalf("bob balance should be 200")
	}
}
