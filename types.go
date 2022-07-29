package main

import "github.com/flashbots/go-boost-utils/types"

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
