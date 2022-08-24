package main

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flashbots/go-boost-utils/types"
)

type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  any    `json:"result"`
}

// PayloadID is an identifier of the payload build process
type PayloadID [8]byte

type PayloadStatusV1 struct {
	Status          string      `json:"status"`
	LatestValidHash *types.Hash `json:"latestValidHash"`
	ValidationError *string     `json:"validationError"`
}

type ForkChoiceResponse struct {
	PayloadStatus PayloadStatusV1 `json:"payloadStatus"`
	PayloadID     *PayloadID      `json:"payloadId"`
}

type PayloadAttributes struct {
	Timestamp  hexutil.Uint64 `json:"timestamp"`
	PrevRandao types.Hash     `json:"prevRandao"`
	Slot       uint64         `json:"slot"`
	BlockHash  types.Hash     `json:"blockHash"`
}

func (req *JSONRPCRequest) UnmarshalJSON(data []byte) error {
	var msg struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
		ID      int             `json:"id"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	var params []any
	switch msg.Method {
	case builderAttributes:
		var payloadParams []*PayloadAttributes
		if err := json.Unmarshal(msg.Params, &payloadParams); err != nil {
			return err
		}
		for _, p := range payloadParams {
			params = append(params, p)
		}
	default:
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return err
		}
	}
	*req = JSONRPCRequest{
		JSONRPC: msg.JSONRPC,
		Method:  msg.Method,
		Params:  params,
		ID:      msg.ID,
	}
	return nil
}
