package vm

import (
	"encoding/hex"

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

	// Rollback support (Snapshot & Revert)
	Snapshot() int
	RevertToSnapshot(id int)

	// Log buffering support
	AddLog(log *types.Log)
	GetLogs() []*types.Log
	ClearLogs()

	// Dirty accounts tracking for SMT synchronization
	GetDirtyAccounts() []types.Address
	ClearDirtyAccounts()
}

type snapshot struct {
	id       int
	balances map[types.Address]*uint256.Int
	nonces   map[types.Address]uint64
	codes    map[types.Address][]byte
	storage  map[types.Address]map[types.Hash]types.Hash
	logLen   int
}

// MemoryStateDB is an in-memory implementation of StateDB for testing.
type MemoryStateDB struct {
	balances      map[types.Address]*uint256.Int
	nonces        map[types.Address]uint64
	codes         map[types.Address][]byte
	storage       map[types.Address]map[types.Hash]types.Hash
	logs          []*types.Log
	snapshots     []snapshot
	nextSnapID    int
	dirtyAccounts map[types.Address]struct{}
}

// NewMemoryStateDB creates an empty in-memory state DB.
func NewMemoryStateDB() *MemoryStateDB {
	return &MemoryStateDB{
		balances:      make(map[types.Address]*uint256.Int),
		nonces:        make(map[types.Address]uint64),
		codes:         make(map[types.Address][]byte),
		storage:       make(map[types.Address]map[types.Hash]types.Hash),
		logs:          make([]*types.Log, 0),
		snapshots:     make([]snapshot, 0),
		nextSnapID:    1,
		dirtyAccounts: make(map[types.Address]struct{}),
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
	m.dirtyAccounts[addr] = struct{}{}
}

func (m *MemoryStateDB) AddBalance(addr types.Address, amount *uint256.Int) {
	m.CreateAccount(addr)
	cur := m.GetBalance(addr)
	cur.Add(cur, amount)
	m.balances[addr] = cur
	m.dirtyAccounts[addr] = struct{}{}
}

func (m *MemoryStateDB) SubBalance(addr types.Address, amount *uint256.Int) {
	m.CreateAccount(addr)
	cur := m.GetBalance(addr)
	cur.Sub(cur, amount)
	m.balances[addr] = cur
	m.dirtyAccounts[addr] = struct{}{}
}

func (m *MemoryStateDB) GetNonce(addr types.Address) uint64 {
	return m.nonces[addr]
}

func (m *MemoryStateDB) SetNonce(addr types.Address, nonce uint64) {
	m.CreateAccount(addr)
	m.nonces[addr] = nonce
	m.dirtyAccounts[addr] = struct{}{}
}

func (m *MemoryStateDB) GetCode(addr types.Address) []byte {
	return m.codes[addr]
}

func (m *MemoryStateDB) SetCode(addr types.Address, code []byte) {
	m.CreateAccount(addr)
	c := make([]byte, len(code))
	copy(c, code)
	m.codes[addr] = c
	m.dirtyAccounts[addr] = struct{}{}
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
	m.dirtyAccounts[addr] = struct{}{}
}

func (m *MemoryStateDB) GetDirtyAccounts() []types.Address {
	out := make([]types.Address, 0, len(m.dirtyAccounts))
	for a := range m.dirtyAccounts {
		out = append(out, a)
	}
	return out
}

func (m *MemoryStateDB) ClearDirtyAccounts() {
	m.dirtyAccounts = make(map[types.Address]struct{})
}

// Snapshot creates a state snapshot and returns its identifier.
func (m *MemoryStateDB) Snapshot() int {
	id := m.nextSnapID
	m.nextSnapID++

	snapBalances := make(map[types.Address]*uint256.Int, len(m.balances))
	for k, v := range m.balances {
		if v != nil {
			snapBalances[k] = v.Clone()
		}
	}

	snapNonces := make(map[types.Address]uint64, len(m.nonces))
	for k, v := range m.nonces {
		snapNonces[k] = v
	}

	snapCodes := make(map[types.Address][]byte, len(m.codes))
	for k, v := range m.codes {
		codeCopy := make([]byte, len(v))
		copy(codeCopy, v)
		snapCodes[k] = codeCopy
	}

	snapStorage := make(map[types.Address]map[types.Hash]types.Hash, len(m.storage))
	for addr, sMap := range m.storage {
		inner := make(map[types.Hash]types.Hash, len(sMap))
		for k, v := range sMap {
			inner[k] = v
		}
		snapStorage[addr] = inner
	}

	m.snapshots = append(m.snapshots, snapshot{
		id:       id,
		balances: snapBalances,
		nonces:   snapNonces,
		codes:    snapCodes,
		storage:  snapStorage,
		logLen:   len(m.logs),
	})

	return id
}

// RevertToSnapshot rolls back the state to the given snapshot identifier.
func (m *MemoryStateDB) RevertToSnapshot(id int) {
	idx := -1
	for i := len(m.snapshots) - 1; i >= 0; i-- {
		if m.snapshots[i].id == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}

	snap := m.snapshots[idx]
	m.balances = snap.balances
	m.nonces = snap.nonces
	m.codes = snap.codes
	m.storage = snap.storage
	if snap.logLen <= len(m.logs) {
		m.logs = m.logs[:snap.logLen]
	}

	m.snapshots = m.snapshots[:idx]
}

// AddLog appends a log entry to the buffer.
func (m *MemoryStateDB) AddLog(log *types.Log) {
	m.logs = append(m.logs, log)
}

// GetLogs returns all buffered log entries.
func (m *MemoryStateDB) GetLogs() []*types.Log {
	res := make([]*types.Log, len(m.logs))
	copy(res, m.logs)
	return res
}

// ClearLogs clears the buffered log entries.
func (m *MemoryStateDB) ClearLogs() {
	m.logs = make([]*types.Log, 0)
}

// ExportedAccount represents persistent account state.
type ExportedAccount struct {
	Balance string            `json:"balance"`
	Nonce   uint64            `json:"nonce"`
	Code    string            `json:"code,omitempty"`
	Storage map[string]string `json:"storage,omitempty"`
}

// ExportState dumps all non-empty accounts in MemoryStateDB.
func (m *MemoryStateDB) ExportState() map[string]ExportedAccount {
	out := make(map[string]ExportedAccount)
	allAddrs := make(map[types.Address]bool)

	for addr := range m.balances {
		allAddrs[addr] = true
	}
	for addr := range m.nonces {
		allAddrs[addr] = true
	}
	for addr := range m.codes {
		allAddrs[addr] = true
	}
	for addr := range m.storage {
		allAddrs[addr] = true
	}

	for addr := range allAddrs {
		acc := ExportedAccount{
			Balance: "0",
			Nonce:   m.nonces[addr],
		}
		if bal, ok := m.balances[addr]; ok && bal != nil {
			acc.Balance = bal.Dec()
		}
		if code, ok := m.codes[addr]; ok && len(code) > 0 {
			acc.Code = hex.EncodeToString(code)
		}
		if st, ok := m.storage[addr]; ok && len(st) > 0 {
			acc.Storage = make(map[string]string)
			for k, v := range st {
				acc.Storage[k.Hex()] = v.Hex()
			}
		}
		out[addr.String()] = acc
	}
	return out
}

// ImportState restores accounts into MemoryStateDB.
func (m *MemoryStateDB) ImportState(accounts map[string]ExportedAccount) error {
	for addrStr, acc := range accounts {
		addr, err := types.HexToAddress(addrStr)
		if err != nil {
			continue
		}
		bal, _ := uint256.FromDecimal(acc.Balance)
		if bal != nil {
			m.balances[addr] = bal
		}
		m.nonces[addr] = acc.Nonce
		if len(acc.Code) > 0 {
			codeBytes, err := hex.DecodeString(acc.Code)
			if err == nil {
				m.codes[addr] = codeBytes
			}
		}
		if len(acc.Storage) > 0 {
			m.storage[addr] = make(map[types.Hash]types.Hash)
			for kStr, vStr := range acc.Storage {
				k, errK := types.HexToHash(kStr)
				v, errV := types.HexToHash(vStr)
				if errK == nil && errV == nil {
					m.storage[addr][k] = v
				}
			}
		}
	}
	return nil
}


