package hooks

import (
	"encoding/hex"

	"github.com/ghks18g/chord-go/pkg/store/smt"
	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/ghks18g/chord-go/pkg/vm"
	"github.com/holiman/uint256"
)

var (
	// FaucetAddress is the canonical system faucet contract address.
	FaucetAddress = types.HexToAddressMust("0x4c219b464eb113f4d3c7ce0a438fb2e99f6b07b6")
	// ForwarderAddress is the canonical EIP-2771 trusted forwarder address.
	ForwarderAddress = types.HexToAddressMust("0x34ec2b8950c43c126d8e7f57dd67d3f134ea1933")
)

// HandleFaucetAndForwarder handles native Faucet claim/request dispatches and EIP-2771 forwarder logic.
func HandleFaucetAndForwarder(state vm.StateDB, tree *smt.Tree, tx *types.Transaction) error {
	if tx.To == nil || len(tx.Data) < 4 {
		return nil
	}

	claimAmount := uint256.NewInt(10000000000000000000) // 10 CHOR
	var recipient types.Address
	isClaimOrRequest := false

	if *tx.To == FaucetAddress {
		selector := hex.EncodeToString(tx.Data[:4])
		if selector == "4e71d92d" { // claim()
			recipient = tx.From
			// EIP-2771: if forwarded, original sender is appended to the last 20 bytes of calldata
			if len(tx.Data) >= 24 {
				copy(recipient[:], tx.Data[len(tx.Data)-20:])
			}
			isClaimOrRequest = true
		} else if selector == "27c78c42" && len(tx.Data) >= 36 { // request(address _recipient)
			copy(recipient[:], tx.Data[16:36])
			isClaimOrRequest = true
		}
	} else if *tx.To == ForwarderAddress {
		// ChordForwarder.execute((from, to, value, gas, nonce, deadline, data), signature)
		// req.from is located at calldata byte offset 80..100
		if len(tx.Data) >= 100 {
			copy(recipient[:], tx.Data[80:100])
			isClaimOrRequest = true
		}
	}

	if isClaimOrRequest && !state.GetBalance(FaucetAddress).Lt(claimAmount) {
		state.SubBalance(FaucetAddress, claimAmount)
		state.AddBalance(recipient, claimAmount)

		// Update Faucet leaf node in SMT
		var fKey [32]byte
		copy(fKey[12:], FaucetAddress[:])
		fBalBytes := state.GetBalance(FaucetAddress).Bytes()
		var fVal [32]byte
		copy(fVal[32-len(fBalBytes):], fBalBytes)
		tree.Update(fKey, fVal)

		// If recipient is different from tx.From (EIP-2771 meta-tx or request), update recipient leaf node in SMT
		if recipient != tx.From {
			var rKey [32]byte
			copy(rKey[12:], recipient[:])
			rBalBytes := state.GetBalance(recipient).Bytes()
			var rVal [32]byte
			copy(rVal[32-len(rBalBytes):], rBalBytes)
			tree.Update(rKey, rVal)
		}
	}

	return nil
}
