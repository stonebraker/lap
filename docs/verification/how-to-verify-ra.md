<!--
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
-->

# Resource Attestation Verification Specification

Below is normative guidance for verifying a Resource Attestation (RA) for an HTML fragment fetched from the web.

---

Verifier procedure (MUST / SHOULD)

1. Select the representation to verify

-   Input: a final response for a resource URL R with entity body bytes B and response headers H (notably ETag).
-   Follow same-origin redirects only. The final same-origin URL after redirects is the fetched URL F.
-   Keep Content-Encoding as received; verification is over the exact bytes B presented to the client.

2. Canonicalize the fetched URL (payload.url)

-   Starting from F, produce canonical URL U:
    -   Lowercase scheme and host
    -   Strip default ports (:80 for http, :443 for https)
    -   Include path and query exactly; exclude fragment
    -   Perform RFC 3986 dot-segment removal; percent-decode once where legal
    -   Do not add or remove a trailing slash unless it is part of the selected path

3. Locate and fetch the attestation document

-   Construct the canonical attestation URL AU by taking U and replacing the path suffix as follows:
    -   Remove any trailing /index.html or /index.json from the path
    -   Append /\_la_resource.json
-   Perform GET AU (same-origin). Response body must be a JSON object with fields described below.

4. Parse the attestation object

-   The attestation JSON MUST be of the form:
    {
    "payload": {
    "url": string,
    "attestation_url": string,
    "hash": string, // "sha256:" + 64-lower-hex
    "etag": string, // exact token expected in resource response
    "iat": number, // seconds since epoch (UTC)
    "exp": number, // seconds since epoch (UTC)
    "kid": string
    },
    "resource_key": string, // 64-lower-hex x-only secp256k1 public key
    "sig": string // 128-lower-hex BIP-340 Schnorr over SHA-256(payload JSON bytes)
    }

5. Perform structural and origin checks

-   payload.url MUST equal U (the canonicalized fetched URL from step 2).
-   payload.attestation_url MUST equal AU (the canonical attestation URL from step 3).
-   AU MUST be same-origin with U (scheme/host/port).

6. Validate freshness window

-   Let now = current UTC seconds.
-   MUST satisfy payload.iat <= now < payload.exp.

7. Validate content hash and ETag

-   Compute h = sha256(B). Let Hhex = lowercase hex(h).
-   MUST satisfy payload.hash == "sha256:" + Hhex.
-   If the resource response included an ETag header, MUST satisfy payload.etag == that exact token (string match).

8. Verify signature (deterministic payload canonicalization)

-   Canonicalize payload for signing by serializing exactly these fields in this order as compact RFC 8259 JSON (no extra whitespace):
    url, attestation_url, hash, etag, iat, exp, kid
-   Let P = UTF-8 bytes of that canonical JSON.
-   Compute m = sha256(P).
-   Decode resource_key (x-only, 64-hex) and sig (128-hex).
-   Verify BIP-340 Schnorr signature over m using resource_key. MUST be valid.

9. Optional single-fetch header mode

-   If the resource response includes header:
    Attestation: BASE64URL( { "payload":{…}, "resource_key":"…", "sig":"…" } )
-   Verifier MAY parse and validate using steps 4–8 without fetching AU. If both header and JSON endpoint are present, implementations MAY prefer the header for latency, but SHOULD consider them equivalent if both validate.

10. Redirect and cross-origin policy

-   All verification above assumes same-origin throughout. Cross-origin redirects MUST NOT be followed for the purpose of attestation; if encountered, verification MUST fail.

Result

-   The RA is valid for U and B at the current time if and only if all checks in steps 5–8 succeed (and step 9 if using header mode).
-   This establishes a live proof of bytes↔URL authenticity. To establish Proof of Publisher, verifiers MUST additionally validate a Namespace Attestation for the publisher’s namespace as specified separately, and confirm that U lies under that namespace.

## JavaScript Implementation Guide (non-normative)

The LAP verifier provides two approaches for implementing BIP-340 Schnorr signature verification in web browsers:

### ES Module Approach (Recommended for Modern Environments)

For applications served via HTTP/HTTPS with ES module support:

```html
<script type="module">
    import { verifyFragment } from "/js/la/verifier.core.js";
    import { schnorr } from "/js/vendor/noble-secp256k1.js";

    const result = await verifyFragment({
        stapled: attestationObject,
        verifyBytes: async () => {
            // Implement content hash verification
            const computed = await computeSHA256(resourceBytes);
            return computed === expectedHash ? null : "hash mismatch";
        },
        verifySigImpl: schnorr, // Optional: uses noble-secp256k1
    });

    console.log(result); // { ok: true, status: "ok", code: "LA_OK" }
</script>
```

**Advantages:**

-   Clean, modern syntax
-   Tree-shaking and bundler support
-   Explicit dependency management
-   TypeScript compatibility

### Script Tag Approach (Universal Compatibility)

For maximum compatibility across all environments (including `file://` protocol):

```html
<!-- Load crypto library via script tag -->
<script src="/js/vendor/noble-secp256k1-global.js"></script>

<script type="module">
    import { verifyFragment } from "/js/la/verifier.core.js";

    // Wait for crypto library to load asynchronously
    function waitForCrypto() {
        return new Promise((resolve, reject) => {
            if (window.NobleSecp256k1?.schnorr) {
                resolve(window.NobleSecp256k1.schnorr);
                return;
            }
            window.addEventListener("noble-loaded", () => {
                resolve(window.NobleSecp256k1.schnorr);
            });
            setTimeout(() => reject(new Error("Crypto timeout")), 5000);
        });
    }

    const schnorr = await waitForCrypto();

    const result = await verifyFragment({
        stapled: attestationObject,
        verifyBytes: async () => {
            /* ... */
        },
        verifySigImpl: schnorr,
    });
</script>
```

**Advantages:**

-   Works in any browser environment
-   No module loader dependencies
-   Graceful fallback handling
-   Offline-capable with vendored libraries

### Cryptographic Library Details

Both approaches use the `@noble/secp256k1` library for BIP-340 Schnorr signature verification:

-   **Library**: [@noble/secp256k1](https://github.com/paulmillr/noble-secp256k1) v1.7.1
-   **Algorithm**: BIP-340 Schnorr signatures over secp256k1
-   **Verification**: `schnorr.verify(signature, message, publicKey)`
-   **Format**: 128-hex signature, 32-byte message hash, 64-hex x-only public key

### Fallback Strategy

The verifier implements a three-tier fallback strategy:

1. **Injected Implementation**: Use `verifySigImpl` parameter if provided
2. **Global Library**: Use `window.NobleSecp256k1.schnorr` if available (script tag approach)
3. **Dynamic Import**: Fall back to ES module import of vendored library

This ensures maximum reliability across different deployment scenarios.

### Testing

Comprehensive test suites are available:

-   **ES Module Tests**: `/tests/core/core.test.html` (requires HTTP server)
-   **Script Tag Tests**: `/tests/core/core-scripttag.test.html` (universal compatibility)

---

Notes (non-normative)

-   Using identity encoding avoids ambiguity; if resources are served compressed, publishers and verifiers MUST agree on the exact representation used for hashing.
-   The kid is an operational label only; it is signed within the payload but SHOULD NOT be used for trust decisions.
