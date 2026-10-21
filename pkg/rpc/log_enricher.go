package rpc

import (
	"encoding/hex"
	"fmt"

	"github.com/ghks18g/chord-go/pkg/types"
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
