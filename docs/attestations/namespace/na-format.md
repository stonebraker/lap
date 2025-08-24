# Namespace Attestation (Durable)

## What it asserts

A valid signature over the payload is a durable statement:

> "The holder of `publisher_key` asserts control of each namespace listed in `payload.namespace` for the window `[iat, exp]`."

No deniability or privacy is expected.

---

## Canonical endpoint

Serve the attestation **at the root** of each asserted namespace:

```
GET /…/alice/_la_namespace.json
→ 200 application/json
```

You may also mirror the same object at the canonical path for **each** namespace in the payload array.

Optional header form on normal responses within the namespace:

```
Namespace-Attestation: BASE64URL( { "payload":{…}, "publisher_key":"…", "sig":"…" } )
Access-Control-Expose-Headers: Namespace-Attestation
```

---

## Payload (signed bytes)

```json
{
    "namespace": [
        "https://example.com/people/alice/",
        "https://www.example.com/people/alice/"
    ],
    "attestation_path": "_la_namespace.json",
    "iat": 1754908800,
    "exp": 1754909400,
    "kid": "publisher-key-2025-08-12"
}
```

### Canonicalization rules (musts)

-   Each entry is a **namespace prefix**: `scheme://host[:port]/path/` (trailing `/` required).
-   Lower‑case `scheme` and `host`; strip default ports (`:443` for https, `:80` for http).
-   Percent‑decode once; remove dot segments; keep a single trailing `/`.
-   IDNs must be punycoded.
-   **Sort** the array ascending (bytewise) after canonicalization and **deduplicate**.
-   Field order for the JSON string you sign is fixed:
    `namespace, attestation_path, iat, exp, kid`.

### Signature & encodings

-   Algo: **Schnorr (BIP‑340) over secp256k1**.
-   Message: `SHA‑256(payload_json_bytes)` of the **canonical** payload above.
-   `publisher_key`: 32‑byte x‑only public key (hex, lowercase).
-   `sig`: 64‑byte signature (hex, lowercase).

---

## Full object at the endpoint

```json
{
    "payload": {
        "namespace": [
            "https://example.com/people/alice/",
            "https://www.example.com/people/alice/"
        ],
        "attestation_path": "_la_namespace.json",
        "iat": 1754908800,
        "exp": 1754909400,
        "kid": "publisher-key-2025-08-12"
    },
    "publisher_key": "f1a2…e9c3",
    "sig": "a1b2…ff00"
}
```

**Caching:** `Cache-Control: public` is fine here (this is intentionally durable). Verifiers **must** enforce `iat/exp` locally; caches don't.

---

## Verifier logic (strict)

1. **Fetch** either:

    - The attestation using the `attestation_path` from one of the namespaces in question, or
    - a response within the namespace that carries `Namespace-Attestation` header.

2. **Canonicalize** the namespaces in `payload.namespace` and confirm:

    - The array is sorted/unique per the rules.
    - **At least one** entry is a prefix of the URL you used to fetch the attestation (prevents out‑of‑place serving).

3. **Freshness:** check `now ∈ [iat, exp]` (allow small clock skew).
4. **Signature:** rebuild the canonical payload JSON in the fixed field order, hash with SHA‑256, and verify **Schnorr** with `publisher_key`.
5. **Key discovery (optional but recommended for live proof):**
   Fetch `/_lap/keys/pub` from the same namespace and require it equals `publisher_key`. (This binds the published key to the namespace by location as well as by signature.)
6. **Redirects:** allow only **same‑origin** redirects during fetch.

**Output:** boolean `controls_namespace_now` plus `exp` for UI.

---

## JSON Schema (concise)

```json
{
    "$schema": "http://json-schema.org/draft-07/schema#",
    "title": "Namespace Attestation (Durable)",
    "type": "object",
    "required": ["payload", "publisher_key", "sig"],
    "properties": {
        "payload": {
            "type": "object",
            "required": ["namespace", "attestation_path", "iat", "exp"],
            "properties": {
                "namespace": {
                    "type": "array",
                    "minItems": 1,
                    "items": { "type": "string", "format": "uri" }
                },
                "attestation_path": { "type": "string" },
                "iat": { "type": "integer", "minimum": 0 },
                "exp": { "type": "integer", "minimum": 0 },
                "kid": { "type": "string" }
            },
            "additionalProperties": false
        },
        "publisher_key": { "type": "string", "pattern": "^[0-9a-f]{64}$" },
        "sig": { "type": "string", "pattern": "^[0-9a-f]{128}$" }
    },
    "additionalProperties": false
}
```

---

## Nice-to-haves

-   Publish `GET /…/_lap/keys/pub` → text/plain (hex) with `Cache-Control: public, max-age=86400, immutable`.
-   If multiple namespaces are asserted, **mirror** the same attestation at each namespace's canonical path (or at least one) so step (2) can succeed without external trust.

If you want, I can also draft the exact Go helpers for canonicalization + signing/verification that match these rules byte‑for‑byte.
