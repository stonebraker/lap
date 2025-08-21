package wire

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/canonical"
)

type Payload struct {
	URL             string `json:"url"`
	Attestation_URL string `json:"attestation_url"`
	Hash            string `json:"hash"`
	ETag            string `json:"etag"`
	IAT             int64  `json:"iat"`
	EXP             int64  `json:"exp"`
	KID             string `json:"kid"`
}

type Attestation struct {
	Payload     Payload `json:"payload"`
	ResourceKey string  `json:"resource_key"`
	Sig         string  `json:"sig"`
}

type Bundle struct {
	Attestation        Attestation `json:"attestation"`
	ResourceKey        string      `json:"resource_key"`
	ResourceNonce      string      `json:"resource_nonce"`
	PublisherSignature string      `json:"publisher_signature"`
	PublisherKey       string      `json:"publisher_key"`
}

// ToCanonical transforms wire.Payload into canonical.PayloadCanonical for deterministic serialization.
func (p Payload) ToCanonical() canonical.PayloadCanonical {
	return canonical.PayloadCanonical{
		URL:             p.URL,
		Attestation_URL: p.Attestation_URL,
		Hash:            p.Hash,
		ETag:            p.ETag,
		IAT:             p.IAT,
		EXP:             p.EXP,
		KID:             p.KID,
	}
}

func (a Attestation) ToCanonical() canonical.AttestationCanonical {
	return canonical.AttestationCanonical{Payload: a.Payload.ToCanonical(), ResourceKey: a.ResourceKey, Sig: a.Sig}
}

// EncodeAttestationHeader returns base64url(JSON) of {payload, resource_key, sig}.
func EncodeAttestationHeader(a Attestation) (string, error) {
	bytesJSON, err := canonical.MarshalAttestationCanonical(a.ToCanonical())
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytesJSON), nil
}

// DecodeAttestationHeader parses base64url(JSON) header value into Attestation.
func DecodeAttestationHeader(value string) (Attestation, error) {
	var zero Attestation
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
	Namespace []string `json:"namespace"`
	IAT       int64    `json:"iat"`
	EXP       int64    `json:"exp"`
	KID       string   `json:"kid"`
}

type NamespaceAttestation struct {
	Payload      NamespacePayload `json:"payload"`
	PublisherKey string           `json:"publisher_key"`
	Sig          string           `json:"sig"`
}

func (p NamespacePayload) ToCanonical() canonical.NamespacePayloadCanonical {
	return canonical.NamespacePayloadCanonical{
		Namespace: p.Namespace,
		IAT:       p.IAT,
		EXP:       p.EXP,
		KID:       p.KID,
	}
}

func (a NamespaceAttestation) ToCanonical() canonical.NamespaceAttestationCanonical {
	return canonical.NamespaceAttestationCanonical{Payload: a.Payload.ToCanonical(), PublisherKey: a.PublisherKey, Sig: a.Sig}
}
