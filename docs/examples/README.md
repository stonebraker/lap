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

# LAP Implementation Examples

This directory contains practical examples for implementing LAP (Linked Attestations Protocol) fragments and verification.

## Fragment Examples

### HTML Data Attributes Approach

**File**: [`fragment-html-attributes.md`](fragment-html-attributes.md)

Uses HTML `data-*` attributes to embed attestation information directly in the fragment structure. This approach:

-   Distributes attestation data across multiple data attributes
-   Provides clean HTML-native integration
-   Allows granular access to individual attestation fields
-   Requires no JSON parsing for basic verification

### Script Tag JSON Approach

**File**: [`fragment-script-json.md`](fragment-script-json.md)

Embeds the complete attestation as JSON within a `<script type="application/lap+json">` tag. This approach:

-   Uses clean JSON structure matching the attestation endpoint format
-   Provides standard MIME type for proper content identification
-   Requires minimal HTML markup
-   Enables direct mapping between stapled and live attestations

## Choosing an Approach

### Use HTML Data Attributes When:

-   You want fine-grained control over individual attestation fields
-   Your application benefits from accessing attestation data via DOM attributes
-   You prefer avoiding JSON parsing in your verification code
-   You need maximum compatibility with HTML processing tools

### Use Script Tag JSON When:

-   You want exact format matching with attestation endpoints
-   Your verification code already handles JSON parsing
-   You prefer cleaner HTML with minimal markup
-   You need to preserve complex attestation structures exactly

## Implementation Notes

Both approaches:

-   Support the same verification workflow and security properties
-   Use identical validation and drift detection logic
-   Provide the same level of cryptographic integrity
-   Are compatible with the LAP protocol specification

Choose based on your application's specific needs and development preferences. The security and verification guarantees remain identical regardless of the embedding approach used.

## Reference Implementation

See the working implementation in the LAP reference codebase:

-   JavaScript verifier: `apps/server/static/publisherapi/js/la/verifier.core.js`
-   Go CLI verifier: `apps/verifier-cli/cmd/verifier/`
-   Test suite: `apps/server/static/publisherapi/tests/core/`
