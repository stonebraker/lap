# LAP Protocol v0.2 Specification

> 🚧 **PREVIEW SPECIFICATION - UNSTABLE** 🚧
>
> This is a **preview** of LAP v0.2 under active development. The specification is **incomplete** and **subject to breaking changes** without notice.
>
> **⚠️ DO NOT IMPLEMENT IN PRODUCTION ⚠️**
>
> This specification is provided for:
>
> -   Early feedback and review
> -   Protocol design validation
> -   Reference implementation planning
>
> **Stability**: None guaranteed until v0.2.0 release  
> **Breaking Changes**: Expected frequently  
> **Implementation Status**: Reference implementations not yet updated

## Major Changes from v0.1

-   **Fragment Structure**: Simplified with `data-la-fragment-url` and consolidated verification metadata
-   **Cryptography**: Migration from Ed25519 to Schnorr signatures (BIP-340) with secp256k1
-   **Verification**: Streamlined four-check model (Integrity, Origination, Freshness, Association)
-   **Security**: Enhanced threat model and mitigation analysis

## Specification Documents

-   **[Protocol Overview](overview.md)** - Core concepts and basic use case
-   **[Artifacts Specification](artifacts.md)** - Canonical schemas for all LAP artifacts
-   **[Roles Specification](roles-spec.md)** - Server, Client, and Verifier responsibilities
-   **[Verification Specification](verification-spec.md)** - Detailed verification procedures and result format
-   **[Cryptographic Specification](crypto-spec.md)** - Cryptographic methods and requirements
-   **[Threat Model](threat-model.md)** - Security analysis and mitigation strategies

## Feedback

This specification is under active development. Feedback is welcome but expect the protocol to change significantly before stabilization.

---

**Last Updated**: August 2025  
**Status**: Preview / Unstable  
**Target Release**: TBD
