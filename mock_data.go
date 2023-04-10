package main

var (
	mockNewPayloadRequest = `{
		"jsonrpc": "2.0",
		"method": "engine_newPayloadV1",
		"params": [
			{
			  "parentHash": "0x3b8fb240d288781d4aac94d3fd16809ee413bc99294a085798a589dae51ddd4a",
			  "feeRecipient": "0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b",
			  "stateRoot": "0xca3149fa9e37db08d1cd49c9061db1002ef1cd58db2210f2115c8c989b2bdf45",
			  "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
			  "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
			  "prevRandao": "0x0000000000000000000000000000000000000000000000000000000000000000",
			  "blockNumber": "0x1",
			  "gasLimit": "0x1c9c380",
			  "gasUsed": "0x0",
			  "timestamp": "0x5",
			  "extraData": "0x",
			  "baseFeePerGas": "0x7",
			  "blockHash": "0x3559e851470f6e7bbed1db474980683e8c315bfce99b2a6ef47c057c04de7858",
			  "transactions": []
			}
		],
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
	mockForkchoiceRequestWithPayloadAttributesV1 = `{
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
			"prevRandao": "0x0000000000000000000000000000000000000000000000000000000000000000",
			"suggestedFeeRecipient": "0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b"
		  }
		],
		"id": 67
	}`
	mockForkchoiceRequestWithPayloadAttributesV2 = `{
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
			"prevRandao": "0x0000000000000000000000000000000000000000000000000000000000000000",
			"suggestedFeeRecipient": "0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b",
			"withdrawals": [
				{
				  "index": "0x237dde",
				  "validatorIndex": "0x38b37",
				  "address": "0x8f0844fd51e31ff6bf5babe21dccf7328e19fd9f",
				  "amount": "0x1a6d92"
				}
			]
		  }
		],
		"id": 67
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
		  null
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
	mockEthChainIDRequest = `{"jsonrpc":"2.0","method":"eth_chainId","id":1}`
)
