package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/ghks18g/chord-go/pkg/app"
	"github.com/ghks18g/chord-go/pkg/bft"
	"github.com/ghks18g/chord-go/pkg/rpc"
	"github.com/ghks18g/chord-go/pkg/types"
)

func main() {
	port := flag.Int("port", 8545, "RPC server port")
	chainID := flag.String("chain-id", "chord_7613-1", "Cosmos chain ID")
	valID := flag.Int("val-id", 1, "Validator ID (1 to 4)")
	flag.Parse()

	fmt.Println("==================================================")
	fmt.Printf(" Starting CHORD Node Daemon (chordd)\n")
	fmt.Printf(" Chain ID       : %s (EVM Chain ID: 7613)\n", *chainID)
	fmt.Printf(" Validator ID   : Node %d (Quorum: 3-of-4)\n", *valID)
	fmt.Printf(" State Tree     : 256-depth Poseidon-based SMT\n")
	fmt.Printf(" Consensus      : On-Demand Event-Driven BFT\n")
	fmt.Printf(" RPC Port       : :%d\n", *port)
	fmt.Println("==================================================")

	// 1. Initialize State Machine with Genesis
	genesis := types.DefaultGenesis()
	appl := app.NewApp(*chainID)

	// Apply Genesis Allocations (4 Validators 25M CHOR each, Genesis Treasury 900M CHOR)
	for addr, bal := range genesis.Alloc {
		appl.SetBalance(addr, bal)
		log.Printf("[Genesis Alloc] %s funded with %s wei/note", addr.String(), bal.Dec())
	}

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

	// 3. Start Integrated RPC Server
	server := rpc.NewServer(appl, engine)
	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("[RPC] Server listening on %s (EVM JSON-RPC, CometBFT, ZK SMT Prover)", addr)

	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("RPC server fatal: %v", err)
	}
}

