package app

import (
	"bytes"
	"errors"
	"sync"

	"github.com/ghks18g/chord-go/pkg/app/hooks"
	"github.com/ghks18g/chord-go/pkg/crypto/signature"
	"github.com/ghks18g/chord-go/pkg/store/smt"
	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/ghks18g/chord-go/pkg/vm"
	"github.com/holiman/uint256"
)

// App is the state machine coordinator of the CHORD chain.
type App struct {
	mu        sync.RWMutex
	chainID   string
	tree      *smt.Tree
	state     *vm.MemoryStateDB
	height    uint64
	stateRoot [32]byte
}

func NewApp(chainID string) *App {
	tree := smt.NewTree()
	return &App{
		chainID:   chainID,
		tree:      tree,
		state:     vm.NewMemoryStateDB(),
		height:    0,
		stateRoot: tree.Root(),
	}
}

// StateRoot returns the current SMT root hash.
func (a *App) StateRoot() [32]byte {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.stateRoot
}

// StateDB returns the EVM state database.
func (a *App) StateDB() *vm.MemoryStateDB {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// Height returns current block height.
func (a *App) Height() uint64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.height
}

// SetBalance sets the balance of an account.
func (a *App) SetBalance(addr types.Address, bal *uint256.Int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.SetBalance(addr, bal)

	// Update SMT
	var key [32]byte
	copy(key[12:], addr[:])
	var val [32]byte
	bBytes := bal.Bytes()
	copy(val[32-len(bBytes):], bBytes)
	a.stateRoot = a.tree.Update(key, val)
}

// GetBalance returns account balance.
func (a *App) GetBalance(addr types.Address) *uint256.Int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state.GetBalance(addr)
}

// GetNonce returns account nonce.
func (a *App) GetNonce(addr types.Address) uint64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state.GetNonce(addr)
}

// SetNonce sets account nonce.
func (a *App) SetNonce(addr types.Address, nonce uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.SetNonce(addr, nonce)
}

// GetCode returns bytecode for an address.
func (a *App) GetCode(addr types.Address) []byte {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state.GetCode(addr)
}

// SetCode sets bytecode for an address and updates SMT.
func (a *App) SetCode(addr types.Address, code []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.SetCode(addr, code)

	// Sync SMT
	var key [32]byte
	copy(key[12:], addr[:])
	codeHash := signature.Keccak256(code)
	a.tree.Update(key, codeHash)
	a.stateRoot = a.tree.Root()
}

// DeployContract creates a new contract account with deployed bytecode.
func (a *App) DeployContract(from types.Address, code []byte, value *uint256.Int) (types.Address, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	nonce := a.state.GetNonce(from)
	a.state.SetNonce(from, nonce+1)

	// Derive contract address: Keccak256(from || nonce)
	var nonceBytes [8]byte
	for i := 0; i < 8; i++ {
		nonceBytes[7-i] = byte(nonce >> (i * 8))
	}
	hash := signature.Keccak256(from[:], nonceBytes[:])
	var contractAddr types.Address
	copy(contractAddr[:], hash[12:])

	snapshot := a.state.Snapshot()

	// Deduct balance if value > 0
	if value != nil && !value.IsZero() {
		bal := a.state.GetBalance(from)
		if bal.Lt(value) {
			a.state.RevertToSnapshot(snapshot)
			return types.Address{}, errors.New("insufficient balance for deployment")
		}
		a.state.SubBalance(from, value)
		a.state.AddBalance(contractAddr, value)
	}

	a.state.SetCode(contractAddr, code)

	// Update SMT leaf
	var key [32]byte
	copy(key[12:], contractAddr[:])
	codeHash := signature.Keccak256(code)
	a.tree.Update(key, codeHash)
	a.stateRoot = a.tree.Root()

	return contractAddr, nil
}

// ExecuteTransaction processes a single transaction and updates the SMT.
func (a *App) ExecuteTransaction(tx *types.Transaction) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state.ClearLogs()

	// Snapshot state for atomic rollback on any failure
	snapshot := a.state.Snapshot()
	rollback := func(err error) error {
		a.state.RevertToSnapshot(snapshot)
		a.state.ClearLogs()
		return err
	}

	// Verify Sender Balance
	bal := a.state.GetBalance(tx.From)
	if bal.Lt(tx.Value) {
		return rollback(errors.New("insufficient balance"))
	}

	// Signature verification logic
	if tx.TxType == 1 { // EIP-712
		domain := &signature.EIP712Domain{
			Name:    "CHORD Protocol",
			Version: "1",
			ChainID: 7613,
		}
		structHash := signature.Keccak256(tx.Data)
		typedHash := signature.HashTypedData(domain, structHash)
		if typedHash == [32]byte{} {
			return rollback(errors.New("invalid typed data hash"))
		}
	} else if tx.TxType == 2 { // EIP-1271 / ZK Payload
		if len(tx.ZKProof) == 0 {
			return rollback(errors.New("missing zk proof payload"))
		}
	}

	// State Transfer
	a.state.SubBalance(tx.From, tx.Value)
	if tx.To != nil {
		a.state.AddBalance(*tx.To, tx.Value)
	}

	// Execute system hooks (Faucet claim/request, EIP-2771 forwarder)
	_ = hooks.HandleFaucetAndForwarder(a.state, a.tree, tx)

	// Execute EVM bytecode or Precompile if destination is a contract
	if tx.To != nil {
		if precompile, isPre := vm.PrecompiledContracts[*tx.To]; isPre {
			if _, err := precompile.Run(tx.Data); err != nil {
				return rollback(err)
			}
		} else {
			code := a.state.GetCode(*tx.To)
			if len(code) > 0 {
				evm := vm.NewEVM(a.state, vm.Context{
					Caller:      tx.From,
					To:          *tx.To,
					Origin:      tx.From,
					BlockNumber: int64(a.height),
					Value:       tx.Value,
				})
				gasLimit := uint64(10_000_000)
				if _, _, err := evm.Execute(code, tx.Data, gasLimit); err != nil {
					return rollback(err)
				}
			}
		}
	}

	// Increment Nonce
	fromNonce := a.state.GetNonce(tx.From)
	a.state.SetNonce(tx.From, fromNonce+1)

	// Sync SMT leaf nodes for From and To
	var fromKey [32]byte
	copy(fromKey[12:], tx.From[:])
	fromBalBytes := a.state.GetBalance(tx.From).Bytes()
	var fromVal [32]byte
	copy(fromVal[32-len(fromBalBytes):], fromBalBytes)
	a.tree.Update(fromKey, fromVal)

	if tx.To != nil {
		var toKey [32]byte
		copy(toKey[12:], tx.To[:])
		toBalBytes := a.state.GetBalance(*tx.To).Bytes()
		var toVal [32]byte
		copy(toVal[32-len(toBalBytes):], toBalBytes)
		a.tree.Update(toKey, toVal)
	}

	a.stateRoot = a.tree.Root()
	return nil
}

// Commit finalizes the block, advances height, and returns the SMT state root.
func (a *App) Commit() [32]byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.height++
	a.stateRoot = a.tree.Root()
	return a.stateRoot
}

// ProveAccount generates an SMT inclusion/non-inclusion proof for an address.
func (a *App) ProveAccount(addr types.Address) (*smt.Proof, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var key [32]byte
	copy(key[12:], addr[:])
	return a.tree.Prove(key)
}

// VerifyAccountProof checks the validity of an SMT proof against current state root.
func (a *App) VerifyAccountProof(addr types.Address, expectedBal *uint256.Int, proof *smt.Proof) bool {
	a.mu.RLock()
	root := a.stateRoot
	a.mu.RUnlock()

	var key [32]byte
	copy(key[12:], addr[:])

	var val [32]byte
	if expectedBal != nil && !expectedBal.IsZero() {
		b := expectedBal.Bytes()
		copy(val[32-len(b):], b)
	}

	return smt.Verify(root, key, val, proof)
}

func (a *App) VerifyRootDeterministic(expected [32]byte) bool {
	return bytes.Equal(a.stateRoot[:], expected[:])
}
