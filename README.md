# Linked Attestations Protocol (LAP)

This repository contains:

-   **publisherapi**: static file server for demonstrating LAP protocol with live examples
-   **verifier-cli**: CLI tool for LAP resource attestation verification with full cryptographic validation
-   **tools-cli (lapctl)**: primary CLI for LAP operations including key generation and attestation creation

There are two Go modules tied together by `go.work` at the repo root:

-   Root module (servers, CLI): `module lap`
-   SDK module (libraries): `sdks/go` (module `github.com/stonebraker/lap/sdks/go`)

**LAP Protocol Status**: This is a complete implementation of the Linked Attestations Protocol (LAP) with working cryptographic verification, cross-language support (Go + JavaScript), and comprehensive test coverage. The protocol is ready for production use.

📖 **[Read the Complete Protocol Specification →](docs/protocol-overview.md)**

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

Build all (workspace):

```bash
go build ./...
```

### Repository layout

```
apps/
  server/
    cmd/
      publisherapi/           # demo server
    internal/
      httpx/                  # HTTP utilities
    static/
      publisherapi/           # demo content
        evidence/
        js/                   # JavaScript verifier
        people/
          alice/              # demo publisher
            posts/            # example LAP content
              1/, 2/, 3/      # posts with attestations
        tests/                # verification test matrices
  tools-cli/
    cmd/
      lapctl/               # primary LAP CLI tool
  verifier-cli/
    cmd/
      verifier/             # verification CLI
docs/
  examples/                 # implementation examples
  protocol-overview.md      # complete protocol spec
sdks/
  go/                       # Go SDK (separate module)
    pkg/lap/
      canonical/
      crypto/
      verify/
      wire/
```

The implementation uses secp256k1 cryptography for resource attestations with comprehensive verification including hash validation, signature verification, and drift detection.

### Tools CLI: lapctl

Build the tools CLI:

```bash
go build -o bin/lapctl ./apps/tools-cli/cmd/lapctl
```

Create a Resource Attestation (RA) for an HTML file:

```bash
# absolute URL
bin/lapctl ra-create \
  -in apps/server/static/publisherapi/people/alice/posts/1/index.html \
  -url http://localhost:8081/people/alice/posts/1 \
  -kid v1

# or relative URL with -base
bin/lapctl ra-create \
  -in apps/server/static/publisherapi/people/alice/posts/1/index.html \
  -url /people/alice/posts/1 -base http://localhost:8081 \
  -kid v1
```

-   Writes RA JSON to `<dir>/_lap/resource_attestation.json` by default
-   Stores/reads per-resource keys under `keys/<input-path>/resource_key.json` (override with `-keys-dir`)
-   Freshness window: `-window-min` (default 10); optional `-ttl` seconds
-   Optional: `-etag`, `-privkey`, `-rotate`, `-out`  
    -etag: Overrides the ETag embedded in the attestation. If not set, a weak ETag is computed as W/"<sha256(body)>". Use this to match a server-provided ETag exactly.  
    -privkey: Use a specific hex-encoded secp256k1 private key for the resource. If provided, it’s used for signing and stored under the -keys-dir path.  
    -rotate: Forces a new per-resource keypair even if one already exists (unless -privkey is given). The new key is stored and used to sign, changing the resource_key.

Create a fragment (index.htmx) and matching RA from `index.html`:

```bash
bin/lapctl fragment-create \
  -in apps/server/static/publisherapi/people/alice/posts/1/index.html \
  -url /people/alice/posts/1 \
  -base http://localhost:8081 \
  -window-min 10 \
  -kid key-post-1
```

-   Writes fragment to `<dir>/index.htmx` and RA to `<dir>/_lap/resource_attestation.json`
-   Same key handling and timebox flags as `ra-create`

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

-   noble-secp256k1 for JavaScript cryptographic operations
-   btcd/btcec for Go cryptographic operations

See NOTICE file for complete attribution details.
