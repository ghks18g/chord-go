package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	rpcURL := flag.String("rpc", "http://127.0.0.1:8545", "RPC URL")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Usage: chordcli [options] <command> [args]")
		fmt.Println("Commands:")
		fmt.Println("  status                 Query node status")
		fmt.Println("  prove <address>        Fetch Poseidon SMT inclusion proof for account")
		os.Exit(1)
	}

	cmd := args[0]
	switch cmd {
	case "status":
		resp, err := http.Get(*rpcURL + "/status")
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Println(string(body))

	case "prove":
		if len(args) < 2 {
			fmt.Println("Error: address required (e.g., chordcli prove 0x...)")
			os.Exit(1)
		}
		addr := args[1]
		resp, err := http.Get(fmt.Sprintf("%s/prove_account?address=%s", *rpcURL, addr))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Println(string(body))

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}
