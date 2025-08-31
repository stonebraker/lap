package canonical

import (
	"encoding/json"
	"testing"
)

func TestResourceAttestationCanonical_FieldOrder(t *testing.T) {
	ra := ResourceAttestationCanonical{
		FragmentURL:             "https://example.com/resource",
		Hash:                    "sha256:abc123",
		PublisherClaim:          "def456",
		NamespaceAttestationURL: "https://example.com/namespace.json",
	}

	bytes, err := MarshalResourceAttestationCanonical(ra)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Verify the JSON has the expected field order
	expected := `{"fragment_url":"https://example.com/resource","hash":"sha256:abc123","publisher_claim":"def456","namespace_attestation_url":"https://example.com/namespace.json"}`
	if string(bytes) != expected {
		t.Errorf("Field order mismatch:\ngot:  %s\nwant: %s", string(bytes), expected)
	}
}

func TestNamespacePayloadCanonical_FieldOrder(t *testing.T) {
	np := NamespacePayloadCanonical{
		Namespace: "https://example.com/people/alice/",
		Exp:       1754909100,
	}

	bytes, err := MarshalNamespacePayloadCanonical(np)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Verify the JSON has the expected field order
	expected := `{"namespace":"https://example.com/people/alice/","exp":1754909100}`
	if string(bytes) != expected {
		t.Errorf("Field order mismatch:\ngot:  %s\nwant: %s", string(bytes), expected)
	}
}

func TestNamespaceAttestationCanonical_FieldOrder(t *testing.T) {
	na := NamespaceAttestationCanonical{
		Payload: NamespacePayloadCanonical{
			Namespace: "https://example.com/people/alice/",
			Exp:       1754909100,
		},
		Key: "f1a2d3c4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00",
		Sig: "4e0f1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7",
	}

	bytes, err := MarshalNamespaceAttestationCanonical(na)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Verify the JSON has the expected field order
	expected := `{"payload":{"namespace":"https://example.com/people/alice/","exp":1754909100},"key":"f1a2d3c4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00","sig":"4e0f1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7"}`
	if string(bytes) != expected {
		t.Errorf("Field order mismatch:\ngot:  %s\nwant: %s", string(bytes), expected)
	}
}

func TestCanonicalSerialization_Deterministic(t *testing.T) {
	// Test that multiple serializations produce identical output
	ra := ResourceAttestationCanonical{
		FragmentURL:             "https://example.com/resource",
		Hash:                    "sha256:abc123",
		PublisherClaim:          "def456",
		NamespaceAttestationURL: "https://example.com/namespace.json",
	}

	bytes1, err := MarshalResourceAttestationCanonical(ra)
	if err != nil {
		t.Fatalf("Failed to marshal first time: %v", err)
	}

	bytes2, err := MarshalResourceAttestationCanonical(ra)
	if err != nil {
		t.Fatalf("Failed to marshal second time: %v", err)
	}

	if string(bytes1) != string(bytes2) {
		t.Errorf("Serialization not deterministic:\nfirst:  %s\nsecond: %s", string(bytes1), string(bytes2))
	}
}

func TestCanonicalSerialization_NoWhitespace(t *testing.T) {
	ra := ResourceAttestationCanonical{
		FragmentURL:             "https://example.com/resource",
		Hash:                    "sha256:abc123",
		PublisherClaim:          "def456",
		NamespaceAttestationURL: "https://example.com/namespace.json",
	}

	bytes, err := MarshalResourceAttestationCanonical(ra)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Verify no whitespace between elements
	jsonStr := string(bytes)
	if contains(jsonStr, " ") {
		t.Errorf("JSON contains whitespace: %s", jsonStr)
	}
}

func TestCanonicalSerialization_UTF8Encoding(t *testing.T) {
	// Test with UTF-8 content
	ra := ResourceAttestationCanonical{
		FragmentURL:             "https://example.com/resource",
		Hash:                    "sha256:abc123",
		PublisherClaim:          "def456",
		NamespaceAttestationURL: "https://example.com/namespace.json",
	}

	bytes, err := MarshalResourceAttestationCanonical(ra)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Verify UTF-8 encoding
	if !json.Valid(bytes) {
		t.Errorf("Generated JSON is not valid UTF-8: %s", string(bytes))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
