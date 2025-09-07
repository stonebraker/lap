# Linked Attestation Protocol (LAP)

> ⚠️ **EXPERIMENTAL** - Not production ready, seeking feedback

## What is LAP?

LAP lets you verify that content actually came from its claimed publisher, even when it's shared across different websites. Think of it as a live, cryptographic link between content and creator that travels with the content.

**Example:** Alice posts something on her site → Westley embeds it on his site → You can verify it really came from Alice.

## Try the Demo! 🚀

See LAP in action in under 2 minutes:

### 1. Start the servers (3 terminals)

```bash
# Terminal 1: Alice's content
go run ./apps/server/cmd/publisherapi

# Terminal 2: Verification service
go run ./apps/verifier-service/cmd/verifier-service

# Terminal 3: Demo page
go run ./apps/client-server/cmd/client-server
```

### 2. Visit the demo

-   **Alice's posts**: http://localhost:8080/people/alice/frc/posts/
-   **LAP Play page**: http://localhost:8081

### 3. What you'll see

-   Content with cryptographic proof of authenticity
-   Real-time verification across different sites
-   How publishers can associate/dissociate from content

## Requirements

-   Go 1.22+

## Building

To build all binaries for the project:

```bash
go run build.go
```

This will create all binaries in the `bin/` directory:

-   `lapctl` - CLI tools
-   `publisherapi` - Alice's content server
-   `client-server` - Demo page server
-   `verifier` - CLI verifier
-   `verifier-service` - HTTP verifier service

## Learn More

-   **[Complete Specification](docs/v0.2/overview.md)** - Full protocol details
-   **[Repository Structure](#repository-structure)** - Code organization
-   **[Advanced Usage](#advanced-usage)** - CLI tools and SDK

## Repository Structure

```
apps/
  server/           # Alice's content server
  client-server/    # Demo page server
  verifier-service/ # Verification service
  tools-cli/        # CLI tools (lapctl)
sdks/go/           # Go SDK
demo-keys/         # Demo keys (Alice & Westley)
```

## Advanced Usage

<details>
<summary>CLI Tools & SDK</summary>

### lapctl CLI

```bash
# Build all binaries (recommended)
go run build.go

# Or build just lapctl
go build -o bin/lapctl ./apps/tools-cli/cmd/lapctl

# Reset demo artifacts
./bin/lapctl reset-artifacts

# Generate new keys
./bin/lapctl keygen -name myname

# See all commands
./bin/lapctl help
```

### Go SDK

```go
import "github.com/stonebraker/lap/sdks/go/pkg/lap"
```

### Detailed CLI Commands

Build all binaries (recommended):

```bash
go run build.go
```

Or build individual tools:

```bash
go build -o bin/lapctl ./apps/tools-cli/cmd/lapctl
```

Generate a secp256k1 keypair:

```bash
bin/lapctl keygen -name alice
```

Reset all LAP artifacts for Alice (complete refresh):

```bash
bin/lapctl reset-artifacts
```

-   **Purpose**: Complete reset of all LAP artifacts - creates new Namespace Attestation and updates all posts
-   **Output**: Creates new `_la_namespace.json`, `_la_resource.json` and `index.htmx` for posts 1-3, updates host file
-   **Optional**: `-base` (default: `http://localhost:8080`), `-root` (default: `apps/server/static/publisherapi/people/alice`), `-keys-dir` (default: `demo-keys`)

Create a Resource Attestation (RA) for an HTML file:

```bash
bin/lapctl ra-create \
  -in apps/server/static/publisherapi/people/alice/frc/posts/1/content.htmx \
  -url http://localhost:8080/people/alice/frc/posts/1 \
  -publisher-claim ac20898edf97b5a24c59749ec26ea7bc95cc1d2859ef6a194ceb7eeb2c709677 \
  -namespace-attestation-url http://localhost:8080/people/alice/_la_namespace.json
```

-   **Purpose**: Creates an unsigned Resource Attestation JSON that links content to its publisher
-   **Output**: Writes RA JSON to `<dir>/_la_resource.json` by default (override with `-out`)
-   **Content**: Includes SHA-256 hash of the HTML file, publisher's public key, and namespace attestation URL
-   **Required**: `-publisher-claim` (64-char hex secp256k1 X-only public key) and `-namespace-attestation-url`
-   **Optional**: `-base` for resolving relative URLs, `-out` for custom output path

Create a Namespace Attestation (NA) for a namespace:

```bash
bin/lapctl na-create \
  -namespace https://localhost:8080/people/alice/ \
  -exp 1754909400 \
  -privkey b390add8da13892d0a4ca22ef5aa5f8efd4c0331bd3c2b3ce28eade7beac0c5b \
  -out apps/server/static/publisherapi/people/alice
```

-   Writes NA JSON to `<dir>/_la_namespace.json` by default (override with `-out`)
-   Required: `-namespace` URL
-   Optional: `-exp` expiration timestamp (default: 1 year from now), `-privkey` for specific key, `-rotate` to force new keypair

Create a fragment (index.htmx) from `index.html`:

```bash
bin/lapctl fragment-create \
  -in apps/server/static/publisherapi/people/alice/frc/posts/1/content.htmx \
  -url http://localhost:8080/people/alice/frc/posts/1 \
  -publisher-claim ac20898edf97b5a24c59749ec26ea7bc95cc1d2859ef6a194ceb7eeb2c709677 \
  -resource-attestation-url http://localhost:8080/people/alice/frc/posts/1/_la_resource.json \
  -namespace-attestation-url http://localhost:8080/people/alice/_la_namespace.json
```

Show help:

```bash
bin/lapctl help
```

### Technical Details

This repository contains:

-   **publisherapi**: static file server for demonstrating LAP protocol with live examples
-   **client-server**: interactive demo server showing LAP content verification and integration
-   **verifier-cli**: CLI tool for LAP resource attestation verification with full cryptographic validation
-   **verifier-service**: HTTP service for real-time LAP fragment verification
-   **tools-cli (lapctl)**: primary CLI for LAP operations including key generation and attestation creation
-   **Go SDK**: comprehensive Go library for LAP operations (canonicalization, crypto, verification, wire format)

There are two Go modules tied together by `go.work` at the repo root:

-   Root module (servers, CLI): `module lap`
-   SDK module (libraries): `sdks/go` (module `github.com/stonebraker/lap/sdks/go`)

**LAP Protocol Status**: This is a complete implementation of the Linked Attestations Protocol (LAP) v0.2 with working cryptographic verification for **Resource Attestations** and **Namespace Attestations**, Go SDK support, and comprehensive test coverage. JavaScript library support is currently being refactored.

**The protocol is not considered production ready.** The project is currently seeking feedback on all aspects, including _any compelling evidence_ that it cannot perform the function it is meant to perform, the organization and ease of use of the docs and reference implementation, documentation improvements, etc.

### Cryptography

The implementation uses SHA-256 hashing for Resource Attestation content integrity and secp256k1 + Schnorr signatures for Namespace Attestation publisher verification, with comprehensive validation including hash validation, signature verification, and drift detection.

</details>

## License

Copyright 2025 Jason Stonebraker

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## Acknowledgments

This project uses the following open source libraries:

-   btcd/btcec for Go cryptographic operations

See NOTICE file for complete attribution details.
