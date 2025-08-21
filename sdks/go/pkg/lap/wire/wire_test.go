package wire

import "testing"

func TestAttestationHeaderRoundTrip(t *testing.T) {
	a := Attestation{Payload: Payload{URL: "https://x", Hash: "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", ETag: "W/\"abc\"", IAT: 1, EXP: 2, KID: "k"}, Sig: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	enc, err := EncodeAttestationHeader(a)
	if err != nil {
		t.Fatal(err)
	}
	out, err := DecodeAttestationHeader(enc)
	if err != nil {
		t.Fatal(err)
	}
	if out.Payload.URL != a.Payload.URL || out.Sig != a.Sig {
		t.Fatalf("mismatch")
	}
}
