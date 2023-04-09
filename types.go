package main

import (
	"encoding/json"
	"fmt"
	"strings"

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
		JSONRPC string            `json:"jsonrpc"`
		Method  string            `json:"method"`
		Params  []json.RawMessage `json:"params,omitempty"`
		ID      int               `json:"id"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	var params []any
	switch {
	case msg.Method == builderAttributes:
		if len(msg.Params) != 1 {
			return fmt.Errorf("expected 1 param for builderAttributes")
		}
		var payloadParams BuilderPayloadAttributes
		if err := json.Unmarshal(msg.Params[0], &payloadParams); err != nil {
			return err
		}

		params = append(params, &payloadParams)
	case strings.HasPrefix(msg.Method, fcU):
		if len(msg.Params) != 2 {
			return fmt.Errorf("expected 2 params for forkchoiceUpdated")
		}
		params = append(params, msg.Params[0])

		var payloadAttributes PayloadAttributes
		if err := json.Unmarshal(msg.Params[1], &payloadAttributes); string(msg.Params[1]) != "null" && err != nil {
			return err
		}

		params = append(params, &payloadAttributes)
	case strings.HasPrefix(msg.Method, newPayload):
		if len(msg.Params) != 1 {
			return fmt.Errorf("expected 1 param for newPayload")
		}
		var executionPayload ExecutionPayload
		if err := json.Unmarshal(msg.Params[0], &executionPayload); err != nil {
			return err
		}
		params = append(params, &executionPayload)
	default:
		if msg.Params != nil {
			for _, p := range msg.Params {
				params = append(params, p)
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
