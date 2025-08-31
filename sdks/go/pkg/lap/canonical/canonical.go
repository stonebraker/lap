package canonical

import (
	"encoding/json"
)

// ResourceAttestationCanonical for v0.2 maintains key order: fragment_url, hash, publisher_claim, namespace_attestation_url
type ResourceAttestationCanonical struct {
	FragmentURL             string `json:"fragment_url"`
	Hash                    string `json:"hash"`
	PublisherClaim          string `json:"publisher_claim"`
	NamespaceAttestationURL string `json:"namespace_attestation_url"`
}

// NamespacePayloadCanonical for v0.2 maintains key order: namespace, exp
type NamespacePayloadCanonical struct {
	Namespace string `json:"namespace"`
	Exp       int64  `json:"exp"`
}

// NamespaceAttestationCanonical for v0.2 maintains key order: payload, key, sig
type NamespaceAttestationCanonical struct {
	Payload NamespacePayloadCanonical `json:"payload"`
	Key     string                    `json:"key"`
	Sig     string                    `json:"sig"`
}

// MarshalResourceAttestationCanonical returns compact JSON for v0.2 ResourceAttestation with deterministic key order.
func MarshalResourceAttestationCanonical(ra ResourceAttestationCanonical) ([]byte, error) {
	return json.Marshal(ra)
}

// MarshalNamespacePayloadCanonical returns compact JSON for v0.2 NamespacePayload with deterministic key order.
func MarshalNamespacePayloadCanonical(p NamespacePayloadCanonical) ([]byte, error) {
	return json.Marshal(p)
}

// MarshalNamespaceAttestationCanonical returns compact JSON for v0.2 NamespaceAttestation with deterministic key order.
func MarshalNamespaceAttestationCanonical(na NamespaceAttestationCanonical) ([]byte, error) {
	return json.Marshal(na)
}
