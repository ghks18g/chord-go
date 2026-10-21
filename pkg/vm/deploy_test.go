package vm

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

func TestEVM_ContractDeployment(t *testing.T) {
	data, err := os.ReadFile("../../contracts/ChordFaucet.json")
	if err != nil {
		t.Fatalf("failed to read artifact: %v", err)
	}

	var artifact struct {
		Bytecode         string `json:"bytecode"`
		DeployedBytecode string `json:"deployedBytecode"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("failed to unmarshal artifact: %v", err)
	}

	// Constructor arguments: trustedForwarder (32 bytes) + initialOwner (32 bytes)
	fwdHex := "00000000000000000000000034ec2b8950c43c126d8e7f57dd67d3f134ea1933"
	ownerHex := "000000000000000000000000470cc21093319e612e9779ce2c1d53b0f84562f5"
	creationHex := artifact.Bytecode + fwdHex + ownerHex
	if creationHex[:2] == "0x" {
		creationHex = creationHex[2:]
	}

	initCode, err := hex.DecodeString(creationHex)
	if err != nil {
		t.Fatalf("failed to decode initcode: %v", err)
	}

	state := NewMemoryStateDB()
	caller, _ := types.HexToAddress("0xa983D897D00482FEEf3000Ad2BEEf358B9fA06bC")
	contractAddr, _ := types.HexToAddress("0x4c219b464eb113f4d3c7ce0a438fb2e99f6b07b6")

	evm := NewEVM(state, Context{
		Caller:      caller,
		To:          contractAddr,
		Origin:      caller,
		BlockNumber: 1,
		Value:       uint256.NewInt(0),
	})

	deployedRuntimeCode, gasLeft, err := evm.Execute(initCode, nil, 10_000_000)
	t.Logf("Deployment err: %v, gasLeft: %d, deployedLen: %d", err, gasLeft, len(deployedRuntimeCode))

	if err != nil {
		t.Fatalf("constructor failed: %v", err)
	}

	if len(deployedRuntimeCode) == 0 {
		t.Fatalf("constructor returned empty deployed bytecode")
	}

	// Now verify that the deployed runtime code can accept direct payment (receive)
	state.SetCode(contractAddr, deployedRuntimeCode)
	val, _ := uint256.FromDecimal("1000000000000000000000000") // 1M CHOR
	state.SetBalance(caller, val)
	state.SubBalance(caller, val)
	state.AddBalance(contractAddr, val)

	evm2 := NewEVM(state, Context{
		Caller:      caller,
		To:          contractAddr,
		Origin:      caller,
		BlockNumber: 2,
		Value:       val,
	})

	ret, gasLeft2, err := evm2.Execute(deployedRuntimeCode, []byte{}, 10_000_000)
	t.Logf("Receive call err: %v, gasLeft: %d, ret: %x", err, gasLeft2, ret)
	if err != nil {
		t.Fatalf("receive call failed: %v", err)
	}
}
