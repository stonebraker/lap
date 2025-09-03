module github.com/stonebraker/lap/apps/demo-utils

go 1.22

toolchain go1.24.6

require (
	github.com/btcsuite/btcd/btcec/v2 v2.3.5
	github.com/stonebraker/lap/sdks/go v0.0.0-20250831034313-db2334ae7923
)

require (
	github.com/btcsuite/btcd/chaincfg/chainhash v1.0.1 // indirect
	github.com/decred/dcrd/crypto/blake256 v1.0.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.0.1 // indirect
)

replace github.com/stonebraker/lap/sdks/go => ../../sdks/go
