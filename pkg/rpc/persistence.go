package rpc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghks18g/chord-go/pkg/types"
)

type PersistentRPCData struct {
	Receipts     map[string]*TxReceipt      `json:"receipts"`
	Transactions map[string]*RPCTransaction `json:"transactions"`
	Blocks       []*types.Block             `json:"blocks"`
}

// SetDataDir sets the storage directory for persistence.
func (s *Server) SetDataDir(dir string) {
	s.mu.Lock()
	s.dataDir = dir
	s.mu.Unlock()
}

// Persist saves state and RPC data to disk.
func (s *Server) Persist() error {
	s.mu.RLock()
	dir := s.dataDir
	s.mu.RUnlock()

	if dir == "" {
		return nil
	}

	// 1. Save App state (accounts, nonces, code, storage, height)
	if err := s.app.SaveState(dir); err != nil {
		return fmt.Errorf("failed to save app state: %w", err)
	}

	// 2. Save RPC transactions, receipts, and BFT blocks
	s.mu.RLock()
	blocks := s.engine.ExportBlocks()
	data := PersistentRPCData{
		Receipts:     s.receipts,
		Transactions: s.txs,
		Blocks:       blocks,
	}
	s.mu.RUnlock()

	bytesData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal rpc data: %w", err)
	}

	targetPath := filepath.Join(dir, "rpc_data.json")
	tmpPath := targetPath + ".tmp"
	if err := os.WriteFile(tmpPath, bytesData, 0644); err != nil {
		return fmt.Errorf("failed to write rpc data tmp file: %w", err)
	}

	return os.Rename(tmpPath, targetPath)
}

// Restore loads blocks, transactions, receipts, and state from disk.
// Returns (true, nil) if successfully restored, (false, nil) if no existing state was found.
func (s *Server) Restore() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dataDir == "" {
		return false, nil
	}

	restored, err := s.app.LoadState(s.dataDir)
	if err != nil {
		return false, fmt.Errorf("failed to load app state: %w", err)
	}
	if !restored {
		return false, nil
	}

	targetPath := filepath.Join(s.dataDir, "rpc_data.json")
	dataBytes, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("failed to read rpc data file: %w", err)
	}

	var pData PersistentRPCData
	if err := json.Unmarshal(dataBytes, &pData); err != nil {
		return false, fmt.Errorf("failed to unmarshal rpc data: %w", err)
	}

	if pData.Receipts != nil {
		for k, v := range pData.Receipts {
			s.receipts[k] = v
		}
	}
	if pData.Transactions != nil {
		for k, v := range pData.Transactions {
			s.txs[k] = v
		}
	}
	if len(pData.Blocks) > 0 {
		s.engine.RestoreBlocks(pData.Blocks)
		for _, b := range pData.Blocks {
			if b != nil {
				h := b.Hash().Hex()
				s.blocksByHash[h] = b
				s.blocksByHash[strings.ToLower(h)] = b
			}
		}
	}

	return true, nil
}
