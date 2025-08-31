# LAP Protocol v0.2 Specification

> ✅ **COMPLETE SPECIFICATION - OPEN TO USE** ✅
>
> This is the **complete** LAP v0.2 specification. The protocol is **stable** and **open to implementation and feedback**.
>
> **⚠️ EXPERIMENTAL SOFTWARE - NOT PRODUCTION READY ⚠️**
>
> This specification is provided for:
>
> -   Implementation and testing
> -   Community feedback and review
> -   Protocol validation and improvement
>
> **Stability**: Specification stable, implementation experimental  
> **Breaking Changes**: None expected for v0.2  
> **Implementation Status**: Reference implementation conforms to v0.2

## Major Changes from v0.1

-   **Fragment Structure**: Simplified with `data-la-fragment-url` and consolidated verification metadata
-   **Cryptography**: Migration from Ed25519 to Schnorr signatures (BIP-340) with secp256k1
-   **Verification**: Streamlined three-check model (Presence, Integrity, Association)
-   **Security**: Enhanced threat model and mitigation analysis

## Specification Documents

-   **[Protocol Overview](overview.md)** - Core concepts and basic use case
-   **[Artifacts Specification](artifacts.md)** - Canonical schemas for all LAP artifacts
-   **[Roles Specification](roles-spec.md)** - Server, Client, and Verifier responsibilities
-   **[Verification Specification](verification-spec.md)** - Detailed verification procedures and result format
-   **[Cryptographic Specification](crypto-spec.md)** - Cryptographic methods and requirements
-   **[Threat Model](threat-model.md)** - Security analysis and mitigation strategies

## Feedback

This specification is complete and ready for implementation. Feedback and contributions are welcome to improve the protocol and reference implementation.

---

**Last Updated**: January 2025  
**Status**: Complete / Open to Use  
**Implementation**: Reference implementation available
