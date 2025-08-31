package wire

import "testing"

func TestAttestationHeaderRoundTrip(t *testing.T) {
	ra := ResourceAttestation{
		FragmentURL:             "https://example.com/test",
		Hash:                    "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		PublisherClaim:          "f1a2d3c4e5f6078901234567890abcdef1234567890abcdef1234567890abcdef",
		NamespaceAttestationURL: "https://example.com/_la_namespace.json",
	}
	enc, err := EncodeAttestationHeader(ra)
	if err != nil {
		t.Fatal(err)
	}
	out, err := DecodeAttestationHeader(enc)
	if err != nil {
		t.Fatal(err)
	}
	if out.FragmentURL != ra.FragmentURL || out.Hash != ra.Hash || out.PublisherClaim != ra.PublisherClaim || out.NamespaceAttestationURL != ra.NamespaceAttestationURL {
		t.Fatalf("mismatch: got %+v, want %+v", out, ra)
	}
}
