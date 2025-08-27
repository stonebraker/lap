# LAP Protocol Artifacts v0.2

This document defines the canonical schemas for all LAP protocol artifacts. These schemas are the authoritative source for implementers.

For cryptographic specifications, see [crypto-spec.md](crypto-spec.md). For role definitions and responsibilities, see [roles-spec.md](roles-spec.md).)

## Overview

LAP verification establishes publisher-resource association through three proofs:

1. **Resource Presence** - The Resource Attestation is accessible, demonstrating intent to distribute
2. **Resource Integrity** - The content bytes match what was attested
3. **Publisher Association** - The publisher controls the namespace containing the resource

## Protocol Artifacts

LAP consists of three primary artifacts:

1. **Fragment** (unsigned HTML)
2. **Resource Attestation** (unsigned JSON)
3. **Namespace Attestation** (signed JSON)

## Fragment

An HTML fragment containing embedded attestation data and canonical content bytes.

### Structure

```html
<article
    data-la-spec="v0-2"
    data-la-fragment-url="https://example.com/people/alice/posts/123"
>
    <section class="la-preview">
        <h2>Post 123</h2>
        <p>
            Kicking off a new project. Keeping things simple, minimal deps, and
            lots of clarity.
        </p>
    </section>

    <link
        rel="canonical"
        type="text/html"
        data-la-publisher-claim="f1a2d3c4e5f607..."
        data-la-resource-attestation-url="https://example.com/people/alice/posts/123/_la_resource.json"
        data-la-namespace-attestation-url="https://example.com/people/alice/_la_namespace.json"
        href="data:text/html;base64,base64-encoded-resource-content-here"
        hidden
    />
</article>
```

### Key Elements

-   **Root `<article>`**: Contains LAP protocol metadata including spec version and fragment URL
-   **Fragment URL**: `data-la-fragment-url` contains the canonical web address of this LAP fragment
-   **Preview `<section class="la-preview">`**: Human-readable content display (NOT cryptographically verified)
-   **Canonical `<link>`**: Contains verified content bytes in `href="data:text/html;base64,..."`, publisher claim, and pointers to Resource and Namespace Attestations
-   **Publisher claim**: `data-la-publisher-claim` contains the claimed publisher's secp256k1 X-only public key (64 hex chars) for cache optimization
-   **Resource Attestation URL**: `data-la-resource-attestation-url` specifies the complete URL where the Resource Attestation JSON can be fetched
-   **Namespace Attestation URL**: `data-la-namespace-attestation-url` specifies the complete URL where the Namespace Attestation JSON can be fetched

### Content Relationship

The **preview section** (`class="la-preview"`) contains human-readable content for display but is NOT cryptographically verified. The **canonical content bytes** in the `<link>` element represent the authoritative, verified content.

### Security Considerations

**Resource Attestation URL Constraints**: The `data-la-resource-attestation-url` provides complete flexibility in organizing attestation files, but MUST point to a URL served from the same origin as the resource URL. This ensures the Resource Attestation comes from the same authority that controls the resource.

**Examples of valid RA placement**:

-   Resource: `https://example.com/alice/posts/123` → RA: `https://example.com/alice/posts/123/_la_resource.json` ✅
-   Resource: `https://example.com/alice/posts/123` → RA: `https://example.com/alice/attestations/posts-123.json` ✅
-   Resource: `https://example.com/alice/posts/123` → RA: `https://other.com/attestations/123.json` ❌

**Preview vs Canonical Content**: Clients can modify preview content while verification continues to pass. The `class="la-preview"` attribute identifies the preview section for replacement. Conforming clients SHOULD replace preview content with decoded canonical bytes when rendering and MUST NOT modify preview content in ways that misrepresent the canonical bytes.

## Resource Attestation (RA)

An unsigned JSON document that verifies content integrity and points to a Namespace Attestation for publisher association.

### Schema

```json
{
    "url": "https://example.com/people/alice/posts/123",
    "hash": "sha256:7b0c...cafe",
    "namespace_attestation_url": "https://example.com/people/alice/_la_namespace.json"
}
```

### Fields

-   **`url`**: The resource URL this attestation covers
-   **`hash`**: SHA-256 hash of the canonical content bytes
-   **`namespace_attestation_url`**: URL pointing to the Namespace Attestation (required)

## Namespace Attestation (NA)

A JSON document that asserts publisher control over a namespace. Cryptographically signed by the publisher's key pair.

### Schema

```json
{
    "payload": {
        "namespace": "https://example.com/people/alice/",
        "exp": 1754909400
    },
    "key": "f1a2d3c4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00",
    "sig": "4e0f...<128-hex>...9c2a"
}
```

### Fields

-   **`payload.namespace`**: The namespace URL under publisher control (required)
-   **`payload.exp`**: Expiration timestamp (epoch seconds UTC) (required)
-   **`key`**: Publisher's secp256k1 X-only public key (64 hex chars)
-   **`sig`**: Schnorr signature over SHA256(payload_json) (128 hex chars)

## Verification Requirements

### Resource Presence

-   RA must be present and accessible at the expected URL (demonstrates intent to distribute)
-   Fetched RA must be well-formed JSON
-   RA's `url` must match fragment's claimed resource URL
-   RA must be fetched from the URL specified in fragment's `data-la-resource-attestation-url`
-   RA URL must be served from the same origin as the resource URL

### Resource Integrity

-   SHA-256 hash of fragment's canonical content bytes (from `<link>` data URL) must match `hash` in fetched RA

### Publisher Association

-   Fragment's `data-la-publisher-claim` must match the `key` in the fetched NA
-   Fetched NA must be well-formed JSON
-   NA's `sig` must validate against its `key`
-   Fragment's resource URL must fall under the namespace in NA's `payload.namespace`
-   Current time must be before fetched NA's `payload.exp`
