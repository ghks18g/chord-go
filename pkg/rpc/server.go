package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ghks18g/chord-go/pkg/app"
	"github.com/ghks18g/chord-go/pkg/bft"
	"github.com/ghks18g/chord-go/pkg/types"
	"github.com/ghks18g/chord-go/pkg/vm"
	"github.com/holiman/uint256"
)

type TxLog struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
	BlockNumber string   `json:"blockNumber"`
	TxHash      string   `json:"transactionHash"`
	TxIndex     string   `json:"transactionIndex"`
	BlockHash   string   `json:"blockHash"`
	LogIndex    string   `json:"logIndex"`
}

type TxReceipt struct {
	TransactionHash   string   `json:"transactionHash"`
	TransactionIndex  string   `json:"transactionIndex"`
	BlockHash         string   `json:"blockHash"`
	BlockNumber       string   `json:"blockNumber"`
	From              string   `json:"from"`
	To                *string  `json:"to"`
	ContractAddress   *string  `json:"contractAddress"`
	CumulativeGasUsed string   `json:"cumulativeGasUsed"`
	GasUsed           string   `json:"gasUsed"`
	EffectiveGasPrice string   `json:"effectiveGasPrice"`
	Type              string   `json:"type"`
	Status            string   `json:"status"` // "0x1" for success
	Logs              []*TxLog `json:"logs"`
}

type RPCTransaction struct {
	Hash             string  `json:"hash"`
	Nonce            string  `json:"nonce"`
	BlockHash        string  `json:"blockHash"`
	BlockNumber      string  `json:"blockNumber"`
	TransactionIndex string  `json:"transactionIndex"`
	From             string  `json:"from"`
	To               *string `json:"to"`
	Value            string  `json:"value"`
	Gas              string  `json:"gas"`
	GasPrice         string  `json:"gasPrice"`
	Input            string  `json:"input"`
	Type             string  `json:"type"`
	V                string  `json:"v"`
	R                string  `json:"r"`
	S                string  `json:"s"`
}

type Server struct {
	app          *app.App
	engine       *bft.Engine
	mux          *http.ServeMux
	mu           sync.RWMutex
	receipts     map[string]*TxReceipt
	txs          map[string]*RPCTransaction
	blocksByHash map[string]*types.Block
	dataDir      string
}

func NewServer(application *app.App, engine *bft.Engine) *Server {
	s := &Server{
		app:          application,
		engine:       engine,
		mux:          http.NewServeMux(),
		receipts:     make(map[string]*TxReceipt),
		txs:          make(map[string]*RPCTransaction),
		blocksByHash: make(map[string]*types.Block),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	// 1. CometBFT Status
	s.mux.HandleFunc("/status", s.handleStatus)

	// 2. EVM JSON-RPC
	s.mux.HandleFunc("/", s.handleJSONRPC)

	// 3. ZK Sparse Merkle Tree Proof Query
	s.mux.HandleFunc("/prove_account", s.handleProveAccount)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"node_info": map[string]string{
			"network": "chord_7613-1",
			"version": "v0.1.0",
		},
		"sync_info": map[string]interface{}{
			"latest_block_height": s.app.Height(),
			"latest_app_hash":     fmt.Sprintf("0x%x", s.app.StateRoot()),
		},
	})
}

type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      interface{}   `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

func cleanHex(h string) string {
	if strings.HasPrefix(h, "0x") || strings.HasPrefix(h, "0X") {
		return h[2:]
	}
	return h
}

func (s *Server) formatBlock(b *types.Block, height uint64, fullTx bool) map[string]interface{} {
	var blockHashStr string
	var parentHashStr string
	var timestampStr string
	var txList interface{}

	if b != nil {
		blockHashStr = b.Hash().Hex()
		parentHashStr = b.Header.PrevHash.Hex()
		timestampStr = fmt.Sprintf("0x%x", b.Header.Timestamp.Unix())
		height = b.Header.Height

		if fullTx {
			var rpcTxs []*RPCTransaction
			s.mu.RLock()
			for _, tx := range b.Transactions {
				h := fmt.Sprintf("0x%x", tx.Hash())
				if stored, ok := s.txs[h]; ok {
					rpcTxs = append(rpcTxs, stored)
				}
			}
			s.mu.RUnlock()
			txList = rpcTxs
		} else {
			var txHashes []string
			for _, tx := range b.Transactions {
				txHashes = append(txHashes, fmt.Sprintf("0x%x", tx.Hash()))
			}
			txList = txHashes
		}
	} else {
		blockHashStr = fmt.Sprintf("0x%064x", height)
		var prev uint64
		if height > 0 {
			prev = height - 1
		}
		parentHashStr = fmt.Sprintf("0x%064x", prev)
		timestampStr = fmt.Sprintf("0x%x", time.Now().Unix())
		txList = []string{}
	}

	if txList == nil {
		txList = []string{}
	}

	minerStr := "0xa983d897d00482feef3000ad2beef358b9fa06bc"
	if b != nil && b.Header.Proposer != (types.Address{}) {
		minerStr = strings.ToLower(b.Header.Proposer.String())
	} else if height > 0 {
		minerStr = strings.ToLower(s.engine.GetProposerForHeight(height).String())
	}

	return map[string]interface{}{
		"number":           fmt.Sprintf("0x%x", height),
		"hash":             blockHashStr,
		"parentHash":       parentHashStr,
		"nonce":            "0x0000000000000000",
		"sha3Uncles":       "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
		"logsBloom":        "0x" + strings.Repeat("0", 512),
		"transactionsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
		"stateRoot":        fmt.Sprintf("0x%x", s.app.StateRoot()),
		"receiptsRoot":     "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
		"miner":            minerStr,
		"proposer":         minerStr,
		"difficulty":       "0x1",
		"totalDifficulty":  "0x1",
		"extraData":        "0x",
		"size":             "0x3e8",
		"gasLimit":         "0x1c9c380",
		"gasUsed":          "0x5208",
		"timestamp":        timestampStr,
		"transactions":     txList,
		"uncles":           []string{},
		"baseFeePerGas":    "0x3b9aca00",
	}
}

func (s *Server) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		http.Error(w, "empty request body", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if trimmed[0] == '[' {
		var reqs []JSONRPCRequest
		if err := json.Unmarshal(trimmed, &reqs); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		responses := make([]JSONRPCResponse, len(reqs))
		for i := range reqs {
			responses[i] = s.executeRPCRequest(&reqs[i])
		}
		json.NewEncoder(w).Encode(responses)
		return
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(trimmed, &req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	resp := s.executeRPCRequest(&req)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) executeRPCRequest(req *JSONRPCRequest) JSONRPCResponse {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "net_version":
		resp.Result = "7613"

	case "web3_clientVersion":
		resp.Result = fmt.Sprintf("chordd/v0.1.0-%s-%s", runtime.GOARCH, runtime.GOOS)

	case "eth_syncing":
		resp.Result = false

	case "eth_accounts":
		resp.Result = []string{}

	case "eth_mining":
		resp.Result = false

	case "eth_hashrate":
		resp.Result = "0x0"

	case "eth_chainId":
		resp.Result = "0x1dbd" // 7613 in hex

	case "eth_blockNumber":
		resp.Result = fmt.Sprintf("0x%x", s.app.Height())

	case "eth_gasPrice":
		resp.Result = "0x3b9aca00" // 1 Gnote (1,000,000,000 note)

	case "eth_estimateGas":
		gasLimit := "0x5208" // 21,000 gas default
		if len(req.Params) > 0 {
			if txMap, ok := req.Params[0].(map[string]interface{}); ok {
				if data, ok := txMap["data"].(string); ok && len(cleanHex(data)) > 0 {
					gasLimit = "0x186a0" // 100,000 for contract interaction
				}
				if to, ok := txMap["to"].(string); !ok || to == "" || to == "0x" {
					gasLimit = "0x4c4b40" // 5,000,000 for contract deployment
				}
			}
		}
		resp.Result = gasLimit

	case "eth_getBalance":
		resp.Result = "0x0"
		if len(req.Params) > 0 {
			addrStr, _ := req.Params[0].(string)
			addr, err := types.HexToAddress(addrStr)
			if err == nil {
				bal := s.app.GetBalance(addr)
				if bal != nil && !bal.IsZero() {
					resp.Result = bal.Hex()
				}
			}
		}

	case "eth_getCode":
		if len(req.Params) > 0 {
			addrStr, _ := req.Params[0].(string)
			addr, err := types.HexToAddress(addrStr)
			if err == nil {
				code := s.app.GetCode(addr)
				if len(code) > 0 {
					resp.Result = "0x" + hex.EncodeToString(code)
				} else {
					resp.Result = "0x"
				}
			} else {
				resp.Result = "0x"
			}
		}

	case "eth_getTransactionCount":
		if len(req.Params) > 0 {
			addrStr, _ := req.Params[0].(string)
			addr, err := types.HexToAddress(addrStr)
			if err == nil {
				nonce := s.app.GetNonce(addr)
				resp.Result = fmt.Sprintf("0x%x", nonce)
			} else {
				resp.Result = "0x0"
			}
		}

	case "eth_sendTransaction", "eth_sendRawTransaction":
		if len(req.Params) > 0 {
			var fromStr, toStr, dataStr, valStr string
			var rawTxHashStr string

			if rawHex, isStr := req.Params[0].(string); isStr {
				rawBytes, err := hex.DecodeString(cleanHex(rawHex))
				if err != nil {
					resp.Error = map[string]interface{}{"code": -32000, "message": "invalid raw transaction hex"}
					break
				}
				var ethTx gethtypes.Transaction
				if err := ethTx.UnmarshalBinary(rawBytes); err != nil {
					resp.Error = map[string]interface{}{"code": -32000, "message": "failed to decode raw transaction: " + err.Error()}
					break
				}

				signer := gethtypes.LatestSignerForChainID(ethTx.ChainId())
				sender, err := gethtypes.Sender(signer, &ethTx)
				if err != nil {
					sender, err = gethtypes.HomesteadSigner{}.Sender(&ethTx)
				}
				if err != nil {
					resp.Error = map[string]interface{}{"code": -32000, "message": "failed to recover sender: " + err.Error()}
					break
				}

				fromStr = sender.Hex()
				if ethTx.To() != nil {
					toStr = ethTx.To().Hex()
				}
				dataStr = "0x" + hex.EncodeToString(ethTx.Data())
				if ethTx.Value() != nil {
					valStr = "0x" + ethTx.Value().Text(16)
				}
				rawTxHashStr = strings.ToLower(ethTx.Hash().Hex())
			} else if txMap, isMap := req.Params[0].(map[string]interface{}); isMap {
				fromStr, _ = txMap["from"].(string)
				toStr, _ = txMap["to"].(string)
				dataStr, _ = txMap["data"].(string)
				valStr, _ = txMap["value"].(string)
			} else {
				resp.Error = map[string]interface{}{"code": -32602, "message": "invalid params"}
				break
			}

			from, _ := types.HexToAddress(fromStr)
			var dataBytes []byte
			if dataStr != "" {
				b, _ := hex.DecodeString(cleanHex(dataStr))
				dataBytes = b
			}

			val := uint256.NewInt(0)
			if valStr != "" {
				cleaned := cleanHex(valStr)
				if cleaned != "" {
					val.SetBytes(hexToBytes(cleaned))
				}
			}

			var txHash [32]byte
			var contractAddr types.Address
			isDeploy := (toStr == "" || toStr == "0x" || toStr == "0x0000000000000000000000000000000000000000")

			if isDeploy {
				// 1. Contract Deployment
				addr, err := s.app.DeployContract(from, dataBytes, val)
				if err != nil {
					resp.Error = map[string]interface{}{"code": -32000, "message": err.Error()}
					break
				}
				contractAddr = addr

				// Create tx record
				tx := &types.Transaction{
					Nonce:  s.app.GetNonce(from),
					From:   from,
					To:     nil,
					Value:  val,
					Data:   dataBytes,
					TxType: 0,
				}
				if rawTxHashStr != "" {
					var rawHash types.Hash
					b, _ := hex.DecodeString(cleanHex(rawTxHashStr))
					copy(rawHash[:], b)
					tx.TxHash = &rawHash
					txHash = rawHash
				} else {
					txHash = tx.Hash()
				}

				s.engine.AddTx(tx)
				res, _ := s.engine.ProposeAndCommit(types.Address{})
				s.app.Commit()

				txHashStr := fmt.Sprintf("0x%x", txHash)
				blockNumberStr := fmt.Sprintf("0x%x", s.app.Height())
				var blockHashStr string
				if res != nil && res.Block != nil {
					blockHashStr = res.Block.Hash().Hex()
					s.mu.Lock()
					s.blocksByHash[strings.ToLower(blockHashStr)] = res.Block
					s.blocksByHash[blockHashStr] = res.Block
					s.mu.Unlock()
				} else {
					blockHashStr = fmt.Sprintf("0x%x", s.app.StateRoot())
				}
				contractAddrStr := contractAddr.String()
				fromStrClean := from.String()

				// Cache receipt with logs
				logs := EnrichDeployLogs(contractAddrStr, txHashStr, blockHashStr, blockNumberStr)

				valHex := "0x0"
				if !val.IsZero() {
					valHex = fmt.Sprintf("0x%x", val.Bytes())
				}

				inputHex := "0x"
				if len(dataBytes) > 0 {
					inputHex = "0x" + hex.EncodeToString(dataBytes)
				}

				rpcTx := &RPCTransaction{
					Hash:             txHashStr,
					Nonce:            fmt.Sprintf("0x%x", tx.Nonce),
					BlockHash:        blockHashStr,
					BlockNumber:      blockNumberStr,
					TransactionIndex: "0x0",
					From:             fromStrClean,
					To:               nil,
					Value:            valHex,
					Gas:              "0x5208",
					GasPrice:         "0x3b9aca00",
					Input:            inputHex,
					Type:             "0x0",
					V:                "0x1b",
					R:                "0x0",
					S:                "0x0",
				}

				receiptObj := &TxReceipt{
					TransactionHash:   txHashStr,
					TransactionIndex:  "0x0",
					BlockHash:         blockHashStr,
					BlockNumber:       blockNumberStr,
					From:              fromStrClean,
					To:                nil,
					ContractAddress:   &contractAddrStr,
					CumulativeGasUsed: "0x5208",
					GasUsed:           "0x5208",
					EffectiveGasPrice: "0x3b9aca00",
					Type:              "0x2",
					Status:            "0x1",
					Logs:              logs,
				}
				if rawTxHashStr != "" {
					receiptObj.TransactionHash = rawTxHashStr
					rpcTx.Hash = rawTxHashStr
				}

				s.mu.Lock()
				s.receipts[txHashStr] = receiptObj
				s.txs[txHashStr] = rpcTx
				if rawTxHashStr != "" {
					s.receipts[rawTxHashStr] = receiptObj
					s.txs[rawTxHashStr] = rpcTx
				}
				s.mu.Unlock()
				go func() { _ = s.Persist() }()
			} else {
				// 2. Regular Transfer or Contract Call
				to, _ := types.HexToAddress(toStr)
				tx := &types.Transaction{
					Nonce:  s.app.GetNonce(from),
					From:   from,
					To:     &to,
					Value:  val,
					Data:   dataBytes,
					TxType: 0,
				}
				if rawTxHashStr != "" {
					var rawHash types.Hash
					b, _ := hex.DecodeString(cleanHex(rawTxHashStr))
					copy(rawHash[:], b)
					tx.TxHash = &rawHash
					txHash = rawHash
				} else {
					txHash = tx.Hash()
				}

				if err := s.app.ExecuteTransaction(tx); err != nil {
					resp.Error = map[string]interface{}{"code": -32000, "message": err.Error()}
					break
				}

				s.engine.AddTx(tx)
				res, _ := s.engine.ProposeAndCommit(types.Address{})
				s.app.Commit()

				txHashStr := fmt.Sprintf("0x%x", txHash)
				blockNumberStr := fmt.Sprintf("0x%x", s.app.Height())
				var blockHashStr string
				if res != nil && res.Block != nil {
					blockHashStr = res.Block.Hash().Hex()
					s.mu.Lock()
					s.blocksByHash[strings.ToLower(blockHashStr)] = res.Block
					s.blocksByHash[blockHashStr] = res.Block
					s.mu.Unlock()
				} else {
					blockHashStr = fmt.Sprintf("0x%x", s.app.StateRoot())
				}
				fromStrClean := from.String()
				toStrClean := to.String()

				var logs []*TxLog
				if s.app.StateDB() != nil && len(s.app.StateDB().GetLogs()) > 0 {
					logs = ConvertOnChainLogs(s.app.StateDB().GetLogs(), txHashStr, blockHashStr, s.app.Height())
				} else {
					logs = EnrichTransactionLogs(fromStrClean, toStrClean, val, dataStr, txHashStr, blockHashStr, blockNumberStr)
				}

				valHex := "0x0"
				if !val.IsZero() {
					valHex = fmt.Sprintf("0x%x", val.Bytes())
				}

				inputHex := "0x"
				if len(dataBytes) > 0 {
					inputHex = "0x" + hex.EncodeToString(dataBytes)
				}

				rpcTx := &RPCTransaction{
					Hash:             txHashStr,
					Nonce:            fmt.Sprintf("0x%x", tx.Nonce),
					BlockHash:        blockHashStr,
					BlockNumber:      blockNumberStr,
					TransactionIndex: "0x0",
					From:             fromStrClean,
					To:               &toStrClean,
					Value:            valHex,
					Gas:              "0x5208",
					GasPrice:         "0x3b9aca00",
					Input:            inputHex,
					Type:             "0x0",
					V:                "0x1b",
					R:                "0x0",
					S:                "0x0",
				}

				receiptObj := &TxReceipt{
					TransactionHash:   txHashStr,
					TransactionIndex:  "0x0",
					BlockHash:         blockHashStr,
					BlockNumber:       blockNumberStr,
					From:              fromStrClean,
					To:                &toStrClean,
					CumulativeGasUsed: "0x5208",
					GasUsed:           "0x5208",
					EffectiveGasPrice: "0x3b9aca00",
					Type:              "0x2",
					Status:            "0x1",
					Logs:              logs,
				}
				if rawTxHashStr != "" {
					receiptObj.TransactionHash = rawTxHashStr
					rpcTx.Hash = rawTxHashStr
				}

				s.mu.Lock()
				s.receipts[txHashStr] = receiptObj
				s.txs[txHashStr] = rpcTx
				if rawTxHashStr != "" {
					s.receipts[rawTxHashStr] = receiptObj
					s.txs[rawTxHashStr] = rpcTx
				}
				s.mu.Unlock()
				go func() { _ = s.Persist() }()
			}

			if rawTxHashStr != "" {
				resp.Result = rawTxHashStr
			} else {
				resp.Result = fmt.Sprintf("0x%x", txHash)
			}
		}

	case "eth_getTransactionByHash":
		if len(req.Params) > 0 {
			txHash, _ := req.Params[0].(string)
			txHashLower := strings.ToLower(txHash)
			s.mu.RLock()
			rpcTx, exists := s.txs[txHash]
			if !exists {
				rpcTx, exists = s.txs[txHashLower]
			}
			s.mu.RUnlock()

			if exists {
				resp.Result = rpcTx
			} else {
				resp.Result = nil
			}
		}

	case "eth_getTransactionReceipt":
		if len(req.Params) > 0 {
			txHash, _ := req.Params[0].(string)
			txHashLower := strings.ToLower(txHash)
			s.mu.RLock()
			receipt, exists := s.receipts[txHash]
			if !exists {
				receipt, exists = s.receipts[txHashLower]
			}
			s.mu.RUnlock()

			if exists && receipt != nil {
				resp.Result = receipt
			} else {
				resp.Result = nil
			}
		}

	case "eth_feeHistory":
		blockCount := 4
		if len(req.Params) > 0 {
			if s, ok := req.Params[0].(string); ok {
				cleaned := cleanHex(s)
				if val, err := strconv.ParseInt(cleaned, 16, 64); err == nil && val > 0 {
					blockCount = int(val)
				}
			} else if f, ok := req.Params[0].(float64); ok && f > 0 {
				blockCount = int(f)
			}
		}
		if blockCount > 1024 {
			blockCount = 1024
		}
		baseFee := "0x3b9aca00"
		baseFees := make([]string, blockCount+1)
		gasUsedRatios := make([]float64, blockCount)
		rewards := make([][]string, blockCount)
		for i := 0; i < blockCount; i++ {
			baseFees[i] = baseFee
			gasUsedRatios[i] = 0.0
			rewards[i] = []string{"0x0"}
		}
		baseFees[blockCount] = baseFee

		resp.Result = map[string]interface{}{
			"oldestBlock":   "0x1",
			"baseFeePerGas": baseFees,
			"gasUsedRatio":  gasUsedRatios,
			"reward":        rewards,
		}

	case "eth_maxPriorityFeePerGas":
		resp.Result = "0x3b9aca00"

	case "eth_getBlockByNumber":
		height := s.app.Height()
		fullTx := false
		if len(req.Params) > 0 {
			hStr, _ := req.Params[0].(string)
			if hStr != "latest" && hStr != "pending" {
				height = HexToUint64(hStr)
			}
		}
		if len(req.Params) > 1 {
			if b, ok := req.Params[1].(bool); ok {
				fullTx = b
			}
		}

		b, found := s.engine.GetBlock(height)
		if found && b != nil {
			resp.Result = s.formatBlock(b, height, fullTx)
		} else {
			resp.Result = s.formatBlock(nil, height, fullTx)
		}

	case "eth_getBlockByHash":
		var hashStr string
		fullTx := false
		if len(req.Params) > 0 {
			hashStr, _ = req.Params[0].(string)
		}
		if len(req.Params) > 1 {
			if b, ok := req.Params[1].(bool); ok {
				fullTx = b
			}
		}

		s.mu.RLock()
		b, exists := s.blocksByHash[strings.ToLower(hashStr)]
		if !exists {
			b, exists = s.blocksByHash[hashStr]
		}
		s.mu.RUnlock()

		if exists && b != nil {
			resp.Result = s.formatBlock(b, b.Header.Height, fullTx)
		} else {
			resp.Result = s.formatBlock(nil, s.app.Height(), fullTx)
		}

	case "eth_call":
		var toStr, dataStr, fromStr, valStr string
		if len(req.Params) > 0 {
			if callMap, isMap := req.Params[0].(map[string]interface{}); isMap {
				toStr, _ = callMap["to"].(string)
				dataStr, _ = callMap["data"].(string)
				if dataStr == "" {
					dataStr, _ = callMap["input"].(string)
				}
				fromStr, _ = callMap["from"].(string)
				valStr, _ = callMap["value"].(string)
			}
		}

		toAddr, err := types.HexToAddress(toStr)
		var fromAddr types.Address
		if fromStr != "" {
			fromAddr, _ = types.HexToAddress(fromStr)
		}

		var dataBytes []byte
		if dataStr != "" {
			b, _ := hex.DecodeString(cleanHex(dataStr))
			dataBytes = b
		}

		val := uint256.NewInt(0)
		if valStr != "" {
			cleaned := cleanHex(valStr)
			if cleaned != "" {
				val.SetBytes(hexToBytes(cleaned))
			}
		}

		if err == nil {
			// 1. Precompile dispatch
			if precompile, isPrecompile := vm.PrecompiledContracts[toAddr]; isPrecompile {
				res, runErr := precompile.Run(dataBytes)
				if runErr != nil {
					resp.Error = map[string]interface{}{"code": -32000, "message": runErr.Error()}
				} else if len(res) == 0 {
					resp.Result = "0x"
				} else {
					resp.Result = "0x" + hex.EncodeToString(res)
				}
				break
			}

			// 2. Contract bytecode execution
			code := s.app.GetCode(toAddr)
			if len(code) > 0 {
				ctx := vm.Context{
					Caller:      fromAddr,
					To:          toAddr,
					Origin:      fromAddr,
					BlockNumber: int64(s.app.Height()),
					Value:       val,
				}
				evm := vm.NewEVM(s.app.StateDB(), ctx)
				gasLimit := uint64(50_000_000)
				res, _, runErr := evm.Execute(code, dataBytes, gasLimit)
				if runErr != nil && !errors.Is(runErr, vm.ErrExecutionReverted) {
					resp.Error = map[string]interface{}{"code": -32000, "message": runErr.Error()}
				} else if len(res) == 0 {
					resp.Result = "0x"
				} else {
					resp.Result = "0x" + hex.EncodeToString(res)
				}
				break
			}
		}

		// Fallback mock EVM call response for view methods
		resp.Result = "0x0000000000000000000000000000000000000000000000000000000000000001"

	default:
		resp.Result = "0x0"
	}

	return resp
}

func hexToBytes(h string) []byte {
	if len(h)%2 != 0 {
		h = "0" + h
	}
	b, _ := hex.DecodeString(h)
	return b
}

func (s *Server) handleProveAccount(w http.ResponseWriter, r *http.Request) {
	addrStr := r.URL.Query().Get("address")
	addr, err := types.HexToAddress(addrStr)
	if err != nil {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}

	proof, err := s.app.ProveAccount(addr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	bal := s.app.GetBalance(addr)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"address":    addr.String(),
		"balance":    bal.String(),
		"state_root": fmt.Sprintf("0x%x", s.app.StateRoot()),
		"proof":      proof,
	})
}

// Helper to construct test server
func NewTestServer(application *app.App, engine *bft.Engine) *httptest.Server {
	srv := NewServer(application, engine)
	return httptest.NewServer(srv.Handler())
}

func HexToUint64(hexStr string) uint64 {
	if len(hexStr) >= 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}
	val, _ := strconv.ParseUint(hexStr, 16, 64)
	return val
}

// Suppress unused warning
var _ = uint256.NewInt
