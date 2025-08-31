package wire

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/canonical"
)

// Fragment represents the parsed LAP fragment for v0.2
type Fragment struct {
	Spec                        string `json:"spec"`                          // "v0.2"
	FragmentURL                 string `json:"fragment_url"`
	PreviewContent              string `json:"preview_content"`               // Raw HTML from .html file
	CanonicalContent            []byte `json:"canonical_content"`             // Same as preview, but as bytes
	PublisherClaim              string `json:"publisher_claim"`               // X-only public key
	ResourceAttestationURL      string `json:"resource_attestation_url"`
	NamespaceAttestationURL     string `json:"namespace_attestation_url"`
}

// ResourceAttestation for v0.2 (unsigned JSON format)
type ResourceAttestation struct {
	FragmentURL             string `json:"fragment_url"`
	Hash                    string `json:"hash"`                    // "sha256:..."
	PublisherClaim          string `json:"publisher_claim"`         // X-only public key for triangulation
	NamespaceAttestationURL string `json:"namespace_attestation_url"`
}

// NamespaceAttestation for v0.2 (signed JSON format)
type NamespaceAttestation struct {
	Payload NamespacePayload `json:"payload"`
	Key     string           `json:"key"`    // X-only public key (64 hex)
	Sig     string           `json:"sig"`    // Schnorr signature (128 hex)
}

type NamespacePayload struct {
	Namespace string `json:"namespace"`
	Exp       int64  `json:"exp"`
}

// ToCanonical transforms wire.ResourceAttestation into canonical.ResourceAttestationCanonical for deterministic serialization.
func (ra ResourceAttestation) ToCanonical() canonical.ResourceAttestationCanonical {
	return canonical.ResourceAttestationCanonical{
		FragmentURL:             ra.FragmentURL,
		Hash:                    ra.Hash,
		PublisherClaim:          ra.PublisherClaim,
		NamespaceAttestationURL: ra.NamespaceAttestationURL,
	}
}

// ToCanonical transforms wire.NamespacePayload into canonical.NamespacePayloadCanonical for deterministic serialization.
func (p NamespacePayload) ToCanonical() canonical.NamespacePayloadCanonical {
	return canonical.NamespacePayloadCanonical{
		Namespace: p.Namespace,
		Exp:       p.Exp,
	}
}

// ToCanonical transforms wire.NamespaceAttestation into canonical.NamespaceAttestationCanonical for deterministic serialization.
func (na NamespaceAttestation) ToCanonical() canonical.NamespaceAttestationCanonical {
	return canonical.NamespaceAttestationCanonical{
		Payload: na.Payload.ToCanonical(),
		Key:     na.Key,
		Sig:     na.Sig,
	}
}

// EncodeAttestationHeader returns base64url(JSON) of ResourceAttestation for v0.2.
func EncodeAttestationHeader(ra ResourceAttestation) (string, error) {
	bytesJSON, err := canonical.MarshalResourceAttestationCanonical(ra.ToCanonical())
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytesJSON), nil
}

// DecodeAttestationHeader parses base64url(JSON) header value into ResourceAttestation for v0.2.
func DecodeAttestationHeader(value string) (ResourceAttestation, error) {
	var zero ResourceAttestation
	if value == "" {
		return zero, errors.New("empty attestation header")
	}
	bytesJSON, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return zero, err
	}
	if err := json.Unmarshal(bytesJSON, &zero); err != nil {
		return zero, err
	}
	return zero, nil
}
