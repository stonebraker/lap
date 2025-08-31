# Linked Attestation Protocol (LAP)

> ⚠️ **PROTOCOL IN DEVELOPMENT** ⚠️  
> LAP is currently in active development. The protocol specification is **experimental** and **subject to changes**.
>
> -   **v0.1**: Legacy implementation (archived in `docs/v0.1/`)
> -   **v0.2**: **Complete specification** - open to use and feedback, **NOT PRODUCTION READY**
>
> **Do not use in production systems.** This is experimental software.

## Documentation

-   **[v0.2 Specification](docs/v0.2/overview.md)** - Complete specification, open to use and feedback
-   **[v0.1 Legacy Documentation](docs/v0.1/)** - Archived, no longer maintained

## Status

![Status](https://img.shields.io/badge/status-experimental-red)
![Version](https://img.shields.io/badge/version-v0.2-orange)
![Stability](https://img.shields.io/badge/stability-unstable-red)

**Current Phase**: Active development and community feedback  
**Specification Status**: v0.2 complete and open to use  
**Production Ready**: Not yet - experimental software

# Linked Attestations Protocol (LAP)

## Overview

The LAP (Linked Attestation Protocol) provides reasonable proof of content integrity and publisher authenticity for peer-to-peer, distributed micro-content. It **_associates_** distributed Content with its original Publisher, regardless of where the Content appears on the web. Just as importantly, it allows a Publisher to **_dissociate_** from Content should they choose to do so. Dissociation leaves **_no durable evidence_** of a prior Publisher <-> Content association. Verification ensures Content Presence, Content Integrity, and Publisher Association.

The Linked Attestation Protocol uses basic HTTP + a bit of cryptography, and requires no intermediary, central coordinator, blockchain, or token. It is designed to be very easy for developers and non-developers to implement utilizing lightweight libraries and SDKs.

### Basic use case

1. Alice posts a self-contained bit of micro-content (like a social media post) to her own website and permits cross origin access to Jane.
2. Jane fetches the micro-content (an html fragment, not a full html doc), and embeds it in a page on her own website.
3. Eduardo visits Jane's site and is able to verify that the post he sees there actually came from Alice.

**What it does**  
This protocol concerns itself with linking a publisher to distributed micro-content, and allowing for the verification of that link. Conversely, it allows for the unlinking of a publisher from distributed micro-content, should the publisher choose.

**What it does not do**  
It does not concern itself with the fetching and embedding of content which can be easily implemented thanks to libraries like HTMX or Datastar and may one day be part of the HTTP spec.

📖 **[Read the Complete Protocol Specification →](docs/v0.2/overview.md)**

## Quick Start

This repository contains:

-   **publisherapi**: static file server for demonstrating LAP protocol with live examples
-   **verifier-cli**: CLI tool for LAP resource attestation verification with full cryptographic validation
-   **tools-cli (lapctl)**: primary CLI for LAP operations including key generation and attestation creation

There are two Go modules tied together by `go.work` at the repo root:

-   Root module (servers, CLI): `module lap`
-   SDK module (libraries): `sdks/go` (module `github.com/stonebraker/lap/sdks/go`)

**LAP Protocol Status**: This is a complete implementation of the Linked Attestations Protocol (LAP) v0.2 with working cryptographic verification for **Resource Attestations** and **Namespace Attestations**, Go SDK support, and comprehensive test coverage. JavaScript library support is currently being refactored.

**The protocol is not considered production ready.** The project is currently seeking feedback on all aspects, including _any compelling evidence_ that it cannot perform the function it is meant to perform, the organization and ease of use of the docs and reference implementation, documentation improvements, etc.

### Requirements

-   Go 1.22+

### Build / Run

Run from the repository root so default static directories resolve correctly. You can override directories with `-dir` flags.

Create a `bin/` directory for built binaries:

```bash
mkdir -p bin
```

Build binaries:

```bash
go build -o bin/publisherapi ./apps/server/cmd/publisherapi
go build -o bin/verifier ./apps/verifier-cli/cmd/verifier
go build -o bin/lapctl ./apps/tools-cli/cmd/lapctl
```

Build all (workspace):

```bash
go build ./apps/...
```

Run the demo server (from repo root):

```bash
go run ./apps/server/cmd/publisherapi   # serves ./apps/server/static/publisherapi on :8081 by default
```

Optional: run with explicit flags

```bash
go run ./apps/server/cmd/publisherapi -addr :8081 -dir apps/server/static/publisherapi
```

Run the verifier CLI:

```bash
go run ./apps/verifier-cli/cmd/verifier -h
```

### Repository layout

```
apps/
  keygen-cli/               # key generation CLI tool
    cmd/
      keygen/
  server/
    cmd/
      publisherapi/         # demo server
    internal/
      httpx/                # HTTP utilities
    static/
      publisherapi/         # demo content
        people/
          alice/            # demo publisher
            posts/          # example LAP content
              1/, 2/, 3/    # posts with attestations
  tools-cli/
    cmd/
      lapctl/               # primary LAP CLI tool
  verifier-cli/
    cmd/
      verifier/             # verification CLI
docs/
  v0.1/                     # legacy documentation
  v0.2/                     # current specification
sdks/
  go/                       # Go SDK (separate module)
    pkg/lap/
      canonical/
      crypto/
      testutil/
      verify/
      wire/
keys/                       # key storage
```

### Cryptography

The implementation uses SHA-256 hashing for Resource Attestation content integrity and secp256k1 + Schnorr signatures for Namespace Attestation publisher verification, with comprehensive validation including hash validation, signature verification, and drift detection.

### Tools CLI: lapctl

Build the tools CLI:

```bash
go build -o bin/lapctl ./apps/tools-cli/cmd/lapctl
```

Generate a secp256k1 keypair:

```bash
bin/lapctl keygen -name alice
```

Create a fragment (index.htmx) from `index.html`:

```bash
bin/lapctl fragment-create \
  -in apps/server/static/publisherapi/people/alice/posts/1/index.html \
  -url /people/alice/posts/1 \
  -base http://localhost:8081 \
  -publisher-claim f1a2d3c4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00 \
  -resource-attestation-url https://localhost:8081/people/alice/posts/1/_la_resource.json \
  -namespace-attestation-url https://localhost:8081/people/alice/_la_namespace.json
```

Update multiple posts (1-3) with fragments and attestations:

```bash
bin/lapctl update-posts \
  -base http://localhost:8081 \
  -dir apps/server/static/publisherapi/people/alice
```

-   **Purpose**: Batch process posts 1-3 to generate Resource Attestations, fragments, and update host file
-   **Output**: Creates `_la_resource.json` and `index.htmx` for each post, updates `posts/index.html` host file
-   **Required**: `-base` URL and `-dir` root directory path
-   **Prerequisites**: Must have `keys/alice_publisher_key.json` (create with `bin/lapctl keygen -name alice`)

Reset all LAP artifacts for Alice (complete refresh):

```bash
bin/lapctl reset-artifacts
```

-   **Purpose**: Complete reset of all LAP artifacts - creates new Namespace Attestation and updates all posts
-   **Output**: Creates new `_la_namespace.json`, `_la_resource.json` and `index.htmx` for posts 1-3, updates host file
-   **Optional**: `-base` (default: `http://localhost:8081`), `-root` (default: `apps/server/static/publisherapi/people/alice`), `-keys-dir` (default: `keys`)
-   **Prerequisites**: Must have `keys/alice_publisher_key.json` (create with `bin/lapctl keygen -name alice`)

Create a Resource Attestation (RA) for an HTML file:

```bash
# absolute URL
bin/lapctl ra-create \
  -in apps/server/static/publisherapi/people/alice/posts/1/index.html \
  -url http://localhost:8081/people/alice/posts/1 \
  -publisher-claim f1a2d3c4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00 \
  -namespace-attestation-url https://localhost:8081/people/alice/_la_namespace.json

# or relative URL with -base
bin/lapctl ra-create \
  -in apps/server/static/publisherapi/people/alice/posts/1/index.html \
  -url /people/alice/posts/1 -base http://localhost:8081 \
  -publisher-claim f1a2d3c4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00 \
  -namespace-attestation-url https://localhost:8081/people/alice/_la_namespace.json
```

-   **Purpose**: Creates an unsigned Resource Attestation JSON that links content to its publisher
-   **Output**: Writes RA JSON to `<dir>/_la_resource.json` by default (override with `-out`)
-   **Content**: Includes SHA-256 hash of the HTML file, publisher's public key, and namespace attestation URL
-   **Required**: `-publisher-claim` (64-char hex secp256k1 X-only public key) and `-namespace-attestation-url`
-   **Optional**: `-base` for resolving relative URLs, `-out` for custom output path

Create a Namespace Attestation (NA) for a namespace:

```bash
bin/lapctl na-create \
  -namespace https://localhost:8081/people/alice/ \
  -exp 1754909400 \
  -privkey a1b2c3d4e5f6071829384756647382910abcdef1234567890fedcba0987654321
```

-   Writes NA JSON to `<dir>/_la_namespace.json` by default (override with `-out`)
-   Required: `-namespace` URL
-   Optional: `-exp` expiration timestamp (default: 1 year from now), `-privkey` for specific key, `-rotate` to force new keypair

Show help:

```bash
bin/lapctl help
```

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
