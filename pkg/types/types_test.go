package types

import (
	"testing"
	"time"

	"github.com/holiman/uint256"
)

func TestTypes(t *testing.T) {
	from, err := HexToAddress("0x1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatalf("failed to parse from address: %v", err)
	}
	to, err := HexToAddress("0x2222222222222222222222222222222222222222")
	if err != nil {
		t.Fatalf("failed to parse to address: %v", err)
	}

	val := uint256.NewInt(1000)
	tx := &Transaction{
		Nonce:  1,
		From:   from,
		To:     &to,
		Value:  val,
		Data:   []byte("test-data"),
		TxType: 1, // EIP-712
	}

	h := tx.Hash()
	if h == [32]byte{} {
		t.Fatalf("expected non-zero tx hash")
	}

	gen := DefaultGenesis()
	if gen.ChainID != "chord_7613-1" || gen.EVMChainID != 7613 {
		t.Fatalf("unexpected genesis chain id: %s, %d", gen.ChainID, gen.EVMChainID)
	}

	block := &Block{
		Header: Header{
			ChainID:   gen.ChainID,
			Height:    1,
			Timestamp: time.Now(),
			Proposer:  from,
		},
		Transactions: []*Transaction{tx},
	}

	bh := block.Hash()
	if bh == [32]byte{} {
		t.Fatalf("expected non-zero block hash")
	}
}
