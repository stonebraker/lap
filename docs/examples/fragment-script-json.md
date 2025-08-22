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

# LAP Fragment Example: Script Tag JSON

This example demonstrates a LAP fragment using a `<script>` tag with `type="application/lap+json"` to embed attestation information as JSON.

## Overview

This approach embeds the complete attestation object as JSON within a script tag, making it easy to parse while keeping the attestation data separate from the HTML structure.

## Complete Example

```html
<!--
  Client fetches fragment from: https://example.com/people/alice/messages/123
  and embeds it in its own document. This happens client side via Javascript,
  presumably using a library like HTMX, though the specific library isn't fundamental
  to this example.
-->
<article
    id="msg-123"
    data-lap-spec="https://lap.dev/spec/v0-1"
    data-lap-profile="fragment"
    data-lap-attestation-format="script"
    data-lap-bytes-format="link-data"
    data-lap-url="https://example.com/people/alice/messages/123"
    data-lap-preview="#msg-123-preview"
    data-lap-attestation="#msg-123-attestation"
    data-lap-bytes="#msg-123-bytes"
>
    <!-- Stapled attestation (exact JSON from attestation endpoint) -->
    <script
        id="msg-123-attestation"
        type="application/lap+json"
        class="lap-attestation"
    >
        {
            "payload": {
                "url": "https://example.com/people/alice/messages/123",
                "attestation_url": "https://example.com/people/alice/messages/123/_lap/resource_attestation",
                "hash": "sha256:7b0c0d2f3a4b5c6d7e8f90112233445566778899aabbccddeeff001122334455",
                "etag": "W/\"123-abcde\"",
                "iat": 1754908800,
                "exp": 1754909400,
                "kid": "resource-key-2025-08-12"
            },
            "resource_key": "aa11bb22cc33dd44ee55ff6600112233445566778899aabbccddeeff00112233",
            "sig": "bd7a51fe428ea29a5dac521db4931a0e5f86ae38b634321b50ab34b82bf9f207"
        }
    </script>

    <!-- Human-readable content preview -->
    <div id="msg-123-preview" class="preview">Hello, my name is Alice.</div>

    <!-- Canonical bytes as data: URL -->
    <link
        id="msg-123-bytes"
        rel="alternate"
        type="text/html; charset=utf-8"
        class="lap-bytes"
        data-hash="sha256:7b0c0d2f3a4b5c6d7e8f90112233445566778899aabbccddeeff001122334455"
        href="data:text/html;base64,PHNwYW4gaWQ9Im1zZy0xMjMiPkhlbGxvLCBteSBuYW1lIGlzIEFsaWNlLjwvc3Bhbj4="
    />

    <!-- Verification UI -->
    <button class="lap-verify-btn" type="button">Verify LAP</button>
    <span
        class="lap-verify-result"
        aria-live="polite"
        style="margin-left:.5rem;"
    ></span>
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
-   Attestation data is embedded as JSON in the script tag for easy parsing
-   The preview content is for human readability - verification uses the canonical bytes
-   All URLs must be HTTPS and same-origin between resource and attestation endpoints

This format provides a clean JSON-based way to embed LAP attestations while maintaining HTML structure.

## Advantages

-   **Clean JSON structure**: Easy to parse and validate
-   **Standard MIME type**: Uses `application/lap+json` for proper content type
-   **Minimal HTML**: No complex data attribute structure required
-   **Direct mapping**: JSON structure matches attestation endpoint format exactly

## Processing Steps

1. **Parse**: Extract JSON from `<script type="application/lap+json" class="lap-attestation">`
2. **Validate**: Check attestation structure, URLs, formats, and freshness
3. **Fetch**: Retrieve live attestation from the signed `payload.attestation_url`
4. **Compare**: Verify no drift between stapled and live attestations
5. **Result**: Display verification success/failure with specific error details
