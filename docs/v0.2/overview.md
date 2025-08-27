# LAP Protocol Overview

> ⚠️ **UNSTABLE SPECIFICATION** ⚠️  
> This is LAP v0.2 preview documentation. The protocol is under active development and **subject to breaking changes**. Do not use in production.

## Overview

The LAP (Linked Attestation Protocol) provides reasonable proof of content integrity and publisher authenticity for peer-to-peer, distributed micro-content. It **_associates_** distributed Content with its original Publisher, regardless of where the Content appears on the web. Just as importantly, it allows a Publisher to **_dissociate_** from Content should they choose to do so. Dissociation leaves **_no durable evidence_** of a prior Publisher <-> Content association.

The Linked Attestation Protocol uses basic HTTP + a bit of cryptography, and requires no intermediary, central coordinator, blockchain, or token. It is designed to be very easy for developers and non-developers to implement utilizing lightweight libraries and SDKs.

In terms of LAP's ability to reasonably demonstrate Publisher <-> Content association and dissociation, it provides _a degree of associative assurance_ in the spectrum between unsigned content and cryptographically signed content:

| Unsigned Content | Unlinked Attestation  | <---------------> | Linked Attestation      | Signed Content    |
| :--------------- | :-------------------- | :---------------: | :---------------------- | :---------------- |
| (Zero Assurance) | (Very Weak Assurance) |                   | (Very Strong Assurance) | (Total Assurance) |

### Basic use case:

1. Alice posts a self-contained bit of micro-content (such as a typical social media post) to her own website and permits cross origin access to Jane.
2. Jane fetches the micro-content (an html fragment, not a full html doc), and embeds it in a page on her own website.
3. Eduardo visits Jane's site and is able to verify that the post he sees there actually came from Alice.
4. Alice decides to remove the content (dissociate) by removing the resource (fragment) and its Resource Attestation from the endpoint.
5. Eduardo attempts to re-verify Alice's post, but it fails verification.
    - He looks at the HTML markup for the fragment and a saved copy of the Resource Attestation and finds no cryptographic evidence that links Alice to the Content; only easily forgeable artifacts.

### What it does

This protocol concerns itself with linking a publisher to distributed micro-content, and allowing for live verification of that link. Conversely, it allows for the unlinking of a publisher from distributed micro-content, should the publisher choose, leaving no durable evidence of a prior Publisher <-> Content association.

### What it doesn't do

It does not concern itself with the fetching and embedding of content which can be easily implemented thanks to libraries like HTMX or Datastar and should one day be part of the HTTP spec.

### Disclaimer

Association and Dissociation are technical terms used to describe state within the confines of the Linked Attestations Protocol. They carry **no legal meaning** or implication as used for describing the protocol.

## Three Verification Goals

### 1. Resource Presence

The Resource Attestation is accessible at the expected endpoint, demonstrating intent to distribute.

### 2. Resource Integrity

The content bytes match what was attested by comparing SHA-256 hashes.

### 3. Publisher Association

The publisher controls the namespace containing the resource through a valid Namespace Attestation.

## Protocol artifacts

LAP consists of the following three primary artifacts:

1. Fragment (unsigned HTML)
2. Resource Attestation (unsigned JSON)
3. Namespace Attestation (signed JSON)

For detailed canonical schemas, see [artifacts.md](artifacts.md).  
For cryptographic specifications, see [crypto-spec.md](crypto-spec.md).  
For role definitions and responsibilities, see [roles-spec.md](roles-spec.md).

#### Verification Scenarios

-   **Inbound verification**: When fetching content directly from the publisher, only Publisher Association needs verification via the Namespace Attestation. If the Namespace Attestation is cached, zero network calls are needed. Typically initiated by clients using verifiers.
-   **At-rest verification**: When verifying embedded content, all three checks are performed using both Resource and Namespace Attestations. Namespace Attestations can be cached for optimization. Typically initiated by users to verify content from untrusted sources.

### Fragment

A self-contained HTML fragment that embeds both human-readable content and cryptographic attestation data. The fragment contains preview content for immediate display and canonical content bytes for verification, along with metadata that enables publisher-resource association verification.

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

#### Fragment Structure

-   **Root `<article>`**: Contains LAP protocol metadata including spec version and fragment URL
-   **Preview `<section class="la-preview">`**: Human-readable content for display (NOT cryptographically verified)
-   **Canonical `<link>`**: Contains the verified content bytes, publisher claim, and attestation URLs

#### Security Considerations

**Resource Attestation URL Constraints**: The `data-la-resource-attestation-url` provides complete flexibility in organizing attestation files, but MUST point to a URL served from the same origin as the resource URL. This ensures the Resource Attestation comes from the same authority that controls the resource.

**Examples of valid RA placement**:

-   Resource: `https://example.com/alice/posts/123` → RA: `https://example.com/alice/posts/123/_la_resource.json` ✅
-   Resource: `https://example.com/alice/posts/123` → RA: `https://example.com/alice/attestations/posts-123.json` ✅
-   Resource: `https://example.com/alice/posts/123` → RA: `https://other.com/attestations/123.json` ❌ (different origin)

**Preview vs Canonical Content**: The preview section contains human-readable content but is NOT cryptographically verified. Malicious clients can modify preview content while verification continues to pass. The canonical content bytes in the `<link>` element represent the authoritative, verified content. Conforming clients SHOULD replace the preview content with the decoded canonical bytes when rendering.

## Resource Attestation (RA)

An unsigned JSON document that serves as the bridge between a specific resource and its publisher. The Resource Attestation contains a content hash for integrity verification and points to a Namespace Attestation that establishes publisher authority. Its presence at the expected endpoint demonstrates intent to distribute, while its removal enables clean dissociation.

```json
{
    "url": "https://example.com/people/alice/posts/123",
    "hash": "sha256:7b0c...cafe",
    "namespace_attestation_url": "https://example.com/people/alice/_la_namespace.json"
}
```

## Namespace Attestation (NA)

A cryptographically signed declaration of publisher authority over a namespace and all its subdirectories. The Namespace Attestation serves as durable evidence of a publisher's claim to control a web namespace, but only demonstrates active control when both present on the server and not expired. It is a public announcement with no expectation of privacy, containing an expiration timestamp set by the publisher to limit the claim's validity period.

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

## Verification Flow

LAP verification establishes publisher-resource association through three sequential checks:

### 1. Resource Presence

Confirms the Resource Attestation is accessible at the expected endpoint, demonstrating intent to distribute. Validates same-origin serving and URL matching.

### 2. Resource Integrity

Verifies that the content bytes match what was attested by comparing SHA-256 hashes.

### 3. Publisher Association

Proves publisher control over the namespace containing the resource through a valid Namespace Attestation.

**Verification succeeds** only when all three checks pass, providing proof of publisher-resource association.

**Verification fails** at the first failed check, with remaining checks skipped for efficiency.

For detailed verification procedures, check definitions, and failure codes, see [verification-spec.md](verification-spec.md).
