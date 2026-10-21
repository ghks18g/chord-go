package vm

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

type FaucetArtifact struct {
	DeployedBytecode string `json:"deployedBytecode"`
}

func TestEVM_FaucetReceive(t *testing.T) {
	data, err := os.ReadFile("../../contracts/ChordFaucet.json")
	if err != nil {
		t.Fatalf("failed to read artifact: %v", err)
	}

	var artifact FaucetArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("failed to unmarshal artifact: %v", err)
	}

	cleanHex := artifact.DeployedBytecode
	if len(cleanHex) >= 2 && cleanHex[:2] == "0x" {
		cleanHex = cleanHex[2:]
	}
	code, err := hex.DecodeString(cleanHex)
	if err != nil {
		t.Fatalf("failed to decode bytecode: %v", err)
	}

	state := NewMemoryStateDB()
	faucetAddr, _ := types.HexToAddress("0x4c219b464eb113f4d3c7ce0a438fb2e99f6b07b6")
	treasuryAddr, _ := types.HexToAddress("0x470cC21093319e612E9779cE2c1d53b0F84562F5")

	val, _ := uint256.FromDecimal("1000000000000000000000000") // 1M CHOR
	state.SetBalance(treasuryAddr, val)
	state.SetCode(faucetAddr, code)

	evm := NewEVM(state, Context{
		Caller:      treasuryAddr,
		To:          faucetAddr,
		Origin:      treasuryAddr,
		BlockNumber: 3,
		Value:       val,
	})

	ret, gasLeft, err := evm.Execute(code, []byte{}, 10_000_000)
	t.Logf("ret: %x, gasLeft: %d, err: %v", ret, gasLeft, err)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
}
