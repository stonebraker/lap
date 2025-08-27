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

# Verification Result Contract v2

## Design Philosophy

This v0.2 contract focuses on the core purpose of Linked Attestations: proving publisher-resource association through two linked attestations that can be cleanly dissociated when needed.

The verification establishes:

1. **Resource Integrity** - The content bytes match what was attested
2. **Resource Origination** - The resource attestation comes from the same origin as the claimed URL
3. **Resource Freshness** - The resource attestation is still being served (live verification)
4. **Publisher Resource Association** - The publisher controls the namespace containing the resource

When all checks pass, we have proof of publisher-resource association.

## Protocol Artifacts

LAP verification involves three key artifacts. For detailed canonical schemas, see [artifacts.md](artifacts.md). For cryptographic specifications, see [crypto-spec.md](crypto-spec.md). For role definitions and responsibilities, see [roles-spec.md](roles-spec.md).

### Fragment

An HTML fragment containing:

-   **Canonical content bytes**: Embedded in a `<link>` element's `data:` URL (e.g., `href="data:text/html;base64,..."`)
-   **Resource URL**: The fragment's claimed resource URL (derived from context)
-   **Publisher claim**: Publisher's public key (from `data-la-publisher-claim` attribute in the `<link>` element)
-   **Resource Attestation URL**: URL where Resource Attestation can be fetched (from `data-la-resource-attestation-url` attribute)
-   **Namespace Attestation URL**: URL where Namespace Attestation can be fetched (from `data-la-namespace-attestation-url` attribute)

### Resource Attestation (RA)

An unsigned JSON document fetched from the network containing:

-   **`url`**: The resource URL this attestation covers
-   **`hash`**: SHA-256 hash of the canonical content bytes
-   **`namespace_attestation_url`**: URL pointing to the Namespace Attestation (required)

### Namespace Attestation (NA)

A JSON document fetched from the network containing:

-   **`payload.namespace`**: The namespace URL under publisher control
-   **`payload.exp`**: Expiration timestamp (epoch seconds UTC)
-   **`key`**: Publisher's secp256k1 X-only public key
-   **`sig`**: Schnorr signature over SHA256(payload_json)

## Result Object (normative)

Every verifier MUST return a single object with these fields:

```json
{
    "verified": true,
    "resource_integrity": "pass",
    "resource_origination": "pass",
    "resource_freshness": "pass",
    "publisher_resource_association": "pass",
    "failure": null,
    "context": {
        "resource_attestation_url": "https://example.com/people/alice/posts/123/_la_resource.json",
        "namespace_attestation_url": "https://example.com/people/alice/_la_namespace.json",
        "verified_at": 1641000000
    }
}
```

### Core Fields

-   **verified**: MUST be `true` only when all checks pass and publisher-resource association is proven
-   **resource_integrity**: Status of content hash verification
-   **resource_origination**: Status of resource attestation signature verification
-   **resource_freshness**: Status of resource attestation time window validation
-   **publisher_resource_association**: Status of namespace attestation and URL association
-   **failure**: Details about the first check that failed (null if verified=true)
-   **context**: Essential metadata for debugging including resource URL, attestation URLs, and verification timestamp

### Check Status Values

Each check MUST have one of these values:

-   `"pass"` - Check succeeded
-   `"fail"` - Check failed (verification fails)
-   `"skip"` - Check was not performed (e.g., missing attestation)

### Failure Object

When `verified=false`, the `failure` field MUST contain:

```json
{
    "check": "resource_integrity",
    "reason": "hash_mismatch",
    "message": "Resource content hash does not match attestation",
    "details": {
        "expected": "abc123...",
        "actual": "def456..."
    }
}
```

## Check Definitions

### Resource Integrity

**Check code:** `resource_integrity`

Verifies the fragment's canonical content bytes match the hash in the fetched resource attestation.

**Pass conditions:**

-   SHA-256 hash of fragment's canonical content bytes (from `<link>` data URL) matches `hash` in fetched RA

**Failure reasons:**

-   `hash_mismatch` - SHA-256 of fragment's canonical content bytes differs from fetched RA's `hash`

### Resource Origination

**Check code:** `resource_origination`

Verifies the resource attestation comes from the server that controls the resource URL.

**Pass conditions:**

-   Fetched RA is well-formed JSON
-   Fetched RA URL has the same origin as the fragment's claimed resource URL
-   Fetched RA's `url` field matches fragment's claimed resource URL

**Failure reasons:**

-   `malformed` - Fetched RA JSON is invalid or missing required fields
-   `origin_mismatch` - Fetched RA URL origin differs from resource URL origin
-   `content_url_mismatch` - Fetched RA's `url` differs from fragment's claimed resource URL
-   `fetch_failed` - Could not retrieve resource attestation from network

### Resource Freshness

**Check code:** `resource_freshness`

Verifies the resource attestation is still being served, indicating ongoing publisher commitment to the association claim.

**Pass conditions:**

-   Resource Attestation is successfully fetched from the expected URL (live verification)

**Failure reasons:**

-   `fetch_failed` - Could not retrieve resource attestation from network (indicates dissociation)

### Publisher Resource Association

**Check code:** `publisher_resource_association`

Verifies the publisher controls the namespace containing the resource.

**Pass conditions:**

-   Fragment's `data-la-publisher-claim` (from `<link>` element) matches the `key` in the fetched NA
-   Fetched NA is well-formed JSON
-   Fetched NA's `sig` validates against its `key`
-   Fragment's resource URL falls under the namespace in fetched NA's `payload.namespace`
-   Current time is before fetched NA's `payload.exp` (expires at)

**Failure reasons:**

-   `publisher_claim_mismatch` - Fragment's `data-la-publisher-claim` (from `<link>` element) differs from fetched NA's `key`
-   `malformed` - Fetched NA JSON is invalid or missing required fields
-   `signature_invalid` - Fetched NA's `sig` does not validate against its `key`
-   `fetch_failed` - Could not retrieve namespace attestation from network
-   `url_not_under_namespace` - Fragment's resource URL not under the namespace in fetched NA's `payload.namespace`
-   `expired` - Fetched NA's `payload.exp` timestamp has passed

## Example Results

### Successful Verification

```json
{
    "verified": true,
    "resource_integrity": "pass",
    "resource_origination": "pass",
    "resource_freshness": "pass",
    "publisher_resource_association": "pass",
    "failure": null,
    "context": {
        "resource_attestation_url": "https://example.com/people/alice/posts/123/_la_resource.json",
        "namespace_attestation_url": "https://example.com/people/alice/_la_namespace.json",
        "verified_at": 1641000000
    }
}
```

### Failed Verification (Hash Mismatch)

```json
{
    "verified": false,
    "resource_integrity": "fail",
    "resource_origination": "skip",
    "resource_freshness": "skip",
    "publisher_resource_association": "skip",
    "failure": {
        "check": "resource_integrity",
        "reason": "hash_mismatch",
        "message": "SHA-256 of fragment's canonical content bytes does not match fetched RA's hash",
        "details": {
            "expected": "abc123def456...",
            "actual": "789xyz012abc..."
        }
    },
    "context": {
        "resource_attestation_url": "https://example.com/people/alice/posts/123/_la_resource.json",
        "namespace_attestation_url": "https://example.com/people/alice/_la_namespace.json",
        "verified_at": 1641000000
    }
}
```

### Failed Verification (No Association)

```json
{
    "verified": false,
    "resource_integrity": "pass",
    "resource_origination": "pass",
    "resource_freshness": "pass",
    "publisher_resource_association": "fail",
    "failure": {
        "check": "publisher_resource_association",
        "reason": "url_not_under_namespace",
        "message": "Resource URL is not under the attested namespace",
        "details": {
            "resource_url": "https://other.com/posts/123",
            "namespace": "https://example.com/people/alice/"
        }
    },
    "context": {
        "resource_attestation_url": "https://other.com/posts/123/_la_resource.json",
        "namespace_attestation_url": "https://example.com/people/alice/_la_namespace.json",
        "verified_at": 1641000000
    }
}
```

## Implementation Benefits

This simplified design provides:

1. **Clear Purpose** - Directly maps to LAP's core goal of proving publisher-resource association
2. **Fail Fast** - Stop at first failure, skip remaining checks
3. **Logical Flow** - Integrity → Origination → Freshness → Association
4. **Clean Dissociation** - When attestations are removed, verification clearly fails
5. **Minimal Complexity** - No policies, grace periods, or unnecessary features

## Removed Complexity

This v2 eliminates several features from v1 that don't serve the core LAP purpose:

-   **Policies** - Verification either works or it doesn't
-   **Grace periods** - Expired attestations fail, period
-   **ETag validation** - Hash verification is sufficient
-   **Size validation** - Hash verification covers this
-   **Clock skew tolerance** - Keep time validation simple
-   **Warning states** - Either verified or not

The result is a verification contract focused solely on what LAP needs: proving (or disproving) publisher-resource association through linked attestations.
