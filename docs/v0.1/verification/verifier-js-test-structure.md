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

## Tests

### Directory structure

```
repo/
├─ apps/
│ ├─ server/
│ │ └─ cmd/publisherapi/main.go
│ │ └─ static/ # everything below is just files the Go server
│ │   └─ publisherapi/
│ │     ├─ people/alice/index.html
│ │     ├─ js/
│ │     │ ├─ la/
│ │     │ │ ├─ verifier.core.js # normative: pure ESM, no side-effects, no DOM
│ │     │ │ └─ verifier.ui.js # non‑normative: imports core, adds DOM helpers/badges
│ │     │ └─ vendor/ # (optional) pinned single‑file crypto vendored locally
│ │     │   └─ noble-secp256k1.js
│ │     └─ tests/
│ │       └─ core/
│ │         ├─ vectors.json # golden vectors: payloads, sigs, pass/fail cases
│ │         ├─ fixtures/
│ │         │ ├─ fragment.htmx # sample fragments (exact bytes matter)
│ │         └─ core.test.html # opens in a browser, runs tests w/out deps
```

**Why this works**

-   **No build step:** Everything is plain ESM. Your HTML uses `<script type="module">` to import `verifier.ui.js`.
-   **Normative vs non‑normative:** `verifier.core.js` exposes a tiny, stable API (verification logic + result codes). `verifier.ui.js` is just convenient sugar for the demo site.
-   **Minimal deps:** The only “heavy” bit is Schnorr(secp256k1). If you don’t want a package manager, **vendor one single ESM file** (e.g., a pinned `@noble/secp256k1` export) into `js/vendor/`. Everything else can be Web Crypto + small helpers.
-   **Static tests:** Open the `.test.html` files directly in a browser (or have your Go server expose `/tests/`). No bundlers, no runners required.

Note: The core test page (`tests/core/core.test.html`) runs what are best thought of as lightweight integration tests for the core module. Each case exercises `verifier.core.js` end-to-end with:

-   deterministic signature signing/verification (a tiny in-page Schnorr stub) to validate the signature gate (`LA_SIG_INVALID` vs success),
-   mocked RA fetch and clock,
-   JSON vectors that model policy+code outcomes.

This keeps tests fast and self-contained while still demonstrating how `verifier.core.js` uses Schnorr signatures and policies with realistic inputs. If you want to also run against the real `@noble/secp256k1` in a separate page, swap the stub for the vendor import and reuse the same vectors.

---
