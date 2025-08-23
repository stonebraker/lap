package canonical

import (
	"encoding/json"
)

// ResourcePayloadCanonical mirrors the wire.ResourcePayload but locks the key order in JSON output
// by using struct field order. Keys: url, attestation_url, hash, etag, iat, exp, kid
type ResourcePayloadCanonical struct {
	URL             string `json:"url"`
	Attestation_URL string `json:"attestation_url"`
	Hash            string `json:"hash"`
	ETag            string `json:"etag"`
	IAT             int64  `json:"iat"`
	EXP             int64  `json:"exp"`
	KID             string `json:"kid"`
}

// ResourceAttestationCanonical maintains key order: payload, resource_key, sig
type ResourceAttestationCanonical struct {
	Payload     ResourcePayloadCanonical `json:"payload"`
	ResourceKey string                   `json:"resource_key"`
	Sig         string                   `json:"sig"`
}



// BundleCanonical maintains key order: attestation, resource_key, publisher_signature, publisher_key
type BundleCanonical struct {
	Attestation        ResourceAttestationCanonical `json:"attestation"`
	ResourceKey        string                       `json:"resource_key"`
	PublisherSignature string                       `json:"publisher_signature"`
	PublisherKey       string                       `json:"publisher_key"`
}

// MarshalResourcePayloadCanonical returns the compact JSON bytes for the payload with deterministic key order.
func MarshalResourcePayloadCanonical(p ResourcePayloadCanonical) ([]byte, error) {
	return json.Marshal(p)
}

// MarshalResourceAttestationCanonical returns compact JSON for {payload, resource_key, sig}.
func MarshalResourceAttestationCanonical(a ResourceAttestationCanonical) ([]byte, error) {
	return json.Marshal(a)
}



func MarshalBundleCanonical(b BundleCanonical) ([]byte, error) {
	return json.Marshal(b)
}

// Namespace payload canonical: namespace, attestation_path, iat, exp, kid
type NamespacePayloadCanonical struct {
	Namespace        []string `json:"namespace"`
	AttestationPath string   `json:"attestation_path"`
	IAT             int64    `json:"iat"`
	EXP             int64    `json:"exp"`
	KID             string   `json:"kid"`
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
