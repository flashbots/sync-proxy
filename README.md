# go-template

[![Goreport status](https://goreportcard.com/badge/github.com/flashbots/sync-proxy)](https://goreportcard.com/report/github.com/flashbots/sync-proxy)
[![Test status](https://github.com/flashbots/sync-proxy/workflows/Checks/badge.svg)](https://github.com/flashbots/sync-proxy/actions?query=workflow%3A%22Checks%22)

Flashbots internal proxy to allow redundant execution client (EL) state sync post merge.

* Runs a proxy server that proxies requests from a beacon node (BN) to multiple other execution clients
* Proxies requests from BN to other proxies to achieve redundancy with requests from multiple BNs

More Information:

* https://www.notion.so/flashbots/EL-BN-state-sync-e0dfc2b8d615474a815da55aa5c4de17

## Getting Started

Run a BN pointing to the proxy (default is `localhost:25590`). To run with multiple ELs running, run the proxy specifying the EL endpoints (make sure to point to the authenticated port). 

```bash
git clone https://github.com/flashbots/sync-proxy.git
cd sync-proxy
make build

# Show the help
./sync-proxy -help
```

You can also run multiple BNs with redundant BN requests by running multiple proxies that proxy requests between each other.

To run with EL endpoint and / or multiple proxies:

```
./sync-proxy -builders="localhost:8551,localhost:8552" -proxies="localhost:25591"
```
