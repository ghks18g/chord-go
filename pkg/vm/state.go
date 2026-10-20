package vm

import (
	"github.com/ghks18g/chord-go/pkg/types"

	"github.com/holiman/uint256"
)

// StateDB is the interface required by the EVM to access accounts and storage.
type StateDB interface {
	GetBalance(addr types.Address) *uint256.Int
	SetBalance(addr types.Address, amount *uint256.Int)
	AddBalance(addr types.Address, amount *uint256.Int)
	SubBalance(addr types.Address, amount *uint256.Int)
	GetNonce(addr types.Address) uint64
	SetNonce(addr types.Address, nonce uint64)
	GetCode(addr types.Address) []byte
	SetCode(addr types.Address, code []byte)
	GetState(addr types.Address, key types.Hash) types.Hash
	SetState(addr types.Address, key types.Hash, value types.Hash)
	Exist(addr types.Address) bool
	CreateAccount(addr types.Address)
}

// MemoryStateDB is an in-memory implementation of StateDB for testing.
type MemoryStateDB struct {
	balances map[types.Address]*uint256.Int
	nonces   map[types.Address]uint64
	codes    map[types.Address][]byte
	storage  map[types.Address]map[types.Hash]types.Hash
}

// NewMemoryStateDB creates an empty in-memory state DB.
func NewMemoryStateDB() *MemoryStateDB {
	return &MemoryStateDB{
		balances: make(map[types.Address]*uint256.Int),
		nonces:   make(map[types.Address]uint64),
		codes:    make(map[types.Address][]byte),
		storage:  make(map[types.Address]map[types.Hash]types.Hash),
	}
}

func (m *MemoryStateDB) Exist(addr types.Address) bool {
	_, ok := m.balances[addr]
	return ok
}

func (m *MemoryStateDB) CreateAccount(addr types.Address) {
	if _, ok := m.balances[addr]; !ok {
		m.balances[addr] = uint256.NewInt(0)
		m.nonces[addr] = 0
		m.storage[addr] = make(map[types.Hash]types.Hash)
	}
}

func (m *MemoryStateDB) GetBalance(addr types.Address) *uint256.Int {
	if b, ok := m.balances[addr]; ok {
		return b.Clone()
	}
	return uint256.NewInt(0)
}

func (m *MemoryStateDB) SetBalance(addr types.Address, amount *uint256.Int) {
	m.CreateAccount(addr)
	m.balances[addr] = amount.Clone()
}

func (m *MemoryStateDB) AddBalance(addr types.Address, amount *uint256.Int) {
	m.CreateAccount(addr)
	cur := m.GetBalance(addr)
	cur.Add(cur, amount)
	m.balances[addr] = cur
}

func (m *MemoryStateDB) SubBalance(addr types.Address, amount *uint256.Int) {
	m.CreateAccount(addr)
	cur := m.GetBalance(addr)
	cur.Sub(cur, amount)
	m.balances[addr] = cur
}

func (m *MemoryStateDB) GetNonce(addr types.Address) uint64 {
	return m.nonces[addr]
}

func (m *MemoryStateDB) SetNonce(addr types.Address, nonce uint64) {
	m.CreateAccount(addr)
	m.nonces[addr] = nonce
}

func (m *MemoryStateDB) GetCode(addr types.Address) []byte {
	return m.codes[addr]
}

func (m *MemoryStateDB) SetCode(addr types.Address, code []byte) {
	m.CreateAccount(addr)
	c := make([]byte, len(code))
	copy(c, code)
	m.codes[addr] = c
}

func (m *MemoryStateDB) GetState(addr types.Address, key types.Hash) types.Hash {
	if s, ok := m.storage[addr]; ok {
		return s[key]
	}
	return types.EmptyHash()
}

func (m *MemoryStateDB) SetState(addr types.Address, key types.Hash, value types.Hash) {
	m.CreateAccount(addr)
	if m.storage[addr] == nil {
		m.storage[addr] = make(map[types.Hash]types.Hash)
	}
	m.storage[addr][key] = value
}
