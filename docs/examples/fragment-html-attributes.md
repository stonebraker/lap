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

# LAP Fragment Example: HTML Data Attributes

This example demonstrates a LAP fragment using HTML `data-*` attributes to embed attestation information.

## Fragment Structure

LAP fragments use an `<article>` element with specific data attributes to declare compliance and structure.

### Required Attributes

The root `<article>` **MUST** include:

-   `data-lap-spec` — identifier for the spec/version (URL or token)
-   `data-lap-profile="fragment"` — declares this node is a LAP fragment
-   `data-lap-attestation-format` — how the stapled attestation is represented
    -   `div` (nested `div`/`data-lap-*` attributes)
    -   `script` (alternative: `<script type="application/lap+json">`)
-   `data-lap-bytes-format` — how canonical bytes are provided
    -   `link-data` (a `<link rel="alternate" href="data:...">`)

The `<article>` **MUST** also expose selectors to key parts:

-   `data-lap-preview` → preview node selector
-   `data-lap-attestation` → stapled attestation node selector
-   `data-lap-bytes` → canonical bytes node selector

## Complete Example

```html
<article
    id="lap-article-msg-123"
    data-lap-spec="https://lap.dev/spec/v0-1"
    data-lap-profile="fragment"
    data-lap-attestation-format="div"
    data-lap-bytes-format="link-data"
    data-lap-url="https://example.com/people/alice/messages/123"
    data-lap-preview="#lap-preview-msg-123"
    data-lap-attestation="#lap-attestation-msg-123"
    data-lap-bytes="#lap-bytes-msg-123"
>
    <!-- Human-readable preview (not used for verification) -->
    <div id="lap-preview-msg-123" class="lap-preview">
        Hello, my name is Alice.
    </div>

    <!-- Canonical bytes as data: URL -->
    <link
        id="lap-bytes-msg-123"
        rel="alternate"
        type="text/html; charset=utf-8"
        class="lap-bytes"
        data-hash="sha256:7b0c0d2f3a4b5c6d7e8f90112233445566778899aabbccddeeff001122334455"
        href="data:text/html;base64,PHNwYW4gaWQ9Im1zZy0xMjMiPkhlbGxvLCBteSBuYW1lIGlzIEFsaWNlLjwvc3Bhbj4="
    />

    <!-- Stapled attestation using data attributes -->
    <div
        id="lap-attestation-msg-123"
        class="lap-attestation"
        data-lap-resource-key="aa11bb22cc33dd44ee55ff6600112233445566778899aabbccddeeff00112233"
        data-lap-sig="bd7a51fe428ea29a5dac521db4931a0e5f86ae38b634321b50ab34b82bf9f207"
    >
        <div
            class="lap-payload"
            data-lap-url="https://example.com/people/alice/messages/123"
            data-lap-attestation-url="https://example.com/people/alice/messages/123/_lap/resource_attestation.json"
            data-lap-hash="sha256:7b0c0d2f3a4b5c6d7e8f90112233445566778899aabbccddeeff001122334455"
            data-lap-etag='W/"123-abcde"'
            data-lap-iat="1754908800"
            data-lap-exp="1754909400"
            data-lap-kid="resource-key-2025-08-12"
        ></div>
    </div>

    <!-- Optional: Verification UI -->
    <div class="lap-verify-ui">
        <button class="lap-verify-btn">Verify LAP</button>
        <span class="lap-verify-result"></span>
    </div>
</article>
```

## Verification Processing

Verifiers should:

1. Check `data-lap-spec` is a recognized version
2. Route to a parser by `data-lap-attestation-format` + `data-lap-bytes-format`
3. Resolve the three selectors; fail if any required node is missing
4. Extract attestation data from the appropriate format
5. Perform standard LAP verification against the live attestation endpoint

## Implementation Notes

-   The `data-hash` on the bytes link should match `payload.hash` for consistency
-   Attestation data is distributed across `data-lap-*` attributes for easy parsing
-   The preview content is for human readability - verification uses the canonical bytes
-   All URLs must be HTTPS and same-origin between resource and attestation endpoints

This format provides a clean HTML-native way to embed LAP attestations without requiring JSON parsing.
