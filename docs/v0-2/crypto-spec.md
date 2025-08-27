# LAP Cryptographic Specification v0.2

This document defines the canonical cryptographic methods used in the LAP protocol. These specifications are prescriptive and MUST be followed by all implementations.

## Overview

LAP uses minimal cryptography to achieve its verification goals:

1. **SHA-256** for content hashing and message digests
2. **secp256k1** elliptic curve for publisher key generation
3. **Schnorr signatures** (BIP-340) for Namespace Attestation signing
4. **X-only public keys** (32 bytes) for compact representation

## Hash Functions

### SHA-256

**Purpose**: Content integrity verification and signature message digests

**Specification**:

-   Algorithm: SHA-256 as defined in FIPS 180-4
-   Output: 32-byte (256-bit) digest
-   Encoding: Lowercase hexadecimal for text representation
-   Content hash format: `sha256:<64-hex-chars>`

**Usage**:

-   Resource Attestation content integrity: `sha256:` prefix + hex digest
-   Namespace Attestation signature message digest: Raw 32-byte digest (no prefix)

## Elliptic Curve Cryptography

### secp256k1 Curve

**Purpose**: Key generation and digital signatures

**Specification**:

-   Curve: secp256k1 as defined in SEC 2
-   Field size: 256 bits
-   Private keys: 32 bytes (256 bits)
-   Public keys: X-only format (32 bytes, BIP-340)

### Key Formats

**Publisher Private Keys**:

-   Size: 32 bytes (256 bits)
-   Encoding: 64 lowercase hexadecimal characters
-   Generation: Cryptographically secure random number generation
-   Scope: One per publisher (namespace-level)

**Publisher Public Keys**:

-   Format: X-only (BIP-340 specification)
-   Size: 32 bytes (X coordinate only)
-   Encoding: 64 lowercase hexadecimal characters
-   Derivation: X coordinate of secp256k1 public key point

## Digital Signatures

### Schnorr Signatures (BIP-340)

**Purpose**: Namespace Attestation signing

**Specification**:

-   Algorithm: Schnorr signatures as defined in BIP-340
-   Curve: secp256k1
-   Signature size: 64 bytes
-   Encoding: 128 lowercase hexadecimal characters
-   Message: SHA-256 digest of canonical payload JSON

### Signature Process

**Signing**:

1. Serialize payload to canonical JSON format
2. Compute SHA-256 digest of canonical JSON bytes
3. Sign 32-byte digest using Schnorr algorithm (BIP-340)
4. Encode 64-byte signature as 128 hex characters

**Verification**:

1. Parse hex-encoded signature (128 chars → 64 bytes)
2. Parse hex-encoded X-only public key (64 chars → 32 bytes)
3. Serialize payload to canonical JSON format
4. Compute SHA-256 digest of canonical JSON bytes
5. Verify signature against digest using BIP-340 verification

## Canonical JSON Serialization

**Purpose**: Deterministic message creation for signature verification

**Requirements**:

-   No whitespace between elements
-   Keys sorted lexicographically
-   No trailing commas
-   UTF-8 encoding
-   Consistent field ordering

**Example** (Namespace Attestation payload):

```json
{
    "exp": 1754909100,
    "namespace": "https://example.com/people/alice/"
}
```

## Security Considerations

### Key Generation

-   Publisher private keys MUST be generated using cryptographically secure random number generation
-   Publisher private keys MUST be kept secret and never transmitted
-   Publisher public keys are safe to distribute and embed in fragments

### Signature Security

-   Each signature MUST use a unique nonce (handled by BIP-340)
-   Message digests MUST be computed over canonical JSON representation
-   Signature verification MUST validate against the exact canonical payload

### Hash Security

-   SHA-256 provides 128-bit security level (sufficient for LAP use cases)
-   Resource Attestation content hashes MUST include the `sha256:` prefix for field values
-   Namespace Attestation message digests for signatures use raw 32-byte format (no prefix)

## Implementation Requirements

### MUST Requirements

-   Implementations MUST use secp256k1 curve
-   Implementations MUST use Schnorr signatures per BIP-340
-   Implementations MUST use SHA-256 for all hashing
-   Implementations MUST use X-only public key format
-   Implementations MUST serialize payloads canonically for signing
-   Implementations MUST encode keys and signatures as lowercase hex

### SHOULD Requirements

-   Implementations SHOULD validate key and signature formats before processing
-   Implementations SHOULD use constant-time operations for cryptographic functions
-   Implementations SHOULD clear sensitive data from memory after use

### Example Libraries

-   **Go**: `github.com/btcsuite/btcd/btcec/v2` with `schnorr` package
-   **JavaScript**: `@noble/secp256k1` with Schnorr support
-   **Python**: `python-bitcoinlib` or similar secp256k1 libraries
-   **Rust**: `secp256k1` crate with schnorr feature

## Test Vectors

### Key Pair Example

```
Private Key: a1b2c3d4e5f6071829384756647382910abcdef1234567890fedcba0987654321
Public Key:  f1a2d3c4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00
```

### Signature Example

```
Message: {"exp":1754909100,"namespace":"https://example.com/people/alice/"}
SHA-256:  d4e5f6071829384756647382910abcdef1234567890fedcba0987654321a1b2c3
Signature: 4e0f1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7
```

_Note: These are example values for illustration. Real implementations should generate secure random keys._
