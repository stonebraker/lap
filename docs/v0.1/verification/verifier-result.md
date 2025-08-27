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

# Verification Result Contract

## Result object (normative)

Every verifier MUST return a single object with these fields:

```json
{
  "ok": true,
  "status": "ok | warn | error",
  "code": "LA_OK | LA_EXPIRED | LA_HASH_MISMATCH | ...",
  "message": "string (NON‑normative; MAY be localized)",
  "details": { "…impl-defined but RECOMMENDED keys…" },
  "telemetry": {
    "url": "string",
    "kid": "string|nullable",
    "iat": 0,
    "exp": 0,
    "now": 0,
    "policy": "strict|graceful|auto-refresh|offline-fallback"
  }
}
```

-   **ok**: MUST be `true` only when the attestation is valid _and_ fresh per the active policy.
-   **status**: coarse bucket for quick handling.
-   **code**: REQUIRED, stable identifier; drives programmatic behavior and conformance tests.
-   **message**: OPTIONAL, non‑normative; implementers MAY customize.
-   **details**: OPTIONAL; see per‑code suggestions below.
-   **telemetry**: RECOMMENDED; aids debugging and UI.

## Code namespace and semantics (normative)

Use the `LA_` prefix. Codes are stable strings (not numbers) so they’re readable in logs and won’t collide across languages. If you also want numeric IDs, add a parallel `num` field—string codes stay authoritative.

### Success

-   `LA_OK` — All required checks passed and attestation is fresh.

    -   details: `{ "fresh_until": <exp> }`

### Warnings (verification succeeded, but attention suggested)

-   `LA_EXPIRED_GRACE` — Sig/hash valid, but `now > exp` and policy accepted it.

    -   details: `{ "expired_at": <exp>, "skew": <seconds> }`

-   `LA_OFFLINE_STALE` — Last known valid result surfaced due to offline policy.

    -   details: `{ "cached_at": <ts> }`

### Fetch / transport

-   `LA_FETCH_FAILED` — Network or HTTP (e.g., non‑200) blocked verification.

    -   details: `{ "http_status": 0|>=300, "phase": "resource|bundle", "reason": "timeout|cors|..."}`

-   `LA_NO_ATTESTATION` — Neither headers nor bundle provided required material.

### Structure / parsing

-   `LA_ATTESTATION_MALFORMED` — Attestation JSON/base64 invalid or fails schema.
-   `LA_UNSUPPORTED_ALG` — Algorithm not recognized (e.g., non‑sha256 or non‑schnorr).

### Binding & integrity

-   `LA_URL_MISMATCH` — `payload.url` ≠ fetched URL.

    -   details: `{ "expected": "<url>", "actual": "<url>" }`

-   `LA_ETAG_MISMATCH` — `payload.etag` set but doesn’t match response `ETag`.

    -   details: `{ "expected": "W/\"…\"", "actual": "W/\"…\"" }`

-   `LA_HASH_MISMATCH` — Body bytes hashed ≠ `payload.hash`.

    -   details: `{ "expected_sha256": "<hex>", "actual_sha256": "<hex>" }`

-   `LA_SIG_INVALID` — Schnorr/EdDSA verification over `SHA256(payload_json)` failed.

    -   details: `{ "kid": "<id>" }`

-   `LA_KEY_INVALID` — Resource key is malformed or wrong length.

#### Live vs stapled drift (attestation refresh continuity)

These codes signal that the freshly fetched attestation no longer matches the stapled attestation on values that MUST remain stable across refresh windows.

-   `LA_URL_DRIFT` — `payload.url` differs between live and stapled attestations.

    -   details: `{ "expected": "<url_from_stapled>", "actual": "<url_from_live>" }`

-   `LA_ATTESTATION_URL_DRIFT` — `payload.attestation_url` differs between live and stapled.

    -   details: `{ "expected": "<url_from_stapled>", "actual": "<url_from_live>" }`

-   `LA_HASH_DRIFT` — `payload.hash` differs between live and stapled (distinct from bytes-vs-hash mismatch).

    -   details: `{ "expected_sha256": "<hex_from_stapled>", "actual_sha256": "<hex_from_live>" }`

-   `LA_RESOURCE_KEY_DRIFT` — `resource_key` differs between live and stapled.

    -   details: `{ "expected": "<key_from_stapled>", "actual": "<key_from_live>" }`

-   `LA_KID_DRIFT` — `payload.kid` differs between live and stapled.

    -   details: `{ "expected": "<kid_from_stapled>", "actual": "<kid_from_live>" }`

### Freshness / time

-   `LA_IAT_IN_FUTURE` — `now < iat - skew`.
-   `LA_EXPIRED` — `now > exp + skew` and policy is strict (hard failure).
-   `LA_CLOCK_SKEW_EXCESSIVE` — Out of tolerated skew window.

### Policy / flow

-   `LA_EXPIRED_AFTER_REFRESH` — Auto‑refresh attempted; still expired.
-   `LA_POLICY_REJECTED` — Caller policy forbids accepting this result (e.g., offline not allowed).

> Additive rule: new codes MAY be added later; existing codes MUST NOT change meaning.

## Policy semantics (normative)

The `telemetry.policy` field declares how the verifier evaluated freshness and outages. Policies are identifiers, not configuration snapshots.

-   **`strict`** — Baseline conformance.

    -   Requires a successful live fetch of the attestation bundle.
    -   Enforces bytes integrity, drift checks, and time window strictly.
    -   Outcomes:
        -   Fresh: `LA_OK`.
        -   `now > exp (+skew)`: `LA_EXPIRED` (status `error`).
        -   Fetch/transport failure: `LA_FETCH_FAILED` (status `error`).
        -   All binding/integrity issues are `error` codes (e.g., `LA_HASH_MISMATCH`).

-   **`graceful`** — Accepts brief staleness as a warning.

    -   Same as `strict`, but if the only failure is freshness and it is within the configured grace/skew window, verification succeeds with a warning.
    -   Outcomes:
        -   Fresh: `LA_OK`.
        -   Slightly expired (within grace): `LA_EXPIRED_GRACE` (status `warn`).
        -   Beyond grace: `LA_EXPIRED` (status `error`).
        -   Transport and binding/integrity failures remain `error`.

-   **`auto-refresh`** — Attempt to recover freshness before returning.

    -   On staleness (`notyet`/`expired`), the verifier may refetch or re-resolve once (or a small number of times) before deciding.
    -   Outcomes:
        -   Refresh succeeds and is fresh: `LA_OK`.
        -   Still expired after refresh: `LA_EXPIRED_AFTER_REFRESH` (status `error`).
        -   Transport failure during refresh: `LA_FETCH_FAILED` (status `error`).
        -   Other failures are identical to `strict`.

-   **`offline-fallback`** — Prefer availability with explicit stale signaling.

    -   If the live fetch fails, the verifier MAY surface the last known good result (if present) instead of erroring.
    -   Requirements:
        -   The stapled bytes must still pass integrity checks.
        -   A previous `LA_OK` (or `LA_EXPIRED_GRACE`) must be available in cache along with its timestamps.
    -   Outcomes:
        -   Live fetch OK and fresh: `LA_OK`.
        -   Live fetch fails but prior good exists: `LA_OFFLINE_STALE` (status `warn`, with `details.cached_at`).
        -   Live fetch fails and no prior good exists: `LA_FETCH_FAILED` (status `error`).

Notes:

-   Policies do not change the meaning of binding/integrity drift codes; those are always errors.
-   “Skew”/grace is implementation-configurable; see defaults below.

## Minimal mapping to UI (non‑normative)

-   `status=ok`: green check.
-   `status=warn`: yellow clock (“expired but provenance verified”).
-   `status=error`: red x with actionable hint (from `details`).

## Suggested messages (non‑normative templates)

Implementations MAY start with these English defaults:

-   `LA_HASH_MISMATCH`: “Content bytes don’t match the attested hash.”
-   `LA_SIG_INVALID`: “Signature does not verify with the resource key.”
-   `LA_EXPIRED`: “Attestation expired at {expISO}.”
-   `LA_URL_MISMATCH`: “Attestation URL differs from the fetched URL.”

## Why this split works

-   **Interop & tests:** Conformance tests key off `code`, not prose.
-   **Freedom to localize:** UIs can emit friendlier or localized `message`s.
-   **Matches client shape:** The codes map 1:1 to the reason states being returned in prototypes (e.g., `expired`, `sig-mismatch`, `hash-mismatch`, `fetch-failed`, `no-attestation`, `url-mismatch`, `etag-mismatch`).

## Drop‑in defaults (practical)

-   Default **policy** = `strict`.
-   Default **skew** = 120s.
-   Always populate `telemetry.iat|exp|now|policy`; many UIs and logs want them even on errors.
