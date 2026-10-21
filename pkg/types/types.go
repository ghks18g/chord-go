package types

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/holiman/uint256"
)

// Hash represents a 32-byte cryptographic hash.
type Hash [32]byte

func EmptyHash() Hash {
	return Hash{}
}

func (h Hash) Bytes() []byte {
	return h[:]
}

func BytesToHash(b []byte) Hash {
	var h Hash
	if len(b) > 32 {
		b = b[len(b)-32:]
	}
	copy(h[32-len(b):], b)
	return h
}

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

func (h Hash) Hex() string {
	return "0x" + hex.EncodeToString(h[:])
}

func HexToHash(s string) (Hash, error) {
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return Hash{}, err
	}
	if len(b) != 32 {
		return Hash{}, fmt.Errorf("invalid hash length: %d", len(b))
	}
	var h Hash
	copy(h[:], b)
	return h, nil
}

// Address represents a 20-byte account address.
type Address [20]byte

func (a Address) Bytes() []byte {
	return a[:]
}

func BytesToAddress(b []byte) Address {
	var a Address
	if len(b) > 20 {
		b = b[len(b)-20:]
	}
	copy(a[20-len(b):], b)
	return a
}

func (a Address) String() string {
	return "0x" + hex.EncodeToString(a[:])
}

// Hex returns the EIP-55 or 0x-prefixed hexadecimal representation.
func (a Address) Hex() string {
	return a.String()
}

func HexToAddress(s string) (Address, error) {
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return Address{}, err
	}
	if len(b) != 20 {
		return Address{}, fmt.Errorf("invalid address length: %d", len(b))
	}
	var a Address
	copy(a[:], b)
	return a, nil
}

func HexToAddressMust(s string) Address {
	addr, err := HexToAddress(s)
	if err != nil {
		panic(err)
	}
	return addr
}

// Log represents a contract log event.
type Log struct {
	Address     Address `json:"address"`
	Topics      []Hash  `json:"topics"`
	Data        []byte  `json:"data"`
	BlockNumber uint64  `json:"blockNumber"`
	TxHash      Hash    `json:"transactionHash,omitempty"`
	TxIndex     uint    `json:"transactionIndex,omitempty"`
	BlockHash   Hash    `json:"blockHash,omitempty"`
	Index       uint    `json:"logIndex,omitempty"`
	Removed     bool    `json:"removed,omitempty"`
}

type Transaction struct {
	Nonce     uint64          `json:"nonce"`
	From      Address         `json:"from"`
	To        *Address        `json:"to,omitempty"`
	Value     *uint256.Int    `json:"value"`
	Data      []byte          `json:"data,omitempty"`
	Signature []byte          `json:"signature,omitempty"`
	ZKProof   []byte          `json:"zk_proof,omitempty"` // EIP-1271 or ZK verification payload
	TxType    uint8           `json:"tx_type"`            // 0: Standard ECDSA, 1: EIP-712, 2: ZK Payload
	TxHash    *Hash           `json:"tx_hash,omitempty"`  // Raw/Keccak-256 hash override if set
}

// Hash returns the transaction hash (Keccak-256 if explicit, otherwise SHA-256).
func (tx *Transaction) Hash() Hash {
	if tx.TxHash != nil {
		return *tx.TxHash
	}
	data, _ := json.Marshal(struct {
		Nonce  uint64       `json:"nonce"`
		From   Address      `json:"from"`
		To     *Address     `json:"to,omitempty"`
		Value  *uint256.Int `json:"value"`
		Data   []byte       `json:"data,omitempty"`
		TxType uint8        `json:"tx_type"`
	}{
		Nonce:  tx.Nonce,
		From:   tx.From,
		To:     tx.To,
		Value:  tx.Value,
		Data:   tx.Data,
		TxType: tx.TxType,
	})
	return sha256.Sum256(data)
}

// Header represents the header of a block in CHORD.
type Header struct {
	ChainID     string    `json:"chain_id"`
	Height      uint64    `json:"height"`
	PrevHash    Hash      `json:"prev_hash"`
	StateRoot   Hash      `json:"state_root"` // Poseidon SMT Root
	TxRoot      Hash      `json:"tx_root"`
	Timestamp   time.Time `json:"timestamp"`
	Proposer    Address   `json:"proposer"`
}

// Block represents a full block with transactions.
type Block struct {
	Header       Header         `json:"header"`
	Transactions []*Transaction `json:"transactions"`
}

func (b *Block) Hash() Hash {
	data, _ := json.Marshal(b.Header)
	return sha256.Sum256(data)
}

// AccountState represents the account balance and nonce stored in SMT.
type AccountState struct {
	Nonce   uint64       `json:"nonce"`
	Balance *uint256.Int `json:"balance"`
	CodeHash Hash        `json:"code_hash,omitempty"`
}

// GenesisValidator represents a genesis consensus validator.
type GenesisValidator struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Address     Address `json:"address"`
	VotingPower int64   `json:"voting_power"`
	Weight      float64 `json:"weight"`
}

// Genesis defines the initial state of the CHORD chain.
type Genesis struct {
	ChainID    string                  `json:"chain_id"`
	EVMChainID uint64                  `json:"evm_chain_id"`
	Validators []GenesisValidator      `json:"validators"`
	Alloc      map[Address]*uint256.Int `json:"alloc"`
}

func DefaultGenesis() Genesis {
	// 4 SECP256K1 Validator Addresses (25% weight each)
	v1, _ := HexToAddress("0xa983D897D00482FEEf3000Ad2BEEf358B9fA06bC")
	v2, _ := HexToAddress("0xAeA7958f66EEdCc146a988b11eb0268F68E5B9DF")
	v3, _ := HexToAddress("0x4980B80169F964EecE5EDe078B53f207Afd0b921")
	v4, _ := HexToAddress("0x483A0b4F5Db0C6ac6a7F67deE01494D45E266d18")

	// Genesis Treasury / Minting & Airdrop Reserve
	multiSig, _ := HexToAddress("0x470cC21093319e612E9779cE2c1d53b0F84562F5")

	alloc := make(map[Address]*uint256.Int)

	// 25,000,000 CHOR each (25% staking share per validator) = 2.5e25 wei (note)
	valStaking, _ := uint256.FromDecimal("25000000000000000000000000")
	alloc[v1] = valStaking
	alloc[v2] = new(uint256.Int).Set(valStaking)
	alloc[v3] = new(uint256.Int).Set(valStaking)
	alloc[v4] = new(uint256.Int).Set(valStaking)

	// 900,000,000 CHOR in Genesis Treasury = 9.0e26 wei (note)
	multiSigBal, _ := uint256.FromDecimal("900000000000000000000000000")
	alloc[multiSig] = multiSigBal

	return Genesis{
		ChainID:    "chord_7613-1",
		EVMChainID: 7613,
		Validators: []GenesisValidator{
			{ID: 1, Name: "CHORD Validator 1 (Gateway Proposer)", Address: v1, VotingPower: 2500, Weight: 0.25},
			{ID: 2, Name: "CHORD Validator 2 (Private BFT Peer)", Address: v2, VotingPower: 2500, Weight: 0.25},
			{ID: 3, Name: "CHORD Validator 3 (Private BFT Peer)", Address: v3, VotingPower: 2500, Weight: 0.25},
			{ID: 4, Name: "CHORD Validator 4 (Private BFT Peer)", Address: v4, VotingPower: 2500, Weight: 0.25},
		},
		Alloc: alloc,
	}
}

