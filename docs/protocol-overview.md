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

# Linked Attestations Protocol

## Description

Linked Attestations Protocol delivers portable, time-boxed proof of content origin and publisher control—verifiable live at fetch and later at rest—without gatekeepers and with clean dissociation when needed. LAP focuses on **addressable HTML fragments**—small, linkable pieces of hypermedia content—so you can verify exactly who published what you embed, making syndication safer, caching and mirroring more trustworthy, and keeping content portable across the open web without platform lock-in.

## Purpose

A tiny, HTTP-friendly way to provide **Live Verification** that:

1. **Namespace Attestation** — proves a publisher controls a namespace.  
   _Example:_ Serve a signed document at `https://example.com/people/alice/_la_namespace.json` asserting control of `https://example.com/people/alice/` during a specified time window.

2. **Resource Attestation** — proves specific bytes originate from a URL.  
   _Example:_ For `https://example.com/people/alice/messages/123`, publish `…/messages/123/_lap/resource_attestation` signed by a server generated and controlled, **per-resource key** over `{ url, attestation_url, hash, etag, iat, exp }` for the fragment’s exact bytes.

3. **Live Verification (Resource Attestation + Namespace Attestation) = Proof of Publisher** — links content to the publisher **right now**.  
   _Example:_ Verify the resource attestation for `…/messages/123`, then verify the namespace attestation from `…/people/alice/_la_namespace.json`; because the URL sits under Alice’s namespace, the fragment can be reasonably associated with Alice at fetch time.

4. **Dissociation** — removing either attestation severs the live link.  
   _Example:_  
   • Remove the resource attestation → you can’t prove the bytes at `…/messages/123`; namespace control alone doesn’t bind content.  
   • Remove the namespace attestation → you can still prove bytes↔URL, but not publisher↔content.  
   • Remove both → no enduring cryptographic association remains anywhere.

## Examples

### Concise Example

#### `/_la_namespace.json`

```json
{
    "payload": {
        "namespace": ["https://example.com/people/alice/"],
        "attestation_url": "https://example.com/people/alice/_la_namespace.json",
        "iat": 1754908800, // epoch seconds (UTC)
        "exp": 1754909400, // expiration timestamp (epoch seconds UTC)
        "kid": "publisher-key-2025-08-12" // identifier for publisher key
    },
    "publisher_key": "f1a2d3c4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00",
    "sig": "4e0f...<128-hex>...9c2a" // 128 hex chars — Ed25519(sig over SHA256(payload_json))
}
```

#### `/_lap/resource_attestation.json` (for `https://example.com/people/alice/messages/123`)

```json
{
    "payload": {
        "url": "https://example.com/people/alice/messages/123",
        "attestation_url": "http://localhost:8081/people/alice/posts/1/_lap/resource_attestation.json",
        "hash": "sha256:7b0c...cafe", // SHA-256 of message bytes from 4.2
        "iat": 1754908800, // epoch seconds (UTC)
        "exp": 1754909100, // expiration timestamp (epoch seconds UTC)
        "kid": "msg-123-key" // identifier for per-resource key
    },
    "sig": "deadbeef..." // 128 hex chars — Ed25519(sig over SHA256(payload_json))
}
```

#### Self-contained HTML fragment (at `…/messages/123`)

```html
<article
    id="lap-article-123"
    data-lap-spec="https://lap.dev/spec/v0-1"
    data-lap-profile="fragment"
    data-lap-attestation-format="div"
    data-lap-bytes-format="link-data"
    data-lap-url="http://localhost:8081/people/alice/posts/123"
    data-lap-preview="#lap-preview-123"
    data-lap-attestation="#lap-attestation-123"
    data-lap-bytes="#lap-bytes-123"
>
    <div id="lap-preview-123" class="lap-preview">
        <article>
            <header>
                <h2>Post 1</h2>
            </header>
            <p>
                Kicking off a new project. Keeping things simple, minimal deps,
                and lots of clarity.
            </p>
        </article>
    </div>

    <link
        id="lap-bytes-123"
        rel="alternate"
        type="text/html; charset=utf-8"
        class="lap-bytes"
        data-hash="sha256:7b0c...cafe"
        href="data:text/html;base64,base64-encoded-resource-content-here"
    />

    <div
        id="lap-attestation-123"
        class="lap-attestation"
        data-lap-resource-key="secp256k1-public-key-here"
        data-lap-sig="schnorr-signature-here"
        hidden
    >
        <div
            class="lap-payload"
            data-lap-url="https://example.com/people/alice/posts/123"
            data-lap-attestation-url="https://example.com/people/alice/posts/123/_lap/resource_attestation.json"
            data-lap-hash="sha256:7b0c...cafe"
            data-lap-etag='W/\"7b0c...cafe\"'
            data-lap-iat="1754908800"
            data-lap-exp="1754909100"
            data-lap-kid="resource-key-identifier"
        ></div>
    </div>
</article>
```

## How it works

1. **Resource authenticity:** The resource’s exact bytes are signed by a **resource key** exposed under the resource’s own path (a **resource attestation**). The attestation is included alongside the resource (an HTML fragment) when served.
2. **Publisher control:** The publisher’s control of the namespace is proven by a **namespace attestation** served from within that namespace. The signed attestation asserts control. The presence of the attestation at the namespace demonstrates it. Assertion plus demonstration provides proof of the publisher's control of the namespace, and their desire to make it known.
3. **Live-linked proof:** When a verifier checks both, it gets a **highly plausible proof** that this publisher is responsible for this content **right now**. A process known as **live verification**.
4. **Safely dissociating from claims:** If links are removed, the proof intentionally falls apart in predictable ways:

    - **Remove only the resource attestation:** You can no longer prove resource authenticity - that a piece of content originated from a specific namespace. Assuming a resource attestation and the resource itself were archived, the signed attestation would be merely an assertion signed by an ephemeral key pair. Such an assertion could just as easily be signed by any arbitrary key pair. Importantly, there is **no durable evidence** tying the publisher to the content once the resource attestation is removed, since it can no longer be demonstrated that the content originated from the publisher's namespace.
    - **Remove only the namespace attestation:** You can still prove resource authenticity with the resource attestation, but you cannot link the resource's bytes to the publisher. There is **no enduring publisher-to-content proof**. Assuming a namespace attestation has been archived, but is not currently present on the namespace, it merely asserts control (which anybody could do), it does not demonstrate control.
    - **Remove both:** No live verification remains. Any archived artifacts do **not** cryptographically bind the publisher to the resource bytes.

5. **Time-boxed claims:** All attestations carry **issued-at** and **expires** timestamps; verifiers must require the current time to be within that window to accept a live proof.

---

### The Value of Ephemeral Publisher <-> Content Associations

Ephemeral publisher-to-content associations let you **link when you want and unlink when you don’t**. By checking a resource attestation (bytes ↔ URL) and a namespace attestation (publisher ↔ namespace) **live** within their issued-at/expiring window, verifiers get a strong, time-boxed signal: “this publisher is responsible for these bytes **right now**.” When either attestation is later removed, that live link dissolves—there’s no enduring, cryptographic chain from publisher to those exact bytes—so publishers can correct mistakes, retract posts, or hand off namespaces without dragging a permanent record behind them. This reduces replay and misattribution risk, limits long-term liability, and keeps privacy and operational agility intact, while still delivering solid proof at the moment of consumption.

The association is baked into the Linked Attestations protocol itself—not handed over to a third-party gatekeeper—so proof travels with the content. Wherever the bytes show up on the web — your site, a mirror, an embed — the publisher-to-content link can be proven there, not just inside a platform’s walled garden.

While publisher-to-content association is valuable, dissociation can be equally valuable.

The ability for a publisher to remove content from their platform and dissociate from it is a necessary feature of any system wishing to support expressiveness for a broad range of participants whose use cases are innumerable, unique, and entirely subjective. From the unfortunate post Bob made from his company's social media account: "Our customers are our greatest asses." That was supposed to be "assets", Bob. To the influencer whose online presence was built representing Brand A, but is now representing Brand B. To the college kid who had a rough night and might have posted some regrettable remarks at 3am. These are just a few situations where someone who published something on the internet might appreciate being able to easily dissociate from it.

On the other end of the spectrum from signed content is unsigned content. This is the kind of content that resides on proprietary websites that typically use a username and password to establish identity. In this situation, an intermediary is trusted by participants to faithfully associate a piece of content with a creator. When you see a post from a friend on a platform like Facebook, you believe it was your friend who actually posted it, because you trust the intermediary. If that post was just floating around on the internet unsigned, you'd have no way of verifying that it was actually posted by your friend. I suspect this lack of verifiability could be one of those hard-to-see architectural shortcomings of the current web that silently drives vendor lock-in.

In order to enable free-range content, Linked Attestations seek to live in the space between the two extremes of unsigned content and signed content. A linked attestation provides very high plausibility of Publisher <-> Content association while it exists, and very low plausibility of association when the link is removed. Each on their own, Resource Attestations and Namespace Attestations are useful for myriad purposes. When combined, it is possible to provide a retractable proof-of-publisher for portable content, potentially enabling _the web as social media platform_, among other use cases.

| Unsigned Content | Unlinked Attestation  | <---------------> | Linked Attestation      | Signed Content    |
| :--------------- | :-------------------- | :---------------: | :---------------------- | :---------------- |
| (Zero Assurance) | (Very Weak Assurance) |                   | (Very Strong Assurance) | (Total Assurance) |

---

## Appendix: Vision and Purpose

_Author's notes_

### Re-centering on Purpose

I've designed the Linked Attestations Protocol with real-world use and adoption in mind, utilizing hypermedia technologies like html and http that are simple, familiar, and low-key powerful, with hopes that when micro-content is http addressable, restful, embeddable, transportable, and live-verifiable, a host of new forms of expressivity and interactivity might emerge across the web.

— Jason Stonebraker
