package verify

import (
	"testing"
	"time"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/canonical"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/crypto"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/wire"
)

func TestVerifyFragment_Success(t *testing.T) {
	// Generate a key pair first
	priv, pubKey, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Create test data
	content := []byte("<h1>Test Post</h1><p>Content</p>")
	contentHash := crypto.ComputeContentHashField(content)
	
	fragment := wire.Fragment{
		Spec:                        "v0.2",
		FragmentURL:                 "https://example.com/people/alice/frc/posts/123",
		PreviewContent:              string(content),
		CanonicalContent:            content,
		PublisherClaim:              pubKey, // Use the generated key
		ResourceAttestationURL:      "https://example.com/people/alice/frc/posts/123/_la_resource.json",
		NamespaceAttestationURL:     "https://example.com/people/alice/_la_namespace.json",
	}

	resourceAttestation := wire.ResourceAttestation{
		FragmentURL:             "https://example.com/people/alice/frc/posts/123",
		Hash:                    contentHash,
		PublisherClaim:          pubKey, // Use the generated key
		NamespaceAttestationURL: "https://example.com/people/alice/_la_namespace.json",
	}

	// Create a valid namespace attestation
	namespacePayload := wire.NamespacePayload{
		Namespace: "https://example.com/people/alice/",
		Exp:       time.Now().Add(1 * time.Hour).Unix(),
	}

	canonicalPayload := namespacePayload.ToCanonical()
	payloadBytes, err := canonical.MarshalNamespacePayloadCanonical(canonicalPayload)
	if err != nil {
		t.Fatal(err)
	}

	digest := crypto.HashSHA256(payloadBytes)
	sig, err := crypto.SignSchnorrHex(priv, digest)
	if err != nil {
		t.Fatal(err)
	}

	namespaceAttestation := wire.NamespaceAttestation{
		Payload: namespacePayload,
		Key:     pubKey,
		Sig:     sig,
	}

	// Perform verification
	result := VerifyFragment(fragment, resourceAttestation, namespaceAttestation)

	// Verify all checks passed
	if !result.Verified {
		t.Errorf("Expected verification to pass, got failure: %+v", result.Failure)
	}

	if result.ResourcePresence != "pass" {
		t.Errorf("Expected resource_presence to be 'pass', got '%s'", result.ResourcePresence)
	}

	if result.ResourceIntegrity != "pass" {
		t.Errorf("Expected resource_integrity to be 'pass', got '%s'", result.ResourceIntegrity)
	}

	if result.PublisherAssociation != "pass" {
		t.Errorf("Expected publisher_association to be 'pass', got '%s'", result.PublisherAssociation)
	}

	if result.Failure != nil {
		t.Errorf("Expected no failure, got %+v", result.Failure)
	}

	// Verify context
	if result.Context == nil {
		t.Fatal("Expected context to be set")
	}

	if result.Context.ResourceAttestationURL != fragment.ResourceAttestationURL {
		t.Errorf("Expected resource attestation URL to match, got %s, want %s", 
			result.Context.ResourceAttestationURL, fragment.ResourceAttestationURL)
	}

	if result.Context.NamespaceAttestationURL != fragment.NamespaceAttestationURL {
		t.Errorf("Expected namespace attestation URL to match, got %s, want %s", 
			result.Context.NamespaceAttestationURL, fragment.NamespaceAttestationURL)
	}
}

func TestVerifyFragment_ResourcePresenceFailure(t *testing.T) {
	content := []byte("<h1>Test Post</h1><p>Content</p>")
	
	fragment := wire.Fragment{
		Spec:                        "v0.2",
		FragmentURL:                 "https://example.com/people/alice/frc/posts/123",
		PreviewContent:              string(content),
		CanonicalContent:            content,
		PublisherClaim:              "f1a2d3c4e5f6078901234567890abcdef1234567890abcdef1234567890abcdef",
		ResourceAttestationURL:      "https://example.com/people/alice/frc/posts/123/_la_resource.json",
		NamespaceAttestationURL:     "https://example.com/people/alice/_la_namespace.json",
	}

	// Resource attestation with mismatched URL
	resourceAttestation := wire.ResourceAttestation{
		FragmentURL:             "https://example.com/people/alice/frc/posts/456", // Different URL
		Hash:                    "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		PublisherClaim:          "f1a2d3c4e5f6078901234567890abcdef1234567890abcdef1234567890abcdef",
		NamespaceAttestationURL: "https://example.com/people/alice/_la_namespace.json",
	}

	namespaceAttestation := wire.NamespaceAttestation{
		Payload: wire.NamespacePayload{
			Namespace: "https://example.com/people/alice/",
			Exp:       time.Now().Add(1 * time.Hour).Unix(),
		},
		Key: "f1a2d3c4e5f6078901234567890abcdef1234567890abcdef1234567890abcdef",
		Sig: "dummy-signature",
	}

	result := VerifyFragment(fragment, resourceAttestation, namespaceAttestation)

	// Should fail at resource presence check
	if result.Verified {
		t.Error("Expected verification to fail")
	}

	if result.ResourcePresence != "fail" {
		t.Errorf("Expected resource_presence to be 'fail', got '%s'", result.ResourcePresence)
	}

	if result.ResourceIntegrity != "skip" {
		t.Errorf("Expected resource_integrity to be 'skip', got '%s'", result.ResourceIntegrity)
	}

	if result.PublisherAssociation != "skip" {
		t.Errorf("Expected publisher_association to be 'skip', got '%s'", result.PublisherAssociation)
	}

	if result.Failure == nil {
		t.Fatal("Expected failure details")
	}

	if result.Failure.Check != "resource_presence" {
		t.Errorf("Expected failure check to be 'resource_presence', got '%s'", result.Failure.Check)
	}

	if result.Failure.Reason != "fragment_url_mismatch" {
		t.Errorf("Expected failure reason to be 'fragment_url_mismatch', got '%s'", result.Failure.Reason)
	}
}

func TestVerifyFragment_ResourceIntegrityFailure(t *testing.T) {
	content := []byte("<h1>Test Post</h1><p>Content</p>")
	
	fragment := wire.Fragment{
		Spec:                        "v0.2",
		FragmentURL:                 "https://example.com/people/alice/frc/posts/123",
		PreviewContent:              string(content),
		CanonicalContent:            content,
		PublisherClaim:              "f1a2d3c4e5f6078901234567890abcdef1234567890abcdef1234567890abcdef",
		ResourceAttestationURL:      "https://example.com/people/alice/frc/posts/123/_la_resource.json",
		NamespaceAttestationURL:     "https://example.com/people/alice/_la_namespace.json",
	}

	// Resource attestation with correct URL but wrong hash
	resourceAttestation := wire.ResourceAttestation{
		FragmentURL:             "https://example.com/people/alice/frc/posts/123",
		Hash:                    "sha256:wronghash", // Wrong hash
		PublisherClaim:          "f1a2d3c4e5f6078901234567890abcdef1234567890abcdef1234567890abcdef",
		NamespaceAttestationURL: "https://example.com/people/alice/_la_namespace.json",
	}

	namespaceAttestation := wire.NamespaceAttestation{
		Payload: wire.NamespacePayload{
			Namespace: "https://example.com/people/alice/",
			Exp:       time.Now().Add(1 * time.Hour).Unix(),
		},
		Key: "f1a2d3c4e5f6078901234567890abcdef1234567890abcdef1234567890abcdef",
		Sig: "dummy-signature",
	}

	result := VerifyFragment(fragment, resourceAttestation, namespaceAttestation)

	// Should pass resource presence but fail at resource integrity
	if result.Verified {
		t.Error("Expected verification to fail")
	}

	if result.ResourcePresence != "pass" {
		t.Errorf("Expected resource_presence to be 'pass', got '%s'", result.ResourcePresence)
	}

	if result.ResourceIntegrity != "fail" {
		t.Errorf("Expected resource_integrity to be 'fail', got '%s'", result.ResourceIntegrity)
	}

	if result.PublisherAssociation != "skip" {
		t.Errorf("Expected publisher_association to be 'skip', got '%s'", result.PublisherAssociation)
	}

	if result.Failure == nil {
		t.Fatal("Expected failure details")
	}

	if result.Failure.Check != "resource_integrity" {
		t.Errorf("Expected failure check to be 'resource_integrity', got '%s'", result.Failure.Check)
	}

	if result.Failure.Reason != "hash_mismatch" {
		t.Errorf("Expected failure reason to be 'hash_mismatch', got '%s'", result.Failure.Reason)
	}
}

func TestVerifyFragment_PublisherAssociationFailure(t *testing.T) {
	content := []byte("<h1>Test Post</h1><p>Content</p>")
	contentHash := crypto.ComputeContentHashField(content)
	
	fragment := wire.Fragment{
		Spec:                        "v0.2",
		FragmentURL:                 "https://example.com/people/alice/frc/posts/123",
		PreviewContent:              string(content),
		CanonicalContent:            content,
		PublisherClaim:              "f1a2d3c4e5f6078901234567890abcdef1234567890abcdef1234567890abcdef",
		ResourceAttestationURL:      "https://example.com/people/alice/frc/posts/123/_la_resource.json",
		NamespaceAttestationURL:     "https://example.com/people/alice/_la_namespace.json",
	}

	resourceAttestation := wire.ResourceAttestation{
		FragmentURL:             "https://example.com/people/alice/frc/posts/123",
		Hash:                    contentHash,
		PublisherClaim:          "f1a2d3c4e5f6078901234567890abcdef1234567890abcdef1234567890abcdef",
		NamespaceAttestationURL: "https://example.com/people/alice/_la_namespace.json",
	}

	// Namespace attestation with wrong key
	namespaceAttestation := wire.NamespaceAttestation{
		Payload: wire.NamespacePayload{
			Namespace: "https://example.com/people/alice/",
			Exp:       time.Now().Add(1 * time.Hour).Unix(),
		},
		Key: "differentkey1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", // Different key
		Sig: "dummy-signature",
	}

	result := VerifyFragment(fragment, resourceAttestation, namespaceAttestation)

	// Should pass resource presence and integrity but fail at publisher association
	if result.Verified {
		t.Error("Expected verification to fail")
	}

	if result.ResourcePresence != "pass" {
		t.Errorf("Expected resource_presence to be 'pass', got '%s'", result.ResourcePresence)
	}

	if result.ResourceIntegrity != "pass" {
		t.Errorf("Expected resource_integrity to be 'pass', got '%s'", result.ResourceIntegrity)
	}

	if result.PublisherAssociation != "fail" {
		t.Errorf("Expected publisher_association to be 'fail', got '%s'", result.PublisherAssociation)
	}

	if result.Failure == nil {
		t.Fatal("Expected failure details")
	}

	if result.Failure.Check != "publisher_association" {
		t.Errorf("Expected failure check to be 'publisher_association', got '%s'", result.Failure.Check)
	}

	if result.Failure.Reason != "publisher_claim_mismatch" {
		t.Errorf("Expected failure reason to be 'publisher_claim_mismatch', got '%s'", result.Failure.Reason)
	}
}

func TestVerifyFragment_ExpiredNamespaceAttestation(t *testing.T) {
	content := []byte("<h1>Test Post</h1><p>Content</p>")
	contentHash := crypto.ComputeContentHashField(content)
	
	// Use a consistent key for this test
	testKey := "f1a2d3c4e5f6078901234567890abcdef1234567890abcdef1234567890abcdef"
	
	fragment := wire.Fragment{
		Spec:                        "v0.2",
		FragmentURL:                 "https://example.com/people/alice/frc/posts/123",
		PreviewContent:              string(content),
		CanonicalContent:            content,
		PublisherClaim:              testKey,
		ResourceAttestationURL:      "https://example.com/people/alice/frc/posts/123/_la_resource.json",
		NamespaceAttestationURL:     "https://example.com/people/alice/_la_namespace.json",
	}

	resourceAttestation := wire.ResourceAttestation{
		FragmentURL:             "https://example.com/people/alice/frc/posts/123",
		Hash:                    contentHash,
		PublisherClaim:          testKey,
		NamespaceAttestationURL: "https://example.com/people/alice/_la_namespace.json",
	}

	// Expired namespace attestation
	namespaceAttestation := wire.NamespaceAttestation{
		Payload: wire.NamespacePayload{
			Namespace: "https://example.com/people/alice/",
			Exp:       time.Now().Add(-1 * time.Hour).Unix(), // Expired
		},
		Key: testKey,
		Sig: "dummy-signature",
	}

	result := VerifyFragment(fragment, resourceAttestation, namespaceAttestation)

	// Should pass resource presence and integrity but fail at publisher association
	if result.Verified {
		t.Error("Expected verification to fail")
	}

	if result.ResourcePresence != "pass" {
		t.Errorf("Expected resource_presence to be 'pass', got '%s'", result.ResourcePresence)
	}

	if result.ResourceIntegrity != "pass" {
		t.Errorf("Expected resource_integrity to be 'pass', got '%s'", result.ResourceIntegrity)
	}

	if result.PublisherAssociation != "fail" {
		t.Errorf("Expected publisher_association to be 'fail', got '%s'", result.PublisherAssociation)
	}

	if result.Failure == nil {
		t.Fatal("Expected failure details")
	}

	if result.Failure.Check != "publisher_association" {
		t.Errorf("Expected failure check to be 'publisher_association', got '%s'", result.Failure.Check)
	}

	if result.Failure.Reason != "expired" {
		t.Errorf("Expected failure reason to be 'expired', got '%s' (message: %s)", result.Failure.Reason, result.Failure.Message)
	}
}

func TestIsURLUnderNamespace(t *testing.T) {
	tests := []struct {
		url       string
		namespace string
		expected  bool
	}{
		{
			url:       "https://example.com/people/alice/frc/posts/123",
			namespace: "https://example.com/people/alice/",
			expected:  true,
		},
		{
			url:       "https://example.com/people/alice/frc/posts/123",
			namespace: "https://example.com/people/alice",
			expected:  true,
		},
		{
			url:       "https://example.com/people/alice/frc/posts/123",
			namespace: "https://example.com/people/bob/",
			expected:  false,
		},
		{
			url:       "https://example.com/people/alice/frc/posts/123",
			namespace: "https://other.com/people/alice/",
			expected:  false,
		},
		{
			url:       "https://example.com/people/alice",
			namespace: "https://example.com/people/alice/",
			expected:  true,
		},
	}

	for _, test := range tests {
		result := isURLUnderNamespace(test.url, test.namespace)
		if result != test.expected {
			t.Errorf("isURLUnderNamespace(%q, %q) = %v, want %v", 
				test.url, test.namespace, result, test.expected)
		}
	}
}
