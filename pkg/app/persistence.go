package app

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ghks18g/chord-go/pkg/crypto/signature"
	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/ghks18g/chord-go/pkg/vm"
)

type PersistentAppState struct {
	Height    uint64                        `json:"height"`
	StateRoot string                        `json:"state_root"`
	Accounts  map[string]vm.ExportedAccount `json:"accounts"`
}

// SetHeight sets the block height (used during state restore).
func (a *App) SetHeight(h uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.height = h
}

// SaveState saves the current state to dataDir/state.json atomically.
func (a *App) SaveState(dataDir string) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	state := PersistentAppState{
		Height:    a.height,
		StateRoot: hex.EncodeToString(a.stateRoot[:]),
		Accounts:  a.state.ExportState(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal app state: %w", err)
	}

	targetPath := filepath.Join(dataDir, "state.json")
	tmpPath := targetPath + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tmp state file: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to commit state file: %w", err)
	}

	return nil
}

// LoadState loads the state from dataDir/state.json if it exists.
// Returns (true, nil) if successfully restored, (false, nil) if file does not exist.
func (a *App) LoadState(dataDir string) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	targetPath := filepath.Join(dataDir, "state.json")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read state file: %w", err)
	}

	var state PersistentAppState
	if err := json.Unmarshal(data, &state); err != nil {
		return false, fmt.Errorf("failed to unmarshal state file: %w", err)
	}

	// Restore height
	a.height = state.Height

	// Restore StateDB accounts
	if err := a.state.ImportState(state.Accounts); err != nil {
		return false, fmt.Errorf("failed to import state accounts: %w", err)
	}

	// Rebuild SMT leaves from restored accounts
	for addrStr, acc := range state.Accounts {
		addr, err := types.HexToAddress(addrStr)
		if err != nil {
			continue
		}
		var key [32]byte
		copy(key[12:], addr[:])

		if len(acc.Code) > 0 {
			codeBytes, _ := hex.DecodeString(acc.Code)
			codeHash := signature.Keccak256(codeBytes)
			a.tree.Update(key, codeHash)
		} else {
			bal := a.state.GetBalance(addr)
			if bal != nil && !bal.IsZero() {
				bBytes := bal.Bytes()
				var bVal [32]byte
				copy(bVal[32-len(bBytes):], bBytes)
				a.tree.Update(key, bVal)
			}
		}
	}
	a.stateRoot = a.tree.Root()

	return true, nil
}
