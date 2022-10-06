package main

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/core/beacon"
	"github.com/prysmaticlabs/prysm/v3/proto/builder"
	// "github.com/ethereum/go-ethereum/common/hexutil"
	// "github.com/flashbots/go-boost-utils/types"
)

type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params,omitempty"`
	ID      int    `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  any    `json:"result"`
}

// PayloadID is an identifier of the payload build process
type PayloadID [8]byte

type PayloadStatusV1 = beacon.PayloadStatusV1

type ForkChoiceResponse = beacon.ForkChoiceResponse

type BuilderPayloadAttributes = builder.BuilderPayloadAttributes

func (req *JSONRPCRequest) UnmarshalJSON(data []byte) error {
	var msg struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
		ID      int             `json:"id"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	var params []any
	switch msg.Method {
	case builderAttributes:
		var payloadParams []*BuilderPayloadAttributes
		if err := json.Unmarshal(msg.Params, &payloadParams); err != nil {
			return err
		}
		for _, p := range payloadParams {
			params = append(params, p)
		}
	default:
		if msg.Params != nil {
			if err := json.Unmarshal(msg.Params, &params); err != nil {
				return err
			}
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
