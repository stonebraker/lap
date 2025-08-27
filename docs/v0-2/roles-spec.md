# LAP Protocol Roles Specification v0.2

This document defines the roles and responsibilities of participants in the LAP protocol ecosystem.

## Overview

LAP involves three primary roles that work together to establish publisher-resource association:

1. **Server** - Serves resource fragments and attestations
2. **Client** - Consumes and renders resource fragments
3. **Verifier** - Performs verification of publisher-resource association

## Server Role

The Server generates, signs, and serves LAP protocol artifacts.

### MUST Requirements

**Resource Management:**

-   Servers MUST generate Resource Attestations for each fragment
-   Servers MUST ensure Resource Attestations are served from the same origin as the resource

**Fragment Generation:**

-   Servers MUST embed canonical content bytes in the fragment's `<link>` element using `href="data:text/html;base64,..."`
-   Servers MUST include the SHA-256 hash of canonical content bytes in the `<link>` element's `data-hash` attribute
-   Servers MUST embed Resource Attestation data in the `<link>` element's data attributes
-   Servers MUST include the publisher's public key in the fragment's `data-la-publisher-claim` attribute
-   Servers MUST specify the Resource Attestation URL in the `<link>` element's `data-la-resource-attestation-url` attribute
-   Servers MUST specify the Namespace Attestation URL in the `<link>` element's `data-la-namespace-attestation-url` attribute
-   Servers MUST ensure the Resource Attestation URL is served from the same origin as the resource URL

**Resource Attestation:**

-   Servers MUST create a Resource Attestation for each fragment
-   Servers MUST sign the Resource Attestation using the resource's private key
-   Servers MUST serve the Resource Attestation at the URL specified in `payload.attestation_path`
-   Servers MUST include `namespace_attestation_url` pointing to the Namespace Attestation

**Namespace Attestation:**

-   Servers MUST create a Namespace Attestation asserting control over their namespace
-   Servers MUST sign the Namespace Attestation using the publisher's private key
-   Servers MUST serve the Namespace Attestation at the specified URL
-   Servers MUST ensure the namespace covers all resources they publish

### SHOULD Requirements

-   Servers SHOULD use appropriate expiration times for attestations
-   Servers SHOULD implement proper key rotation strategies
-   Servers SHOULD serve fragments with appropriate CORS headers for cross-origin embedding

## Client Role

The Client fetches, embeds, and renders resource fragments.

### MUST Requirements

**Fragment Handling:**

-   Clients MUST preserve all LAP protocol attributes when embedding fragments
-   Clients MUST NOT modify the canonical content bytes in the `<link>` element
-   Clients MUST NOT alter the attestation URLs in the `<link>` element's `data-la-resource-attestation-url` and `data-la-namespace-attestation-url` attributes

**Verification State Handling:**

-   Clients MUST clearly indicate when a fragment has not been verified OR not render it
-   Clients MUST clearly indicate when a fragment has failed verification OR not render it
-   Clients are NOT required to indicate successful verification

**Preview Content Handling:**

-   Clients MUST NOT modify preview content in ways that misrepresent the canonical bytes
-   Clients SHOULD replace the contents of the `<section class="la-preview">` element with decoded canonical bytes from the `<link>` element when rendering
-   Clients MUST replace only the contents of the preview section, not the entire `<section>` element, to preserve fragment structure
-   Clients MUST handle canonical content decode/parse failures gracefully by falling back to preview content with clear verification failure indicators
-   Clients MUST sanitize canonical content before DOM injection to prevent XSS attacks
-   Clients SHOULD validate that decoded canonical content is well-formed HTML before replacement

### SHOULD Requirements

-   Clients SHOULD verify fragments before rendering (using a Verifier)
-   Clients SHOULD handle verification failures gracefully
-   Clients SHOULD respect fragment expiration times
-   Clients MUST provide means for users to manually verify at-rest fragments
-   If clients do not provide interactive verification means, they MUST provide means for users to copy a LAP fragment's html markup to clipboard

### Security Considerations

**Preview vs Canonical Content:**

-   The preview element contains human-readable content but is NOT cryptographically verified
-   Malicious clients can modify preview content while verification continues to pass
-   The canonical content bytes in the `<link>` element are the authoritative, verified content
-   Conforming clients SHOULD prioritize canonical bytes over preview content for rendering

## Verifier Role

The Verifier performs cryptographic verification of publisher-resource association.

### MUST Requirements

**Verification Process:**

-   Verifiers MUST perform all four verification checks: Resource Integrity, Resource Origination, Resource Freshness, and Publisher Resource Association
-   Verifiers MUST validate Resource Attestation signatures against the resource's public key
-   Verifiers MUST validate Namespace Attestation signatures against the publisher's public key
-   Verifiers MUST confirm the fragment's `data-la-publisher-claim` matches the Namespace Attestation's `key`

**Network Operations:**

-   Verifiers MUST fetch Resource Attestations from the URL specified in the fragment's `data-la-resource-attestation-url`
-   Verifiers MUST fetch Namespace Attestations from the URL specified in the fragment's `data-la-namespace-attestation-url` or the Resource Attestation's `namespace_attestation_url`
-   Verifiers MUST handle network failures gracefully

**Result Reporting:**

-   Verifiers MUST return verification results conforming to the normative result object specification
-   Verifiers MUST report specific failure reasons and error codes
-   Verifiers MUST implement fail-fast behavior (stop at first failure, skip remaining checks)

### Implementation Options

Verifiers can be implemented as:

-   **Verification libraries** vendored or imported by clients
-   **Verification web services** called by clients over HTTP/API
-   **Browser plugins** installed by users
-   **Custom verification programs** (though not recommended for most use cases)

### SHOULD Requirements

-   Verifiers SHOULD cache Namespace Attestations to optimize performance
-   Verifiers SHOULD implement appropriate timeout handling for network requests
-   Verifiers SHOULD validate attestation expiration times against current time

## Role Interactions

### Server → Client

1. Server generates fragment with embedded attestations
2. Client fetches fragment from server
3. Client embeds fragment in their content

### Client → Verifier

1. Client calls verifier with fragment
2. Verifier performs verification process
3. Verifier returns verification result to client

### Verifier → Server

1. Verifier fetches Resource Attestation from server
2. Verifier fetches Namespace Attestation from server
3. Verifier validates cryptographic signatures

## Conformance Levels

### Minimal Conformance

-   Servers MUST generate valid fragments and attestations
-   Clients MUST preserve fragment integrity
-   Verifiers MUST perform all required verification checks

### Full Conformance

-   All MUST requirements satisfied
-   All SHOULD requirements implemented
-   Proper error handling and security considerations addressed

## Security Model

The LAP protocol's security relies on:

-   **Cryptographic signatures** for authenticity
-   **Live attestations** for freshness and dissociation
-   **Namespace control** for publisher association
-   **Client conformance** for content integrity (weakest link)

The preview content represents the primary attack vector, as clients can modify it while verification continues to pass. Implementations should prioritize canonical content bytes over preview content for security-critical applications.
