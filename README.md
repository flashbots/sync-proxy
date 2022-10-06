#

[![Goreport status](https://goreportcard.com/badge/github.com/flashbots/sync-proxy)](https://goreportcard.com/report/github.com/flashbots/sync-proxy)
[![Test status](https://github.com/flashbots/sync-proxy/workflows/Checks/badge.svg)](https://github.com/flashbots/sync-proxy/actions?query=workflow%3A%22Checks%22)

Flashbots proxy to allow redundant execution client (EL) state sync post merge.

* Runs a proxy server that proxies requests from a beacon node (BN) to multiple other execution clients
* Can drive EL sync from multiple BNs for redundancy

## Getting Started

Run a BN pointing to the proxy (default is `localhost:25590`). To run with multiple ELs running, run the proxy specifying the EL endpoints (make sure to point to the authenticated port). 

```bash
git clone https://github.com/flashbots/sync-proxy.git
cd sync-proxy
make build

# Show the help
./sync-proxy -help
```

To run with an EL endpoint:

```
./sync-proxy -builders="localhost:8551,localhost:8552"
```

## Caveats

The sync proxy attempts to sync to best beacon node based on the slot number in a custom rpc call sent by the open source [flashbots prysm client](https://github.com/flashbots/prysm). If not using the flashbots prysm client the sync proxy will sync to the beacon node that sends the sync proxy a request first. It will only switch if the first beacon node stops sending requests. 

The sync proxy attempts to identify the best beacon node based on the originating host of the request. If you are using the same host for multiple beacon nodes to sync the EL, the sync proxy won't be able to distinguish between the beacon nodes and will proxy all requests from the same host to the ELs.
