package verify

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/canonical"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/crypto"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/wire"
)

// VerifyAttestation performs the end-to-end checks for an attestation given resource bytes and headers.
// - Confirms body hash and ETag match payload
// - Confirms signature over canonical payload JSON using the resource key
// - Confirms exp is in the future
func VerifyAttestation(att wire.Attestation, resourceKeyHex string, body []byte, etag string, fetchedURL string, now time.Time) error {
	if att.Payload.URL != fetchedURL {
		return fmt.Errorf("url mismatch: payload=%s fetched=%s", att.Payload.URL, fetchedURL)
	}
	computed := crypto.ComputeContentHashField(body)
	if att.Payload.Hash != computed {
		return fmt.Errorf("hash mismatch: payload=%s computed=%s", att.Payload.Hash, computed)
	}
	if att.Payload.ETag != etag {
		return fmt.Errorf("etag mismatch: payload=%s header=%s", att.Payload.ETag, etag)
	}
	if att.Payload.EXP <= now.Unix() {
		return errors.New("attestation expired")
	}
	// Canonical payload bytes
	bytesPayload, err := canonical.MarshalPayloadCanonical(att.Payload.ToCanonical())
	if err != nil {
		return err
	}
	digest := crypto.HashSHA256(bytesPayload)
	ok, err := crypto.VerifySchnorrHex(resourceKeyHex, att.Sig, digest)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("signature verification failed")
	}
	return nil
}

// VerifyPublisherProof verifies a publisher signature over the raw nonce bytes.
func VerifyPublisherProof(publisherKeyHex, nonceHex, signatureHex string) error {
	nonceBytes, err := hex.DecodeString(nonceHex)
	if err != nil {
		return err
	}
	if len(nonceBytes) != 32 {
		return fmt.Errorf("expected 32-byte nonce, got %d", len(nonceBytes))
	}
	digest := crypto.HashSHA256(nonceBytes)
	ok, err := crypto.VerifySchnorrHex(publisherKeyHex, signatureHex, digest)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("publisher signature invalid")
	}
	return nil
}
