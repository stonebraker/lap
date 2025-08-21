package verify

import (
	"testing"
	"time"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/canonical"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/crypto"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/wire"
)

func TestVerifyAttestation_Negatives(t *testing.T) {
	body := []byte("<span id=\"x\">hi</span>")
	etag := "W/\"" + crypto.HashSHA256Hex(body) + "\""
	url := "https://example.com/people/alice/messages/123"
	iat := time.Now().Unix()
	exp := iat + 600
	att, pubHex, err := func() (wire.Attestation, string, error) {
		priv, pub, err := crypto.GenerateKeyPair()
		if err != nil {
			return wire.Attestation{}, "", err
		}
		payload := wire.Payload{URL: url, Hash: crypto.ComputeContentHashField(body), ETag: etag, IAT: iat, EXP: exp, KID: "kid"}
		bytesPayload, err := canonical.MarshalPayloadCanonical(payload.ToCanonical())
		if err != nil {
			return wire.Attestation{}, "", err
		}
		digest := crypto.HashSHA256(bytesPayload)
		sigHex, err := crypto.SignSchnorrHex(priv, digest)
		if err != nil {
			return wire.Attestation{}, "", err
		}
		return wire.Attestation{Payload: payload, Sig: sigHex}, pub, nil
	}()
	if err != nil {
		t.Fatal(err)
	}

	// 1) Body tamper
	if err := VerifyAttestation(att, pubHex, []byte("tampered"), etag, url, time.Now()); err == nil {
		t.Fatalf("expected hash mismatch, got nil")
	}

	// 2) URL mismatch
	if err := VerifyAttestation(att, pubHex, body, etag, url+"?q=1", time.Now()); err == nil {
		t.Fatalf("expected url mismatch, got nil")
	}

	// 3) ETag mismatch
	if err := VerifyAttestation(att, pubHex, body, "W/\"deadbeef\"", url, time.Now()); err == nil {
		t.Fatalf("expected etag mismatch, got nil")
	}

	// 4) Expired
	attExpired := att
	attExpired.Payload.EXP = time.Now().Add(-1 * time.Minute).Unix()
	if err := VerifyAttestation(attExpired, pubHex, body, etag, url, time.Now()); err == nil {
		t.Fatalf("expected expired error, got nil")
	}

	// 5) Wrong key
	_, otherPub, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyAttestation(att, otherPub, body, etag, url, time.Now()); err == nil {
		t.Fatalf("expected signature verification failure, got nil")
	}
}
