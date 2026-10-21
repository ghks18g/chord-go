package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ghks18g/chord-go/pkg/app"
	"github.com/ghks18g/chord-go/pkg/bft"
	"github.com/ghks18g/chord-go/pkg/rpc"
	"github.com/ghks18g/chord-go/pkg/types"
)

func resolveDataDir(preferred string) string {
	if preferred != "" {
		if err := os.MkdirAll(preferred, 0755); err == nil {
			return preferred
		}
	}
	home, err := os.UserHomeDir()
	if err == nil {
		userChord := filepath.Join(home, ".chord")
		if err := os.MkdirAll(userChord, 0755); err == nil {
			return userChord
		}
	}
	localChord := "./.chord"
	_ = os.MkdirAll(localChord, 0755)
	return localChord
}

func main() {
	port := flag.Int("port", 8545, "RPC server port")
	chainID := flag.String("chain-id", "chord_7613-1", "Cosmos chain ID")
	valID := flag.Int("val-id", 1, "Validator ID (1 to 4)")
	dataDirFlag := flag.String("data-dir", "/root/.chord", "Node persistent data directory")
	flag.Parse()

	dataDir := resolveDataDir(*dataDirFlag)

	fmt.Println("==================================================")
	fmt.Printf(" Starting CHORD Node Daemon (chordd)\n")
	fmt.Printf(" Chain ID       : %s (EVM Chain ID: 7613)\n", *chainID)
	fmt.Printf(" Validator ID   : Node %d (Quorum: 3-of-4)\n", *valID)
	fmt.Printf(" State Tree     : 256-depth Poseidon-based SMT\n")
	fmt.Printf(" Consensus      : On-Demand Event-Driven BFT\n")
	fmt.Printf(" Data Directory : %s\n", dataDir)
	fmt.Printf(" RPC Port       : :%d\n", *port)
	fmt.Println("==================================================")

	// 1. Initialize State Machine
	genesis := types.DefaultGenesis()
	appl := app.NewApp(*chainID)

	// 2. Initialize 4-Node BFT Validators (25.0% Weight Each)
	validators := make([]*bft.Validator, len(genesis.Validators))
	for i, val := range genesis.Validators {
		validators[i] = &bft.Validator{
			ID:          val.ID,
			Address:     val.Address,
			VotingPower: val.VotingPower,
		}
		if val.ID == *valID {
			fmt.Printf(" Validator Role : %s (%s)\n", val.Name, val.Address.String())
			fmt.Printf(" Voting Power   : %d (25.0%% Weight)\n", val.VotingPower)
		}
	}
	engine := bft.NewEngine(validators, false) // create_empty_blocks = false

	// 3. Initialize RPC Server & Persistence
	server := rpc.NewServer(appl, engine)
	server.SetDataDir(dataDir)

	// 4. Restore state if exists, otherwise apply genesis
	restored, err := server.Restore()
	if err != nil {
		log.Printf("[State WARN] Failed to restore state from %s: %v. Re-initializing...", dataDir, err)
		restored = false
	}

	if restored {
		log.Printf("[State] Restored persistent state from %s (Height: #%d, StateRoot: 0x%x)", dataDir, appl.Height(), appl.StateRoot())
	} else {
		log.Printf("[State] No previous state found at %s. Initializing Genesis Allocations...", dataDir)
		for addr, bal := range genesis.Alloc {
			appl.SetBalance(addr, bal)
			log.Printf("[Genesis Alloc] %s funded with %s wei/note", addr.String(), bal.Dec())
		}
		if err := server.Persist(); err != nil {
			log.Printf("[State WARN] Initial persist failed: %v", err)
		}
	}

	// 5. Start Integrated RPC Server
	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("[RPC] Server listening on %s (EVM JSON-RPC, CometBFT, ZK SMT Prover)", addr)

	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("RPC server fatal: %v", err)
	}
}
