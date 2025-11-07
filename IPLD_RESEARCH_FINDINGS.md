# IPLD Multihash Research Findings

## Executive Summary

✅ **IPLD fully supports both double SHA256 and BLAKE3 hash algorithms**

- **Double SHA256** (`dbl-sha2-256`): Code `0x56`, **built-in** to go-multihash
- **BLAKE3**: Code `0x1e`, available via optional import

Both are standardized in the multicodec table and have mature Go implementations.

---

## 1. Multihash Specification Support

### Official Multicodec Table

Reference: [refs/multicodec/table.csv](refs/multicodec/table.csv)

#### BLAKE3
- **Line 21**: `blake3, multihash, 0x1e, draft`
- **Description**: "BLAKE3 has a default 32 byte output length. The maximum length is (2^64)-1 bytes."
- **Status**: Draft (but widely used in practice)
- **Link**: https://github.com/multiformats/multicodec/blob/master/table.csv#L21

#### Double SHA256
- **Line 42**: `dbl-sha2-256, multihash, 0x56, draft`
- **Status**: Draft (Bitcoin standard for txids and block hashes)
- **Link**: https://github.com/multiformats/multicodec/blob/master/table.csv#L42

### Comparison with Single SHA256
- **Line 9**: `sha2-256, multihash, 0x12, permanent`
- Standard single SHA256 is code `0x12` with "permanent" status

---

## 2. Go Implementation Analysis

### go-multihash Library

Repository: https://github.com/multiformats/go-multihash

#### Constants Defined

File: [refs/go-multihash/core/magic.go#L12-L35](refs/go-multihash/core/magic.go#L12-L35)

```go
const (
    IDENTITY      = 0x00
    SHA1          = 0x11
    SHA2_256      = 0x12
    // ...
    BLAKE3        = 0x1E    // ✅ Defined
    // ...
    DBL_SHA2_256  = 0x56    // ✅ Defined
)
```

Also in main package: [refs/go-multihash/multihash.go#L55](refs/go-multihash/multihash.go#L55) and [refs/go-multihash/multihash.go#L67](refs/go-multihash/multihash.go#L67)

#### Double SHA256 Implementation

**Built-in by default** (no import needed)

File: [refs/go-multihash/core/registry.go#L122](refs/go-multihash/core/registry.go#L122)

```go
func init() {
    RegisterVariableSize(IDENTITY, func(_ int) (hash.Hash, bool) { return &identityMultihash{}, true })
    Register(MD5, md5.New)
    Register(SHA1, sha1.New)
    Register(SHA2_224, sha256.New224)
    Register(SHA2_256, sha256.New)
    Register(SHA2_384, sha512.New384)
    Register(SHA2_512, sha512.New)
    Register(SHA2_512_224, sha512.New512_224)
    Register(SHA2_512_256, sha512.New512_256)
    Register(DBL_SHA2_256, func() hash.Hash { return &doubleSha256{sha256.New()} })  // ✅ Built-in
}
```

Implementation: [refs/go-multihash/core/errata.go#L25-L34](refs/go-multihash/core/errata.go#L25-L34)

```go
type doubleSha256 struct {
    hash.Hash
}

func (x doubleSha256) Sum(digest []byte) []byte {
    digest = x.Hash.Sum(digest)      // First SHA256
    h2 := sha256.New()
    h2.Write(digest)
    return h2.Sum(digest[0:0])       // Second SHA256
}
```

This correctly implements Bitcoin's double-SHA256: `SHA256(SHA256(data))`

#### BLAKE3 Implementation

**Requires optional import** (side-effect import pattern)

File: [refs/go-multihash/register/blake3/multihash_blake3.go#L1-L33](refs/go-multihash/register/blake3/multihash_blake3.go#L1-L33)

```go
/*
This package has no purpose except to register the blake3 hash function.

It is meant to be used as a side-effecting import, e.g.

    import (
        _ "github.com/multiformats/go-multihash/register/blake3"
    )
*/
package blake3

import (
    "hash"
    "lukechampine.com/blake3"
    multihash "github.com/multiformats/go-multihash/core"
)

const DefaultSize = 32
const MaxSize = 128

func init() {
    multihash.RegisterVariableSize(multihash.BLAKE3, func(size int) (hash.Hash, bool) {
        if size == -1 {
            size = DefaultSize
        } else if size > MaxSize || size <= 0 {
            return nil, false
        }
        h := blake3.New(size, nil)
        return h, true
    })
}
```

**Usage:**
```go
import (
    _ "github.com/multiformats/go-multihash/register/blake3"
)
```

Uses third-party implementation: `lukechampine.com/blake3`

---

## 3. Multihash Binary Format

### Format Specification

Multihash format: `<hash-function-code><digest-length><digest-bytes>`

Both code and length are **unsigned varints** (variable-length integers).

#### Encoding Function

File: [refs/go-multihash/multihash.go#L258-L270](refs/go-multihash/multihash.go#L258-L270)

```go
// Encode a hash digest along with the specified function code.
// Note: the length is derived from the length of the digest itself.
func Encode(buf []byte, code uint64) ([]byte, error) {
    newBuf := make([]byte, varint.UvarintSize(code)+varint.UvarintSize(uint64(len(buf)))+len(buf))
    n := varint.PutUvarint(newBuf, code)        // Write hash function code
    n += varint.PutUvarint(newBuf[n:], uint64(len(buf)))  // Write digest length

    copy(newBuf[n:], buf)                       // Write digest bytes
    return newBuf, nil
}
```

#### Decoding Function

File: [refs/go-multihash/multihash.go#L278-L311](refs/go-multihash/multihash.go#L278-L311)

```go
func readMultihashFromBuf(buf []byte) (int, uint64, []byte, error) {
    initBufLength := len(buf)
    if initBufLength < 2 {
        return 0, 0, nil, ErrTooShort
    }

    var err error
    var code, length uint64

    code, buf, err = uvarint(buf)       // Read hash function code
    if err != nil {
        return 0, 0, nil, err
    }

    length, buf, err = uvarint(buf)     // Read digest length
    if err != nil {
        return 0, 0, nil, err
    }

    if length > math.MaxInt32 {
        return 0, 0, nil, errors.New("digest too long, supporting only <= 2^31-1")
    }
    if int(length) > len(buf) {
        return 0, 0, nil, errors.New("length greater than remaining number of bytes in buffer")
    }

    // rlen is the advertised size of the multihash
    rlen := (initBufLength - len(buf)) + int(length)
    return rlen, code, buf[:length], nil
}
```

### Size Overhead Examples

Using **unsigned varint encoding**:

| Hash Type | Code | Code Bytes | Digest Size | Length Bytes | Total Overhead | Example Full Size |
|-----------|------|------------|-------------|--------------|----------------|-------------------|
| SHA2-256 | 0x12 | 1 byte | 32 bytes | 1 byte | **2 bytes** | 34 bytes |
| BLAKE3 | 0x1e | 1 byte | 32 bytes | 1 byte | **2 bytes** | 34 bytes |
| DBL-SHA2-256 | 0x56 | 1 byte | 32 bytes | 1 byte | **2 bytes** | 34 bytes |

**Minimal overhead**: Only 2 bytes for standard 32-byte hashes (6.25% increase)

For codes ≥ 128 (0x80), varint uses 2 bytes instead of 1.

---

## 4. High-Level API Usage

### Creating Multihashes

File: [refs/go-multihash/sum.go#L16-L32](refs/go-multihash/sum.go#L16-L32)

```go
// Sum obtains the cryptographic sum of a given buffer.
func Sum(data []byte, code uint64, length int) (Multihash, error) {
    // Get the algorithm.
    hasher, err := mhreg.GetVariableHasher(code, length)
    if err != nil {
        return nil, err
    }

    // Feed data in.
    if _, err := hasher.Write(data); err != nil {
        return nil, err
    }

    return encodeHash(hasher, code, length)
}
```

**Usage examples:**

```go
import (
    mh "github.com/multiformats/go-multihash"
    _ "github.com/multiformats/go-multihash/register/blake3"  // For BLAKE3
)

// Double SHA256 (built-in)
mhash, err := mh.Sum(data, mh.DBL_SHA2_256, -1)

// BLAKE3 (requires import above)
mhash, err := mh.Sum(data, mh.BLAKE3, -1)

// Regular SHA256 for comparison
mhash, err := mh.Sum(data, mh.SHA2_256, -1)
```

### Wrapping Existing Hashes

If you already have a hash digest:

```go
// Wrap a raw 32-byte hash
digest := []byte{...} // your 32-byte hash

// Create multihash from existing digest
mhash, err := mh.Encode(digest, mh.DBL_SHA2_256)
```

### Decoding Multihashes

File: [refs/go-multihash/multihash.go#L223-L236](refs/go-multihash/multihash.go#L223-L236)

```go
// Decode parses multihash bytes into a DecodedMultihash.
func Decode(buf []byte) (*DecodedMultihash, error) {
    // ...
}

type DecodedMultihash struct {
    Code   uint64    // Hash algorithm code
    Name   string    // Human-readable name
    Length int       // Digest length
    Digest []byte    // Raw digest bytes
}
```

**Usage:**

```go
decoded, err := mh.Decode(mhashBytes)
if err != nil {
    return err
}

fmt.Printf("Algorithm: %s (0x%x)\n", decoded.Name, decoded.Code)
fmt.Printf("Digest length: %d\n", decoded.Length)
fmt.Printf("Digest: %x\n", decoded.Digest)

// Verify it's the expected algorithm
if decoded.Code != mh.DBL_SHA2_256 {
    return errors.New("expected double-sha256")
}
```

### Verification Pattern

```go
// To verify data against a multihash:
func VerifyData(data []byte, expectedMH mh.Multihash) error {
    // Decode to get algorithm
    decoded, err := mh.Decode(expectedMH)
    if err != nil {
        return err
    }

    // Hash the data with the same algorithm
    computedMH, err := mh.Sum(data, decoded.Code, decoded.Length)
    if err != nil {
        return err
    }

    // Compare
    if !bytes.Equal(computedMH, expectedMH) {
        return errors.New("hash mismatch")
    }

    return nil
}
```

---

## 5. CID (Content Identifier) Integration

### CID Structure

File: [refs/go-cid/cid.go#L11-L13](refs/go-cid/cid.go#L11-L13)

CIDv1 format:
```
<multibase-prefix><cid-version><multicodec-content-type><multihash>
```

Example: `bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi`

### Creating CIDs with Custom Multihash

File: [refs/go-cid/cid.go#L141-L156](refs/go-cid/cid.go#L141-L156)

```go
// NewCidV1 returns a new Cid using the given multicodec-packed content type.
func NewCidV1(codecType uint64, mhash mh.Multihash) Cid {
    hashlen := len(mhash)

    var b strings.Builder
    b.Grow(1 + varint.UvarintSize(codecType) + hashlen)

    b.WriteByte(1)  // Version 1

    buf := make([]byte, binary.MaxVarintLen64)
    n := binary.PutUvarint(buf, codecType)
    b.Write(buf[:n])

    b.Write([]byte(mhash))

    return Cid{b.String()}
}
```

### Relevant Codec Constants

File: [refs/go-cid/cid.go#L76-L106](refs/go-cid/cid.go#L76-L106)

```go
const (
    Raw         = 0x55      // Raw binary
    DagProtobuf = 0x70      // IPLD DAG-Protobuf
    DagCBOR     = 0x71      // IPLD DAG-CBOR
    DagJSON     = 0x0129    // IPLD DAG-JSON
    BitcoinBlock = 0xb0     // Bitcoin Block
    BitcoinTx    = 0xb1     // Bitcoin Transaction
)
```

**Usage for Bitcoin data:**

```go
import (
    "github.com/ipfs/go-cid"
    mh "github.com/multiformats/go-multihash"
)

// Create CID for Bitcoin transaction
txBytes := []byte{...}  // raw transaction bytes

// Hash with double SHA256
mhash, _ := mh.Sum(txBytes, mh.DBL_SHA2_256, -1)

// Create CID with BitcoinTx codec
cid := cid.NewCidV1(cid.BitcoinTx, mhash)

fmt.Println(cid.String())  // e.g., "bafyrgqb..."
```

---

## 6. DAG-CBOR Encoding

### Overview

File: [refs/go-ipld-prime/codec/dagcbor/doc.go#L1-L42](refs/go-ipld-prime/codec/dagcbor/doc.go#L1-L42)

DAG-CBOR is a restricted profile of CBOR (Compact Binary Object Representation) designed for IPLD:

**Key restrictions:**
- Only explicit-length maps and lists
- Only tag 42 (for CID links)
- Only 64-bit floats
- Maps should be sorted by key (though this implementation is lenient)
- No indefinite-length encoding

### Encoding API

File: [refs/go-ipld-prime/codec/dagcbor/marshal.go#L21-L46](refs/go-ipld-prime/codec/dagcbor/marshal.go#L21-L46)

```go
type EncodeOptions struct {
    // If true, allow encoding of Link nodes as CBOR tag(42);
    // otherwise, reject them as unencodable.
    AllowLinks bool

    // Control the sorting of map keys
    MapSortMode codec.MapSortMode
}

func (cfg EncodeOptions) Encode(n datamodel.Node, w io.Writer) error {
    // Probe for a builtin fast path
    type detectFastPath interface {
        EncodeDagCbor(io.Writer) error
    }
    if n2, ok := n.(detectFastPath); ok {
        return n2.EncodeDagCbor(w)
    }
    // Generic inspection path
    return Marshal(n, cbor.NewEncoder(w), cfg)
}
```

### Size Overhead vs Custom Binary

CBOR is generally efficient but adds some overhead:

**Example comparison for an index node with 100 entries:**

Custom binary format (current):
- 1 byte mode
- 2 bytes count (varint)
- Per entry: 32 bytes key + 32 bytes value = 64 bytes
- Total: ~6,403 bytes

DAG-CBOR equivalent:
- 1 byte map header (or 3 bytes for >23 entries)
- Per entry:
  - 1 byte byte-string header (or 2-3 for 32 bytes)
  - 32 bytes key
  - 1 byte byte-string header
  - 32 bytes value
- Total: ~6,600-6,700 bytes

**Overhead: ~3-5% for typical index nodes**

### When to Use DAG-CBOR

**Advantages:**
- Standardized format with wide tooling support
- Self-describing structure
- Works seamlessly with IPLD ecosystem
- CID links are first-class
- Schema validation available

**Disadvantages:**
- Slightly larger than optimized binary
- Requires IPLD data model abstractions
- More complex than custom encoding

**Recommendation:** Use DAG-CBOR if:
- You want IPFS/IPLD ecosystem compatibility
- You need schema validation
- You want to leverage existing IPLD tooling
- 3-5% size increase is acceptable

Use custom binary if:
- Maximum density is critical
- You control both writer and reader
- You don't need IPLD interoperability

---

## 7. Recommendations for Your Use Case

### Option Analysis

Given your architecture with blockchain data (SHA256) and index structures (BLAKE3):

#### Option B: Hybrid Approach (RECOMMENDED)

**Use multihash only for index structures (BLAKE3 nodes)**

```go
// Blockchain data stays as raw hashes
type BlockchainHash [32]byte  // Raw SHA256 or double-SHA256

// Index structures use multihash
type IndexNodeHash multihash.Multihash  // BLAKE3 as multihash

// Storage keys
type StorageKey interface {
    Bytes() []byte
}

// For blockchain data
func (h BlockchainHash) Bytes() []byte {
    return h[:]
}

// For index data
func (h IndexNodeHash) Bytes() []byte {
    return multihash.Multihash(h)
}
```

**Advantages:**
- Blockchain data (txids, block hashes) stays in native Bitcoin format
- Compatible with existing Bitcoin tools
- Index structures are self-describing
- Can verify index data independently
- Minimal disruption to existing code

**Storage overhead:**
- Blockchain hashes: 32 bytes (no change)
- Index hashes: 34 bytes (2 byte overhead)
- Overall impact: ~0.5-1% total storage increase

### Implementation Strategy

1. **Keep existing blockchain types unchanged:**
   ```go
   type TxID chainhash.Hash      // Bitcoin standard 32-byte hash
   type BlockHash chainhash.Hash // Bitcoin standard 32-byte hash
   ```

2. **Add multihash wrapper for index structures:**
   ```go
   type IndexHash struct {
       mh.Multihash
   }

   func NewIndexHash(data []byte) (IndexHash, error) {
       h, err := mh.Sum(data, mh.BLAKE3, 32)
       return IndexHash{h}, err
   }

   func (h IndexHash) Verify(data []byte) error {
       decoded, _ := mh.Decode(h.Multihash)
       computed, _ := mh.Sum(data, decoded.Code, decoded.Length)
       if !bytes.Equal(computed, h.Multihash) {
           return errors.New("verification failed")
       }
       return nil
   }
   ```

3. **Storage key strategy:**
   ```go
   // BadgerDB keys remain as raw bytes
   // Just use different prefixes
   const (
       PrefixBlock = 0x01  // Block header: raw 32-byte hash
       PrefixTx    = 0x02  // Transaction: raw 32-byte hash
       PrefixIndex = 0x03  // Index node: 34-byte multihash
       PrefixList  = 0x04  // TxID list: 34-byte multihash
   )
   ```

4. **Optional: Add CID support for advanced features:**
   ```go
   func (h IndexHash) ToCID() cid.Cid {
       // Use raw codec for index nodes
       return cid.NewCidV1(cid.Raw, h.Multihash)
   }
   ```

### Migration Path

1. **Phase 1**: Add multihash support alongside existing hashes
2. **Phase 2**: New index nodes use multihash format
3. **Phase 3**: Keep blockchain data in native format (no migration needed)
4. **Phase 4**: Optional - expose CIDs in API for IPFS gateway compatibility

### Future IPFS Integration (Optional)

If you want to enable IPFS storage later:

```go
// Index nodes could be stored in IPFS
func StoreIndexNode(node *IndexNode) (cid.Cid, error) {
    // Encode node (DAG-CBOR or custom)
    data := node.Encode()

    // Hash with BLAKE3
    mhash, _ := mh.Sum(data, mh.BLAKE3, 32)

    // Create CID
    c := cid.NewCidV1(cid.Raw, mhash)

    // Store in IPFS
    ipfsAPI.Add(c, data)

    return c, nil
}
```

---

## 8. Code Examples for Integration

### Basic Setup

```go
package kvstore

import (
    mh "github.com/multiformats/go-multihash"
    _ "github.com/multiformats/go-multihash/register/blake3"
)

// Hash types
type Hash interface {
    Bytes() []byte
    Hex() string
}

// Native blockchain hash (32 bytes)
type ChainHash [32]byte

func (h ChainHash) Bytes() []byte { return h[:] }
func (h ChainHash) Hex() string { return hex.EncodeToString(h[:]) }

// Index hash with algorithm embedded (34 bytes for BLAKE3)
type IndexHash mh.Multihash

func (h IndexHash) Bytes() []byte { return mh.Multihash(h) }
func (h IndexHash) Hex() string { return hex.EncodeToString(h.Bytes()) }
```

### Creating and Verifying Index Hashes

```go
// Hash index node data
func HashIndexNode(data []byte) (IndexHash, error) {
    h, err := mh.Sum(data, mh.BLAKE3, 32)
    return IndexHash(h), err
}

// Verify data against index hash
func (h IndexHash) Verify(data []byte) error {
    decoded, err := mh.Decode(mh.Multihash(h))
    if err != nil {
        return fmt.Errorf("invalid multihash: %w", err)
    }

    computed, err := mh.Sum(data, decoded.Code, decoded.Length)
    if err != nil {
        return fmt.Errorf("hash computation failed: %w", err)
    }

    if !bytes.Equal(computed, h.Bytes()) {
        return errors.New("hash verification failed")
    }

    return nil
}

// Get algorithm info
func (h IndexHash) Algorithm() (code uint64, name string, err error) {
    decoded, err := mh.Decode(mh.Multihash(h))
    if err != nil {
        return 0, "", err
    }
    return decoded.Code, decoded.Name, nil
}
```

### Storage Operations

```go
// Store with appropriate hash type
func (s *Store) Put(hash Hash, data []byte) error {
    return s.db.Put(hash.Bytes(), data)
}

func (s *Store) Get(hash Hash) ([]byte, error) {
    return s.db.Get(hash.Bytes())
}

// Verify retrieved data
func (s *Store) GetAndVerify(hash IndexHash) ([]byte, error) {
    data, err := s.Get(hash)
    if err != nil {
        return nil, err
    }

    if err := hash.Verify(data); err != nil {
        return nil, fmt.Errorf("verification failed: %w", err)
    }

    return data, nil
}
```

---

## 9. Summary & Next Steps

### Key Findings

✅ **Both hash algorithms are fully supported:**
- Double SHA256: Built-in, code `0x56`
- BLAKE3: Optional import, code `0x1e`

✅ **Minimal overhead:**
- 2 bytes per hash (6.25% for 32-byte hashes)
- Varint encoding keeps it compact

✅ **Mature implementations:**
- Well-tested Go libraries
- Used in production by IPFS
- Active maintenance

✅ **Optional IPFS integration:**
- Can add CID support later
- DAG-CBOR available if needed
- No forced dependencies

### Recommended Next Steps

1. **Decide on Option B (Hybrid)** or discuss alternatives
2. **Define trust/verification requirements** - Who verifies what?
3. **Prototype integration:**
   - Add multihash wrapper types
   - Test storage overhead
   - Benchmark performance
4. **Consider DAG-CBOR** only if IPLD compatibility is important
5. **Plan migration** (or start fresh with new index nodes)

### Open Questions

1. **Use cases**: Which scenarios from section "Use Cases to Consider" apply?
2. **Verification**: Is client-side verification a requirement?
3. **IPFS integration**: Is this a future goal or out of scope?
4. **Performance targets**: Any specific constraints on hash computation or storage?

---

## References

### Specifications
- Multihash spec: https://multiformats.io/multihash/
- Multicodec table: https://github.com/multiformats/multicodec/blob/master/table.csv
- CID spec: https://github.com/ipld/cid
- DAG-CBOR spec: https://ipld.io/specs/codecs/dag-cbor/

### Go Libraries
- go-multihash: https://github.com/multiformats/go-multihash
- go-cid: https://github.com/ipfs/go-cid
- go-ipld-prime: https://github.com/ipld/go-ipld-prime
- BLAKE3 (lukechampine): https://github.com/lukechampine/blake3

### Local References
- Cloned repos in `refs/` directory
- See file paths above for specific code locations
