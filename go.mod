module github.com/flashbots/sync-proxy

go 1.18

require (
	github.com/StackExchange/wmi v0.0.0-20210224194228-fe8f1750fd46 // indirect
	github.com/VictoriaMetrics/fastcache v1.6.0 // indirect
	github.com/btcsuite/btcd/btcec/v2 v2.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/deckarep/golang-set v1.8.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/klauspost/cpuid/v2 v2.0.14 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.4.1 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/tsdb v0.10.0 // indirect
	github.com/prysmaticlabs/fastssz v0.0.0-20220628121656-93dfe28febab // indirect
	github.com/prysmaticlabs/gohashtree v0.0.2-alpha // indirect
	github.com/shirou/gopsutil v3.21.4-0.20210419000835-c7a38de76ee5+incompatible // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220614013038-64ee5596c38a // indirect
	github.com/thomaso-mirodin/intmath v0.0.0-20160323211736-5dc6d854e46e // indirect
	github.com/tklauser/go-sysconf v0.3.5 // indirect
	github.com/tklauser/numcpus v0.2.2 // indirect
	golang.org/x/crypto v0.0.0-20220622213112-05595931fe9d // indirect
	golang.org/x/sys v0.0.0-20220702020025-31831981b65f // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/natefinch/npipe.v2 v2.0.0-20160621034901-c1b8fa8bdcce // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require (
	github.com/ethereum/go-ethereum v1.10.25
	github.com/gorilla/mux v1.8.0
	github.com/prysmaticlabs/prysm/v3 v3.1.1
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.2
)

replace github.com/prysmaticlabs/prysm/v3 => github.com/flashbots/prysm/v3 v3.1.2-0.20220914102155-c186a065c9bf
