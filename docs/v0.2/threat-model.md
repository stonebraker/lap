# LAP Protocol Threat Model v0.2

This document analyzes potential security threats to the LAP (Linked Attestation Protocol) and evaluates how the protocol mitigates or fails to address each threat.

## Overview

LAP provides reasonable proof of publisher-resource association through cryptographic attestations and live verification. This threat model examines attacks against the protocol's three verification goals:

1. **Resource Presence** - Resource Attestation is accessible, demonstrating intent to distribute
2. **Resource Integrity** - Content bytes match what was attested
3. **Publisher Association** - Publisher controls the namespace

## Core Verification Goals Analysis

LAP's primary purpose is to establish publisher-resource association through three verification checks. This section analyzes how well LAP achieves its stated goals.

### 1. Resource Presence Verification

**Goal**: Verify the Resource Attestation is accessible, demonstrating intent to distribute.

#### ✅ **SUCCESS: Live Verification**

-   **T8: Replay Attack Prevention**: Live verification requires attestations to be accessible at time of verification, preventing reuse of old attestations after dissociation
-   **T10: Namespace Authority**: Namespace Attestation must be served from the claimed namespace, proving server control

#### ❌ **LIMITATIONS: Infrastructure Dependencies**

-   **T5: DNS Hijacking**: Protocol relies on DNS integrity for origin verification
-   **T7: Man-in-the-Middle**: Depends on HTTPS for secure transport during attestation fetching

### 2. Resource Integrity Verification

**Goal**: Verify content bytes match what was attested.

#### ✅ **SUCCESS: Cryptographic Protection**

-   **T1: Content Tampering Prevention**: SHA-256 hash verification detects any modification to canonical content bytes
-   **T15: Hash Collision Resistance**: SHA-256 provides strong protection against collision attacks

#### ❌ **LIMITATIONS: Client Responsibility**

-   **T2: Preview Content Spoofing**: Preview content is explicitly not verified (acknowledged design trade-off)

### 3. Publisher Association Verification

**Goal**: Verify the publisher controls the namespace containing the resource.

#### ✅ **SUCCESS: Cryptographic Authentication**

-   **T3: Publisher Authentication**: Schnorr signatures provide strong cryptographic proof of publisher identity
-   **T6: Cross-Origin Protection**: Same-origin requirements prevent attestation injection attacks
-   **T16: Signature Security**: BIP-340 Schnorr signatures are cryptographically secure

#### ✅ **SUCCESS: Temporal Protection**

-   **T11: Subdomain Takeover Mitigation**: NA expiration timestamps limit the window of vulnerability - even if an attacker gains subdomain control, they cannot create new valid attestations without the publisher's private key, and existing attestations expire

#### ❌ **LIMITATIONS: Operational Security**

-   **T4: Key Compromise**: Protocol cannot prevent private key theft (operational security issue)

**Summary**: LAP successfully achieves its three core verification goals through strong cryptographic methods and live verification, with limitations primarily in areas outside the protocol's scope (infrastructure security and operational practices).

---

## Extended Threat Analysis

The following sections analyze additional threats that help implementers understand what LAP does and does not protect against.

### Domain and Infrastructure Threats

#### T11: Subdomain Takeover

**Description**: Attacker gains control of subdomain and attempts to create malicious attestations.

**Attack Vector**: Taking over abandoned subdomains to serve fake attestations for resources under that namespace.

**LAP Mitigation**: ✅ **PARTIALLY MITIGATED**

-   **Expiration Protection**: Namespace Attestations have `exp` timestamps that limit the validity window
-   **Key Requirement**: Attacker needs both subdomain control AND the publisher's private key to create valid new attestations
-   **Time-Limited Risk**: Even with subdomain control, existing attestations expire and cannot be renewed without the private key

**Remaining Risk**: If subdomain takeover occurs before NA expiration and the attacker also compromises the private key

**Status**: ✅ Significantly mitigated by temporal controls

### Implementation and Client Threats

#### T12: Malicious Verifier

**Description**: Compromised or malicious verification library returns false results.

**Attack Vector**: Using tampered verification code that always returns "verified: true".

**LAP Mitigation**: ❌ **NOT MITIGATED**

-   Protocol cannot prevent malicious verifier implementations
-   Relies on client choosing trustworthy verification libraries
-   No mechanism to verify verifier integrity

**Status**: ❌ Fail - Client implementation responsibility

#### T13: Verification Bypass

**Description**: Client skips verification entirely and trusts content without checking.

**Attack Vector**: Malicious or lazy client implementations that don't perform verification.

**LAP Mitigation**: ❌ **NOT MITIGATED**

-   Protocol cannot force clients to verify
-   Relies on client conformance and good practices
-   Users must trust their client implementations

**Status**: ❌ Fail - Client implementation responsibility

#### T14: XSS via Canonical Content

**Description**: Malicious canonical content contains JavaScript that executes when injected into DOM.

**Attack Vector**: Publisher includes `<script>` tags or event handlers in canonical content.

**LAP Mitigation**: ✅ **PARTIALLY MITIGATED**

-   Protocol requires clients to sanitize canonical content before DOM injection
-   Verification ensures content authenticity but not safety
-   Client must implement proper XSS prevention

**Status**: ⚠️ Partial - Requires client-side sanitization

### Additional Infrastructure Threats

#### T9: Clock Manipulation

**Description**: Attacker manipulates system clocks to bypass expiration checks.

**Attack Vector**: Setting system time backwards to make expired attestations appear valid.

**LAP Mitigation**: ❌ **NOT MITIGATED**

-   Protocol relies on accurate system clocks
-   No protection against clock manipulation
-   Verification depends on local time validation

**Status**: ❌ Fail - Relies on system clock integrity

## Threat Summary

### Core Verification Goals

| Verification Goal     | Success | Limitations |
| --------------------- | ------- | ----------- |
| Resource Presence     | 2       | 2           |
| Resource Integrity    | 2       | 1           |
| Publisher Association | 4       | 1           |

**Core Goals Total**: 8 Successes ✅ | 4 Limitations ❌

### Extended Threat Categories

| Category                  | Mitigated | Partial | Not Mitigated |
| ------------------------- | --------- | ------- | ------------- |
| Domain and Infrastructure | 1         | 0       | 0             |
| Implementation and Client | 0         | 1       | 2             |
| Additional Infrastructure | 0         | 0       | 1             |

**Extended Total**: 1 Mitigated ✅ | 1 Partial ⚠️ | 3 Not Mitigated ❌

**Overall Assessment**: LAP successfully achieves its three core verification goals with strong cryptographic protection, while acknowledging limitations in areas outside the protocol's scope.

## Security Recommendations

### For Publishers

1. **Secure Key Management**: Use hardware security modules or secure key storage
2. **Regular Key Rotation**: Implement key rotation strategies with appropriate expiration times
3. **Domain Security**: Maintain control over domains and subdomains
4. **Monitor Attestations**: Regularly verify your attestations are being served correctly

### For Clients

1. **Verify Always**: Always perform verification before rendering content
2. **Sanitize Content**: Implement robust XSS prevention when injecting canonical content
3. **Trusted Verifiers**: Use well-audited verification libraries
4. **Secure Transport**: Always use HTTPS for attestation fetching
5. **Replace Preview**: Replace preview content with verified canonical content

### For Verifiers

1. **Fail Securely**: Implement fail-fast behavior and clear error reporting
2. **Validate Inputs**: Thoroughly validate all inputs and network responses
3. **Secure Networking**: Use secure HTTP clients with proper timeout handling
4. **Cache Safely**: Implement secure caching with appropriate TTLs

## Conclusion

**LAP Successfully Achieves Its Core Goals**: The protocol provides strong cryptographic protection for its three verification objectives - Resource Presence, Resource Integrity, and Publisher Association. Through SHA-256 hashing, Schnorr signatures, and live verification, LAP delivers reliable proof of publisher-resource association.

**Key Strengths**:

-   Strong cryptographic foundation (SHA-256, BIP-340 Schnorr signatures)
-   Live verification prevents replay attacks and enables clean dissociation
-   Same-origin enforcement prevents cross-origin attestation injection
-   Namespace validation ensures proper publisher authority

**Acknowledged Limitations**:

1. **Infrastructure Dependencies** - Relies on DNS, HTTPS, and system clock security (by design)
2. **Preview Content** - Explicitly not verified (usability trade-off)
3. **Client Implementation** - Cannot force proper verification or sanitization (trust boundary)

These limitations are largely intentional design decisions. LAP focuses on providing reasonable proof of publisher-resource association through cryptographic means, while relying on established web infrastructure and client conformance for broader security concerns.
