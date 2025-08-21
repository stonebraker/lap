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
    data-lap-url="https://example.com/people/alice/messages/123"
    data-lap-attestation-url="https://example.com/people/alice/messages/123/_lap/resource_attestation"
>
    <!-- Stapled attestation (exact JSON from attestation endpoint) -->
    <script type="application/lap+json" class="lap-attestation">
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
    <div class="preview">Hello, my name is Alice.</div>

    <!-- Verification UI -->
    <button class="lap-verify-btn" type="button">Verify LAP</button>
    <span
        class="lap-verify-result"
        aria-live="polite"
        style="margin-left:.5rem;"
    ></span>
</article>
```

## Verification Logic

The verification process for script-tag embedded attestations:

```javascript
(function () {
    // Helper functions
    const isHex = (s) => typeof s === "string" && /^[0-9a-f]+$/i.test(s);
    const nowSecs = () => Math.floor(Date.now() / 1000);

    // Parse stapled attestation from script tag
    function parseStapled(articleEl) {
        const tag = articleEl.querySelector(
            'script.lap-attestation[type="application/lap+json"]'
        );
        if (!tag) throw new Error("missing stapled attestation");
        return JSON.parse(tag.textContent.trim());
    }

    // Fetch live attestation from endpoint
    async function fetchRA(url) {
        const res = await fetch(url, {
            headers: { Accept: "application/lap+json, application/json;q=0.8" },
            cache: "no-store",
            credentials: "omit",
        });
        if (!res.ok) throw new Error(`fetch failed: ${res.status}`);
        return res.json();
    }

    // Validate attestation structure and freshness
    function checkStapledShape(stapled, articleEl) {
        if (!stapled || typeof stapled !== "object")
            return "malformed stapled object";
        const p = stapled.payload || {};

        // Required fields
        if (!p.url) return "missing payload.url";
        if (!p.attestation_url) return "missing payload.attestation_url";
        if (!p.hash) return "missing payload.hash";
        if (!stapled.sig) return "missing sig";
        if (!stapled.resource_key) return "missing resource_key";
        if (typeof p.iat !== "number" || typeof p.exp !== "number")
            return "missing iat/exp";
        if (!p.kid) return "missing kid";

        // URL validation
        try {
            const u = new URL(p.url);
            if (u.protocol !== "https:") return "payload.url must be https";
            const au = new URL(p.attestation_url);
            if (au.protocol !== "https:")
                return "attestation_url must be https";

            // Canonical path structure
            if (!au.pathname.endsWith("/_lap/resource_attestation")) {
                return "attestation_url path is not canonical";
            }

            // Same-origin and path hierarchy
            if (u.origin !== au.origin)
                return "attestation_url origin mismatch";
            if (!au.pathname.startsWith(u.pathname)) {
                return "attestation_url not under resource path";
            }
        } catch (_) {
            return "invalid url in payload";
        }

        // Format validation
        if (!/^sha256:[0-9a-f]{64}$/i.test(p.hash))
            return "hash must be sha256:<64-hex>";
        if (!isHex(stapled.sig.replace(/^0x/i, ""))) return "sig must be hex";
        if (
            !isHex(stapled.resource_key) ||
            stapled.resource_key.replace(/^0x/i, "").length < 64
        ) {
            return "resource_key must be hex (x-only pubkey)";
        }

        // Freshness check
        const now = nowSecs();
        if (now < p.iat) return "attestation not yet valid (iat in future)";
        if (now > p.exp) return "attestation expired";

        // Optional: verify declared URL matches payload
        const declared = articleEl?.dataset?.lapUrl;
        if (declared && declared !== p.url)
            return "data-lap-url does not match payload.url";

        return null; // validation passed
    }

    // Compare stapled vs live attestation for drift
    function compareStapledVsFetched(stapled, fetched) {
        const ps = stapled.payload || {};
        const pf = (fetched && fetched.payload) || {};

        // Fields that must not drift between stapled and live
        if (pf.url !== ps.url) return "payload.url mismatch (live vs stapled)";
        if (pf.attestation_url !== ps.attestation_url)
            return "attestation_url mismatch (live vs stapled)";
        if ((pf.hash || "") !== (ps.hash || ""))
            return "hash mismatch (live vs stapled)";
        if ((fetched.sig || "") !== (stapled.sig || ""))
            return "signature mismatch (live vs stapled)";
        if ((fetched.resource_key || "") !== (stapled.resource_key || ""))
            return "resource_key mismatch (live vs stapled)";
        if ((pf.kid || "") !== (ps.kid || ""))
            return "kid mismatch (live vs stapled)";

        // Live attestation freshness
        const now = nowSecs();
        if (typeof pf.iat === "number" && now < pf.iat)
            return "live attestation not yet valid";
        if (typeof pf.exp === "number" && now > pf.exp)
            return "live attestation expired";

        return null; // no drift detected
    }

    // Event handler for verification
    document.addEventListener("click", async (e) => {
        const btn = e.target.closest(".lap-verify-btn");
        if (!btn) return;

        const article = btn.closest("article");
        const out = article.querySelector(".lap-verify-result");

        const show = (ok, msg) => {
            out.textContent = ok ? "✔" : "✖";
            out.style.color = ok ? "green" : "crimson";
            out.title = msg;
        };

        try {
            out.textContent = "…";
            out.removeAttribute("style");
            out.title = "verifying…";

            // 1. Parse stapled attestation
            const stapled = parseStapled(article);

            // 2. Validate stapled attestation
            const shapeErr = checkStapledShape(stapled, article);
            if (shapeErr) return show(false, shapeErr);

            // 3. Fetch live attestation
            const raUrl = stapled.payload.attestation_url;
            const live = await fetchRA(raUrl);

            // 4. Compare stapled vs live
            const driftErr = compareStapledVsFetched(stapled, live);
            if (driftErr) return show(false, driftErr);

            // Success
            show(true, "verified live");
        } catch (err) {
            show(
                false,
                err && err.message ? err.message : "verification error"
            );
        }
    });
})();
```

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

This approach provides a straightforward way to embed LAP attestations while maintaining clean separation between content and verification data.
