package abci

import (
	"errors"

	"github.com/ghks18g/chord-go/pkg/app"
	"github.com/ghks18g/chord-go/pkg/types"
)

// RequestPrepareProposal is sent to the proposer to construct a block proposal.
type RequestPrepareProposal struct {
	MaxTxBytes int64
	Txs        []*types.Transaction
}

type ResponsePrepareProposal struct {
	Txs []*types.Transaction
}

// RequestProcessProposal is sent to all validators to validate a proposed block.
type RequestProcessProposal struct {
	Header types.Header
	Txs    []*types.Transaction
}

type ResponseProcessProposal struct {
	Status int // 1: ACCEPT, 2: REJECT
}

const (
	StatusAccept = 1
	StatusReject = 2
)

// RequestFinalizeBlock is sent to execute transactions in the finalized block.
type RequestFinalizeBlock struct {
	Header types.Header
	Txs    []*types.Transaction
}

type ResponseFinalizeBlock struct {
	AppHash   [32]byte
	TxResults []error
}

// Handler implements the ABCI++ interface for CHORD.
type Handler struct {
	app *app.App
}

func NewHandler(application *app.App) *Handler {
	return &Handler{app: application}
}

func (h *Handler) PrepareProposal(req *RequestPrepareProposal) (*ResponsePrepareProposal, error) {
	// Filter valid transactions up to max limit
	return &ResponsePrepareProposal{Txs: req.Txs}, nil
}

func (h *Handler) ProcessProposal(req *RequestProcessProposal) (*ResponseProcessProposal, error) {
	if req.Header.ChainID == "" {
		return &ResponseProcessProposal{Status: StatusReject}, errors.New("empty chain id")
	}
	return &ResponseProcessProposal{Status: StatusAccept}, nil
}

func (h *Handler) FinalizeBlock(req *RequestFinalizeBlock) (*ResponseFinalizeBlock, error) {
	results := make([]error, len(req.Txs))
	for i, tx := range req.Txs {
		results[i] = h.app.ExecuteTransaction(tx)
	}

	appHash := h.app.StateRoot()
	return &ResponseFinalizeBlock{
		AppHash:   appHash,
		TxResults: results,
	}, nil
}

func (h *Handler) Commit() [32]byte {
	return h.app.Commit()
}
