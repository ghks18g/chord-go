package vm

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

func TestEVM_PureFaucetClaim_NoHooks(t *testing.T) {
	data, err := os.ReadFile("../../contracts/ChordFaucet.json")
	if err != nil {
		t.Fatalf("failed to read artifact: %v", err)
	}

	var artifact struct {
		Bytecode string `json:"bytecode"`
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("failed to unmarshal artifact: %v", err)
	}

	fwdHex := "000000000000000000000000621732e9e8d6b8daa4f47ddfc735bdfecbd04244"
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
	deployer, _ := types.HexToAddress("0xa983D897D00482FEEf3000Ad2BEEf358B9fA06bC")
	faucetAddr, _ := types.HexToAddress("0x45aa31965d6c5905e8ac40a23e2a4ca473588b33")
	userAddr, _ := types.HexToAddress("0x3d142880A292e6290CB381a4d86CfD090Aca6F04")

	// 1. Deploy ChordFaucet via constructor
	evmDeploy := NewEVM(state, Context{
		Caller:      deployer,
		To:          faucetAddr,
		Origin:      deployer,
		BlockNumber: 1,
		ChainID:     7613,
		Value:       uint256.NewInt(0),
	})

	deployedRuntimeCode, _, err := evmDeploy.Execute(initCode, nil, 10_000_000)
	if err != nil {
		t.Fatalf("faucet constructor failed: %v", err)
	}
	state.SetCode(faucetAddr, deployedRuntimeCode)

	// 2. Fund Faucet with 1,000,000 CHOR (as in Genesis)
	initialFaucetBal, _ := uint256.FromDecimal("1000000000000000000000000") // 1M CHOR
	state.SetBalance(faucetAddr, initialFaucetBal)
	state.SetBalance(userAddr, uint256.NewInt(0))

	// 3. User invokes claim() -> selector 0x4e71d92d
	claimSelector, _ := hex.DecodeString("4e71d92d")
	evmClaim := NewEVM(state, Context{
		Caller:      userAddr,
		To:          faucetAddr,
		Origin:      userAddr,
		BlockNumber: 2,
		ChainID:     7613,
		Value:       uint256.NewInt(0),
	})

	ret, gasLeft, err := evmClaim.Execute(deployedRuntimeCode, claimSelector, 10_000_000)
	t.Logf("claim execution: err=%v, gasLeft=%d, ret=%x", err, gasLeft, ret)
	if err != nil {
		t.Fatalf("claim() execution failed on EVM: %v", err)
	}

	// 4. Verify user balance is now 10 CHOR!
	tenCHOR, _ := uint256.FromDecimal("10000000000000000000") // 10 CHOR
	userBal := state.GetBalance(userAddr)
	if !userBal.Eq(tenCHOR) {
		t.Fatalf("expected user balance 10 CHOR, got %s", userBal.Dec())
	}

	// 5. Verify Faucet balance was deducted by 10 CHOR
	expectedFaucetBal := new(uint256.Int).Sub(initialFaucetBal, tenCHOR)
	faucetBal := state.GetBalance(faucetAddr)
	if !faucetBal.Eq(expectedFaucetBal) {
		t.Fatalf("expected faucet balance %s, got %s", expectedFaucetBal.Dec(), faucetBal.Dec())
	}

	// 6. Verify duplicate claim() reverts in Solidity
	evmDuplicateClaim := NewEVM(state, Context{
		Caller:      userAddr,
		To:          faucetAddr,
		Origin:      userAddr,
		BlockNumber: 3,
		ChainID:     7613,
		Value:       uint256.NewInt(0),
	})
	_, _, errDup := evmDuplicateClaim.Execute(deployedRuntimeCode, claimSelector, 10_000_000)
	if errDup == nil {
		t.Fatalf("expected duplicate claim to revert, but succeeded!")
	}
	t.Logf("Duplicate claim reverted as expected: %v", errDup)
}
