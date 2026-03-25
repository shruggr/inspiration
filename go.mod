module github.com/shruggr/inspiration

go 1.26.0

require (
	github.com/bsv-blockchain/go-sdk v1.2.19
	github.com/bsv-blockchain/teranode v0.14.0
	github.com/dgraph-io/badger/v4 v4.8.0
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/mattn/go-sqlite3 v1.14.34
	github.com/multiformats/go-multihash v0.2.3
	github.com/segmentio/kafka-go v0.4.50
	google.golang.org/protobuf v1.36.11
	lukechampine.com/blake3 v1.4.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgraph-io/ristretto/v2 v2.2.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/flatbuffers v25.2.10+incompatible // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-varint v0.1.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.42.0 // indirect
	go.opentelemetry.io/otel/metric v1.42.0 // indirect
	go.opentelemetry.io/otel/trace v1.42.0 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
)

replace github.com/bsv-blockchain/go-sdk => ../1sat/go-sdk

replace github.com/bsv-blockchain/teranode => ../1sat/teranode
