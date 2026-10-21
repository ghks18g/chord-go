package rpc

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/holiman/uint256"
)

// ConvertOnChainLogs transforms raw EVM state logs into RPC TxLog format.
func ConvertOnChainLogs(logs []*types.Log, txHashStr, blockHashStr string, blockNumber uint64) []*TxLog {
	res := make([]*TxLog, len(logs))
	for i, l := range logs {
		topics := make([]string, len(l.Topics))
		for j, tp := range l.Topics {
			topics[j] = tp.Hex()
		}
		dataHex := "0x"
		if len(l.Data) > 0 {
			dataHex = "0x" + hex.EncodeToString(l.Data)
		}
		res[i] = &TxLog{
			Address:     l.Address.Hex(),
			Topics:      topics,
			Data:        dataHex,
			BlockNumber: fmt.Sprintf("0x%x", blockNumber),
			TxHash:      txHashStr,
			TxIndex:     "0x0",
			BlockHash:   blockHashStr,
			LogIndex:    fmt.Sprintf("0x%x", i),
		}
	}
	return res
}

// EnrichDeployLogs generates synthetic receipt logs for special system contract deployments.
func EnrichDeployLogs(contractAddrStr, txHashStr, blockHashStr, blockNumberStr string) []*TxLog {
	logs := make([]*TxLog, 0)
	if strings.ToLower(contractAddrStr) == "0x4c219b464eb113f4d3c7ce0a438fb2e99f6b07b6" {
		// Faucet deployment transfers ownership to MultiSig Treasury
		logs = append(logs, &TxLog{
			Address: contractAddrStr,
			Topics: []string{
				"0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0",
				"0x0000000000000000000000000000000000000000000000000000000000000000",
				"0x000000000000000000000000470cc21093319e612e9779ce2c1d53b0f84562f5",
			},
			Data:        "0x",
			BlockNumber: blockNumberStr,
			TxHash:      txHashStr,
			TxIndex:     "0x0",
			BlockHash:   blockHashStr,
			LogIndex:    "0x0",
		})
	}
	return logs
}

// EnrichTransactionLogs handles legacy/system hook log emission for Faucet and Forwarder transactions.
func EnrichTransactionLogs(fromStrClean, toStrClean string, val *uint256.Int, dataStr, txHashStr, blockHashStr, blockNumberStr string) []*TxLog {
	logs := make([]*TxLog, 0)
	lowerTo := strings.ToLower(toStrClean)
	lowerFrom := strings.ToLower(fromStrClean)

	if lowerTo == "0x4c219b464eb113f4d3c7ce0a438fb2e99f6b07b6" {
		if !val.IsZero() {
			// Deposit event to ChordFaucet
			logs = append(logs, &TxLog{
				Address: toStrClean,
				Topics: []string{
					"0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c",
					"0x000000000000000000000000" + cleanHex(lowerFrom),
				},
				Data:        fmt.Sprintf("0x%064x", val.Bytes()),
				BlockNumber: blockNumberStr,
				TxHash:      txHashStr,
				TxIndex:     "0x0",
				BlockHash:   blockHashStr,
				LogIndex:    "0x0",
			})
		} else if strings.HasPrefix(cleanHex(dataStr), "4e71d92d") {
			// User calls claim() on ChordFaucet (or via EIP-2771 Forwarder)
			claimVal := uint256.NewInt(10000000000000000000) // 10 CHOR
			actualRecipient := lowerFrom
			hexClean := cleanHex(dataStr)
			isForwarded := false
			if len(hexClean) >= 48 { // 8 chars selector + 40 chars appended address (EIP-2771)
				actualRecipient = "0x" + hexClean[len(hexClean)-40:]
				isForwarded = true
			}
			// Log #0: ChordFaucet Claim
			logs = append(logs, &TxLog{
				Address: toStrClean,
				Topics: []string{
					"0x47cee97cb7acd717b3c0aa1435d004cd5b3c8c57d70dbceb4e4458bbd60e39d4",
					"0x000000000000000000000000" + cleanHex(actualRecipient),
				},
				Data:        fmt.Sprintf("0x%064x", claimVal.Bytes()),
				BlockNumber: blockNumberStr,
				TxHash:      txHashStr,
				TxIndex:     "0x0",
				BlockHash:   blockHashStr,
				LogIndex:    "0x0",
			})
			// Log #1: ChordForwarder Executed (if forwarded via EIP-2771)
			if isForwarded {
				forwarderAddr := "0x34ec2b8950c43c126d8e7f57dd67d3f134ea1933"
				logs = append(logs, &TxLog{
					Address: forwarderAddr,
					Topics: []string{
						"0xd86b4c50733cb4dee05f54868a1f4fa6cf9347c09642e5496c23508a04cb79ff",
						"0x000000000000000000000000" + cleanHex(actualRecipient),
						"0x000000000000000000000000" + cleanHex(toStrClean),
					},
					Data:        "0x000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000",
					BlockNumber: blockNumberStr,
					TxHash:      txHashStr,
					TxIndex:     "0x0",
					BlockHash:   blockHashStr,
					LogIndex:    "0x1",
				})
			}
		} else if strings.HasPrefix(cleanHex(dataStr), "27c78c42") {
			// Backend Operator / Multi-Sig calls request(address _recipient)
			claimVal := uint256.NewInt(10000000000000000000) // 10 CHOR
			actualRecipient := lowerFrom
			hexClean := cleanHex(dataStr)
			if len(hexClean) >= 72 { // 8 chars selector + 64 chars padded address
				actualRecipient = "0x" + hexClean[32:72]
			}
			logs = append(logs, &TxLog{
				Address: toStrClean,
				Topics: []string{
					"0x47cee97cb7acd717b3c0aa1435d004cd5b3c8c57d70dbceb4e4458bbd60e39d4",
					"0x000000000000000000000000" + cleanHex(actualRecipient),
				},
				Data:        fmt.Sprintf("0x%064x", claimVal.Bytes()),
				BlockNumber: blockNumberStr,
				TxHash:      txHashStr,
				TxIndex:     "0x0",
				BlockHash:   blockHashStr,
				LogIndex:    "0x0",
			})
		}
	} else if lowerTo == "0x34ec2b8950c43c126d8e7f57dd67d3f134ea1933" {
		// Direct execute call on ChordForwarder
		faucetAddr := "0x4c219b464eb113f4d3c7ce0a438fb2e99f6b07b6"
		actualRecipient := lowerFrom
		hexClean := cleanHex(dataStr)
		if len(hexClean) >= 200 {
			actualRecipient = "0x" + hexClean[160:200]
		}
		claimVal := uint256.NewInt(10000000000000000000) // 10 CHOR
		logs = append(logs, &TxLog{
			Address: faucetAddr,
			Topics: []string{
				"0x47cee97cb7acd717b3c0aa1435d004cd5b3c8c57d70dbceb4e4458bbd60e39d4",
				"0x000000000000000000000000" + cleanHex(actualRecipient),
			},
			Data:        fmt.Sprintf("0x%064x", claimVal.Bytes()),
			BlockNumber: blockNumberStr,
			TxHash:      txHashStr,
			TxIndex:     "0x0",
			BlockHash:   blockHashStr,
			LogIndex:    "0x0",
		})
		logs = append(logs, &TxLog{
			Address: toStrClean,
			Topics: []string{
				"0xd86b4c50733cb4dee05f54868a1f4fa6cf9347c09642e5496c23508a04cb79ff",
				"0x000000000000000000000000" + cleanHex(actualRecipient),
				"0x000000000000000000000000" + cleanHex(faucetAddr),
			},
			Data:        "0x000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000000",
			BlockNumber: blockNumberStr,
			TxHash:      txHashStr,
			TxIndex:     "0x0",
			BlockHash:   blockHashStr,
			LogIndex:    "0x1",
		})
	} else if lowerFrom == "0x4c219b464eb113f4d3c7ce0a438fb2e99f6b07b6" {
		// Direct dispatch from Faucet to recipient (Instant faucet API)
		logs = append(logs, &TxLog{
			Address: fromStrClean,
			Topics: []string{
				"0x47cee97cb7acd717b3c0aa1435d004cd5b3c8c57d70dbceb4e4458bbd60e39d4",
				"0x000000000000000000000000" + cleanHex(lowerTo),
			},
			Data:        fmt.Sprintf("0x%064x", val.Bytes()),
			BlockNumber: blockNumberStr,
			TxHash:      txHashStr,
			TxIndex:     "0x0",
			BlockHash:   blockHashStr,
			LogIndex:    "0x0",
		})
	}
	return logs
}
