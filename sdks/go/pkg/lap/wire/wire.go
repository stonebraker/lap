package wire

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/canonical"
)

type ResourcePayload struct {
	URL             string `json:"url"`
	Attestation_URL string `json:"attestation_url"`
	Hash            string `json:"hash"`
	ETag            string `json:"etag"`
	IAT             int64  `json:"iat"`
	EXP             int64  `json:"exp"`
	KID             string `json:"kid"`
}

type ResourceAttestation struct {
	Payload      ResourcePayload `json:"payload"`
	ResourceKey  string          `json:"resource_key"`
	Sig          string          `json:"sig"`
}



type Bundle struct {
	Attestation        ResourceAttestation `json:"attestation"`
	ResourceKey        string              `json:"resource_key"`
	PublisherSignature string              `json:"publisher_signature"`
	PublisherKey       string              `json:"publisher_key"`
}

// ToCanonical transforms wire.ResourcePayload into canonical.ResourcePayloadCanonical for deterministic serialization.
func (p ResourcePayload) ToCanonical() canonical.ResourcePayloadCanonical {
	return canonical.ResourcePayloadCanonical{
		URL:             p.URL,
		Attestation_URL: p.Attestation_URL,
		Hash:            p.Hash,
		ETag:            p.ETag,
		IAT:             p.IAT,
		EXP:             p.EXP,
		KID:             p.KID,
	}
}

func (a ResourceAttestation) ToCanonical() canonical.ResourceAttestationCanonical {
	return canonical.ResourceAttestationCanonical{Payload: a.Payload.ToCanonical(), ResourceKey: a.ResourceKey, Sig: a.Sig}
}

func (b Bundle) ToCanonical() canonical.BundleCanonical {
	return canonical.BundleCanonical{
		Attestation:        b.Attestation.ToCanonical(),
		ResourceKey:        b.ResourceKey,
		PublisherSignature: b.PublisherSignature,
		PublisherKey:       b.PublisherKey,
	}
}

// EncodeAttestationHeader returns base64url(JSON) of {payload, resource_key, sig}.
func EncodeAttestationHeader(a ResourceAttestation) (string, error) {
	bytesJSON, err := canonical.MarshalResourceAttestationCanonical(a.ToCanonical())
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytesJSON), nil
}

// DecodeAttestationHeader parses base64url(JSON) header value into ResourceAttestation.
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

// Namespace wire types

type NamespacePayload struct {
	Namespace        []string `json:"namespace"`
	AttestationPath string   `json:"attestation_path"`
	IAT             int64    `json:"iat"`
	EXP             int64    `json:"exp"`
	KID             string   `json:"kid"`
}

type NamespaceAttestation struct {
	Payload      NamespacePayload `json:"payload"`
	PublisherKey string           `json:"publisher_key"`
	Sig          string           `json:"sig"`
}

func (p NamespacePayload) ToCanonical() canonical.NamespacePayloadCanonical {
	return canonical.NamespacePayloadCanonical{
		Namespace:        p.Namespace,
		AttestationPath: p.AttestationPath,
		IAT:             p.IAT,
		EXP:             p.EXP,
		KID:             p.KID,
	}
}

func (a NamespaceAttestation) ToCanonical() canonical.NamespaceAttestationCanonical {
	return canonical.NamespaceAttestationCanonical{Payload: a.Payload.ToCanonical(), PublisherKey: a.PublisherKey, Sig: a.Sig}
}
