package main

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/prysmaticlabs/prysm/v4/proto/builder"
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

type PayloadStatusV1 = engine.PayloadStatusV1 // same response as newPayloadV2

type ForkChoiceResponse = engine.ForkChoiceResponse // same response as forkchoiceUpdatedV2

type BuilderPayloadAttributes = builder.BuilderPayloadAttributes // only interested in slot number, no need to unmarshal v2 withdrawals

type PayloadAttributes = engine.PayloadAttributes // only interested in timestamp

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
