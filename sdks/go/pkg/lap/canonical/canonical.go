package canonical

import (
	"encoding/json"
)

// PayloadCanonical mirrors the wire.Payload but locks the key order in JSON output
// by using struct field order. Keys: url, attestation_url, hash, etag, iat, exp, kid
type PayloadCanonical struct {
	URL             string `json:"url"`
	Attestation_URL string `json:"attestation_url"`
	Hash            string `json:"hash"`
	ETag            string `json:"etag"`
	IAT             int64  `json:"iat"`
	EXP             int64  `json:"exp"`
	KID             string `json:"kid"`
}

// AttestationCanonical maintains key order: payload, resource_key, sig
type AttestationCanonical struct {
	Payload     PayloadCanonical `json:"payload"`
	ResourceKey string           `json:"resource_key"`
	Sig         string           `json:"sig"`
}

// MarshalPayloadCanonical returns the compact JSON bytes for the payload with deterministic key order.
func MarshalPayloadCanonical(p PayloadCanonical) ([]byte, error) {
	return json.Marshal(p)
}

// MarshalAttestationCanonical returns compact JSON for {payload, resource_key, sig}.
func MarshalAttestationCanonical(a AttestationCanonical) ([]byte, error) {
	return json.Marshal(a)
}

// Namespace payload canonical: namespace, iat, exp, kid
type NamespacePayloadCanonical struct {
	Namespace []string `json:"namespace"`
	IAT       int64    `json:"iat"`
	EXP       int64    `json:"exp"`
	KID       string   `json:"kid"`
}

// Namespace attestation canonical: payload, publisher_key, sig
type NamespaceAttestationCanonical struct {
	Payload      NamespacePayloadCanonical `json:"payload"`
	PublisherKey string                    `json:"publisher_key"`
	Sig          string                    `json:"sig"`
}

func MarshalNamespacePayloadCanonical(p NamespacePayloadCanonical) ([]byte, error) {
	return json.Marshal(p)
}

func MarshalNamespaceAttestationCanonical(a NamespaceAttestationCanonical) ([]byte, error) {
	return json.Marshal(a)
}
