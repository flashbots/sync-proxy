package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
)

type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  PayloadParams `json:"params,omitempty"`
	ID      int           `json:"id"`
}

type PayloadParams struct {
	SlotStage SlotStage `json:"-"`

	// variatic params depended on the type of request
	HeadBlockHash             *common.Hash `json:"headBlockHash,omitempty"`
	FinilizedBlockHash        *common.Hash `json:"finilizedBlockHash,omitempty"`
	SafeBlockHash             *common.Hash `json:"safeBlockHash,omitempty"`
	BlockHash                 *common.Hash `json:"blockHash,omitempty"`
	ParentBlockHash           *common.Hash `json:"parentBlockHash,omitempty"`
	BlockNumber               *uint64      `json:"blockNumber,omitempty"`
	ForkchoiceUpdateTimestamp *uint64      `json:"forkchoiceUpdateTimestamp,omitempty"`
	NewPayloadTimestamp       *uint64      `json:"newPayloadTimestamp,omitempty"`
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
	var params PayloadParams
	switch {
	case strings.HasPrefix(msg.Method, fcU):
		params.SlotStage = FCUOpen

		if err := json.Unmarshal(data, &requestParams); err != nil {
			return err
		}
		if len(requestParams.Params) < 2 {
			return fmt.Errorf("expected at least 2 params for forkchoiceUpdated")
		}

		var state engine.ForkchoiceStateV1
		if err := json.Unmarshal(requestParams.Params[0], &state); err != nil {
			return fmt.Errorf("failed to unmarshal forkchoiceStateV1 %w", err)
		}

		params.HeadBlockHash = &state.HeadBlockHash
		params.FinilizedBlockHash = &state.FinalizedBlockHash
		params.SafeBlockHash = &state.SafeBlockHash

		if string(requestParams.Params[1]) != "null" {
			var attrs engine.PayloadAttributes
			if err := json.Unmarshal(requestParams.Params[1], &attrs); err != nil {
				return fmt.Errorf("failed to unmarshal payloadAttributes %w", err)
			}

			params.ForkchoiceUpdateTimestamp = &attrs.Timestamp
			params.SlotStage = FCUClose
		}

	case strings.HasPrefix(msg.Method, newPayload):
		params.SlotStage = Payload

		if err := json.Unmarshal(data, &requestParams); err != nil {
			return err
		}

		if len(requestParams.Params) < 1 {
			return fmt.Errorf("expected at least 1 param for newPayload")
		}

		var data ExecutionPayload
		if err := json.Unmarshal(requestParams.Params[0], &data); err != nil {
			return fmt.Errorf("failed to unmarshal executionPayload %w", err)
		}

		params.BlockHash = &data.BlockHash
		params.NewPayloadTimestamp = &data.Timestamp
		params.ParentBlockHash = &data.ParentHash
		params.BlockNumber = &data.Number
	default:
		params.SlotStage = Unknown
	}
	*req = JSONRPCRequest{
		JSONRPC: msg.JSONRPC,
		Method:  msg.Method,
		Params:  params,
		ID:      msg.ID,
	}
	return nil
}
