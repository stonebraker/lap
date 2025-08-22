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

# How to Create Resource Attestations

### Server procedure (MUST / SHOULD)

1. **Select the representation (bytes to attest)**

    - Produce the response **entity body** bytes `B` for the requested resource (the exact bytes the client will read for this representation, i.e., after content negotiation and encodings you apply).
    - Servers SHOULD prefer `Content-Encoding: identity` for attested resources to avoid ambiguity.

2. **Compute the ETag (required)**

    - Compute a stable token over `B` and set the response’s `ETag` header to that exact value.
    - Example (recommended): weak ETag over the body digest: `W/"` + `hex(sha256(B))[:32]` + `"` (format is up to you, but it MUST be repeated in the payload verbatim).

3. **Canonicalize the effective URL (`url`)**

    - Start from the final, same-origin URL that produced `B` (after any same-origin redirects).
    - Lowercase `scheme` and `host`. Strip default ports (`:80` for http, `:443` for https).
    - Include **path and query** exactly as selected; exclude fragment.
    - Do RFC 3986 dot-segment removal; percent-decode once where legal; do not add or remove a trailing slash unless it’s part of the selected path.
    - The resulting absolute URL string is `payload.url`.

4. **Construct the canonical attestation URL (`attestation_url`)**

    - Append `/_lap/resource_attestation` to the resource path of `payload.url` (no query, no fragment).
    - Use the same scheme/host/port as `payload.url`.
    - The resulting absolute URL string is `payload.attestation_url`.

5. **Hash the body (`hash`)**

    - Compute `h = sha256(B)` (32 bytes).
    - Set `payload.hash = "sha256:" + lowercase_hex(h)` (64 hex chars).

6. **Set freshness window (`iat`, `exp`)**

    - `payload.iat = floor(now_utc_seconds)`
    - `payload.exp = payload.iat + T`, where `T` ≤ **600 seconds** is RECOMMENDED.

7. **Identify the resource key (`kid`)**

    - Use a generated, per-item resource keypair (secp256k1, x-only public).
    - Set `payload.kid` to a server-chosen identifier for this key (string; version/label).
    - Derive `resource_key` (32-byte x-only pubkey) and encode as **lowercase hex** (64 hex chars). You MUST retain the private key to sign.

8. **Build the canonical payload JSON (deterministic)**

    - The **only** fields are, in this exact order:
      `url, attestation_url, hash, etag, iat, exp, kid`
    - Encode as compact RFC 8259 JSON: no extra whitespace; numbers in base-10 without leading zeros; strings with standard escapes; field order as above.
    - Let `payload_json_bytes` be the exact UTF-8 bytes of this JSON.

9. **Sign the payload (`sig`)**

    - Compute `m = sha256(payload_json_bytes)`.
    - Compute a **BIP-340 Schnorr** signature over `m` using the **resource key’s private key**.
    - Encode the 64-byte signature as **lowercase hex** (128 hex chars) → `sig`.

10. **Assemble the Resource Attestation Object**

    ```json
    {
      "payload": { ... },             // exactly the JSON you signed
      "resource_key": "<64-hex>",     // x-only secp256k1 pubkey (lowercase hex)
      "sig": "<128-hex>"              // Schnorr over SHA-256(payload_json_bytes)
    }
    ```

11. **Publish (two equivalent delivery modes)**

    - **Canonical endpoint (REQUIRED):**

        - `GET <payload.attestation_url>` → the JSON object above.
        - `Content-Type: application/json`
        - `Cache-Control: no-store` (RECOMMENDED).

    - **Inbound header (OPTIONAL, for single-fetch verification):**

        - On the resource response that served `B`, include

            ```
            Attestation: BASE64URL( { "payload":{…}, "resource_key":"…", "sig":"…" } )
            Access-Control-Expose-Headers: Attestation, ETag
            ```

        - If you also send a legacy `Resource-Key` header, it MUST match `resource_key` byte-for-byte.

12. **Redirect policy (security)**

    - When generating `payload.url` and serving `payload.attestation_url`, do not cross origins. Same-origin redirects are allowed; cross-origin redirects MUST NOT be used for attestation.

---

### Notes (non-normative)

-   Using `Content-Encoding: identity` keeps the “exact bytes” unambiguous for both server and verifier. If you serve compressed bytes, ensure verifiers hash the same representation you attested.
-   Keep `T` short (≤10 minutes) for good UX and replay bounds.
-   If you rotate the per-item key, update `kid` and republish the attestation (past signatures remain valid for their windows).
