package main

var (
	mockNewPayloadRequest = `{
		"jsonrpc": "2.0",
		"method": "engine_newPayloadV1",
		"params": ["0x01"],
		"id": 67
	}`
	mockNewPayloadResponseValid = `{
		"jsonrpc": "2.0",
		"id": 67,
		"result": {
		  "status": "VALID",
		  "latestValidHash": "0x3559e851470f6e7bbed1db474980683e8c315bfce99b2a6ef47c057c04de7858",
		  "validationError": ""
		}
	}`
	mockNewPayloadResponseSyncing = `{
		"jsonrpc": "2.0",
		"id": 67,
		"result": {
		  "status": "SYNCING",
		  "latestValidHash": "0x3559e851470f6e7bbed1db474980683e8c315bfce99b2a6ef47c057c04de7858",
		  "validationError": ""
		}
	}`
	mockForkchoiceRequest = `{
		"jsonrpc": "2.0",
		"method": "engine_forkchoiceUpdatedV1",
		"params": [
		  {
			"headBlockHash": "0x3b8fb240d288781d4aac94d3fd16809ee413bc99294a085798a589dae51ddd4a",
			"safeBlockHash": "0x3b8fb240d288781d4aac94d3fd16809ee413bc99294a085798a589dae51ddd4a",
			"finalizedBlockHash": "0x0000000000000000000000000000000000000000000000000000000000000000"
		  },
		  {
			"timestamp": "0x5",
			"random": "0x0000000000000000000000000000000000000000000000000000000000000000",
			"suggestedFeeRecipient": "0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b"
		  }
		],
		"id": 67
	}`
	mockForkchoiceResponse = `{
		"jsonrpc": "2.0",
		"id": 67,
		"result": {
		  "payloadStatus": {
			"status": "VALID",
			"latestValidHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
			"validationError": ""
		  },
		  "payloadId": null
		}
	}`
	mockTransitionRequest = `{
		"jsonrpc": "2.0",
		"method": "engine_exchangeTransitionConfigurationV1",
		"params": ["0x12309ce54000", "0x0000000000000000000000000000000000000000000000000000000000000000", "0x0"],
		"id": 1
	}`
	mockTransitionResponse = `{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"terminalTotalDifficulty": "0x12309ce54000",
			"terminalBlockHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
			"terminalBlockNumber": "0x0"
		}
	}`
)
