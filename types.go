package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/beacon/engine"
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

type PayloadAttributes = engine.PayloadAttributes // only interested in timestamp

type ExecutionPayload = engine.ExecutableData

func (req *JSONRPCRequest) UnmarshalJSON(data []byte) error {
	var msg struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		ID      int    `json:"id"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	var requestParams struct {
		Params []json.RawMessage `json:"params"`
	}
	var params []any
	switch {
	case strings.HasPrefix(msg.Method, fcU):
		if err := json.Unmarshal(data, &requestParams); err != nil {
			return err
		}
		if len(requestParams.Params) < 2 {
			return fmt.Errorf("expected at least 2 params for forkchoiceUpdated")
		}
		params = append(params, requestParams.Params[0])

		var payloadAttributes PayloadAttributes
		if string(requestParams.Params[1]) != "null" {
			if err := json.Unmarshal(requestParams.Params[1], &payloadAttributes); err != nil {
				return err
			}
		}

		params = append(params, &payloadAttributes)
	case strings.HasPrefix(msg.Method, newPayload):
		if err := json.Unmarshal(data, &requestParams); err != nil {
			return err
		}
		if len(requestParams.Params) < 1 {
			return fmt.Errorf("expected at least 1 param for newPayload")
		}
		var executionPayload ExecutionPayload
		if err := json.Unmarshal(requestParams.Params[0], &executionPayload); err != nil {
			return err
		}
		params = append(params, &executionPayload)
	default:
	}
	*req = JSONRPCRequest{
		JSONRPC: msg.JSONRPC,
		Method:  msg.Method,
		Params:  params,
		ID:      msg.ID,
	}
	return nil
}
