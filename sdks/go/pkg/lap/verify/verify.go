package verify

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/canonical"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/crypto"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/wire"
)

// VerificationResult represents the result of v0.2 verification
type VerificationResult struct {
	Verified             bool                 `json:"verified"`
	ResourcePresence     string              `json:"resource_presence"`     // "pass", "fail", "skip"
	ResourceIntegrity    string              `json:"resource_integrity"`    // "pass", "fail", "skip"
	PublisherAssociation string              `json:"publisher_association"` // "pass", "fail", "skip"
	Failure              *FailureDetails     `json:"failure"`
	Context              *VerificationContext `json:"context"`
}

// FailureDetails provides information about verification failures
type FailureDetails struct {
	Check   string                 `json:"check"`
	Reason  string                 `json:"reason"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details"`
}

// VerificationContext provides metadata about the verification
type VerificationContext struct {
	ResourceAttestationURL  string `json:"resource_attestation_url"`
	NamespaceAttestationURL string `json:"namespace_attestation_url"`
	VerifiedAt             int64  `json:"verified_at"`
}

// VerifyFragment performs the three-step v0.2 verification process
func VerifyFragment(fragment wire.Fragment, resourceAttestation wire.ResourceAttestation, namespaceAttestation wire.NamespaceAttestation) VerificationResult {
	result := VerificationResult{
		ResourcePresence:     "skip",
		ResourceIntegrity:    "skip",
		PublisherAssociation: "skip",
		Context: &VerificationContext{
			ResourceAttestationURL:  fragment.ResourceAttestationURL,
			NamespaceAttestationURL: fragment.NamespaceAttestationURL,
			VerifiedAt:             time.Now().Unix(),
		},
	}

	// Step 1: Resource Presence check
	if err := verifyResourcePresence(fragment, resourceAttestation); err != nil {
		result.Failure = &FailureDetails{
			Check:   "resource_presence",
			Reason:  classifyResourcePresenceError(err),
			Message: err.Error(),
			Details:  getResourcePresenceFailureDetails(err, fragment, resourceAttestation),
		}
		result.ResourcePresence = "fail"
		return result
	}
	result.ResourcePresence = "pass"

	// Step 2: Resource Integrity check
	if err := verifyResourceIntegrity(fragment, resourceAttestation); err != nil {
		result.Failure = &FailureDetails{
			Check:   "resource_integrity",
			Reason:  "hash_mismatch",
			Message: err.Error(),
			Details:  getResourceIntegrityFailureDetails(fragment, resourceAttestation),
		}
		result.ResourceIntegrity = "fail"
		return result
	}
	result.ResourceIntegrity = "pass"

	// Step 3: Publisher Association check
	if err := verifyPublisherAssociation(fragment, resourceAttestation, namespaceAttestation); err != nil {
		result.Failure = &FailureDetails{
			Check:   "publisher_association",
			Reason:  classifyPublisherAssociationError(err),
			Message: err.Error(),
			Details:  getPublisherAssociationFailureDetails(err, fragment, resourceAttestation, namespaceAttestation),
		}
		result.PublisherAssociation = "fail"
		return result
	}
	result.PublisherAssociation = "pass"

	// All checks passed
	result.Verified = true
	return result
}

// verifyResourcePresence checks that the Resource Attestation is accessible and matches the fragment
func verifyResourcePresence(fragment wire.Fragment, ra wire.ResourceAttestation) error {
	// Check URL matching
	if ra.FragmentURL != fragment.FragmentURL {
		return fmt.Errorf("resource attestation fragment URL mismatch: got %s, want %s", ra.FragmentURL, fragment.FragmentURL)
	}

	// Check publisher claim triangulation
	if ra.PublisherClaim != fragment.PublisherClaim {
		return fmt.Errorf("publisher claim mismatch: got %s, want %s", ra.PublisherClaim, fragment.PublisherClaim)
	}

	// Check namespace attestation URL consistency
	if ra.NamespaceAttestationURL != fragment.NamespaceAttestationURL {
		return fmt.Errorf("namespace attestation URL mismatch: got %s, want %s", ra.NamespaceAttestationURL, fragment.NamespaceAttestationURL)
	}

	// Check same-origin validation: Resource Attestation URL must have same origin as claimed resource URL
	if !isSameOrigin(fragment.FragmentURL, fragment.ResourceAttestationURL) {
		return fmt.Errorf("resource attestation URL origin mismatch: resource %s, attestation %s", fragment.FragmentURL, fragment.ResourceAttestationURL)
	}

	// Check same-origin validation: Namespace Attestation URL must have same origin as claimed resource URL
	if !isSameOrigin(fragment.FragmentURL, fragment.NamespaceAttestationURL) {
		return fmt.Errorf("namespace attestation URL origin mismatch: resource %s, attestation %s", fragment.FragmentURL, fragment.NamespaceAttestationURL)
	}

	return nil
}

// verifyResourceIntegrity checks that the content hash matches the Resource Attestation
func verifyResourceIntegrity(fragment wire.Fragment, ra wire.ResourceAttestation) error {
	computedHash := crypto.ComputeContentHashField(fragment.CanonicalContent)
	if ra.Hash != computedHash {
		return fmt.Errorf("content hash mismatch: got %s, want %s", ra.Hash, computedHash)
	}
	return nil
}

// verifyPublisherAssociation checks the Namespace Attestation signature and coverage
func verifyPublisherAssociation(fragment wire.Fragment, ra wire.ResourceAttestation, na wire.NamespaceAttestation) error {
	// Check that the fragment URL is covered by the namespace
	if !isURLUnderNamespace(fragment.FragmentURL, na.Payload.Namespace) {
		return fmt.Errorf("fragment URL %s is not covered by namespace %s", fragment.FragmentURL, na.Payload.Namespace)
	}

	// Check that the namespace attestation key matches the publisher claim
	if na.Key != fragment.PublisherClaim {
		return fmt.Errorf("namespace attestation key mismatch: got %s, want %s", na.Key, fragment.PublisherClaim)
	}

	// Check expiration
	if na.Payload.Exp <= time.Now().Unix() {
		return errors.New("namespace attestation expired")
	}

	// Verify the signature over the canonical payload
	canonicalPayload := na.Payload.ToCanonical()
	payloadBytes, err := canonical.MarshalNamespacePayloadCanonical(canonicalPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal canonical payload: %w", err)
	}

	digest := crypto.HashSHA256(payloadBytes)
	ok, err := crypto.VerifySchnorrHex(na.Key, na.Sig, digest)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	if !ok {
		return errors.New("namespace attestation signature invalid")
	}

	return nil
}

// isURLUnderNamespace checks if a URL is covered by a namespace
func isURLUnderNamespace(url, namespace string) bool {
	// Handle exact match
	if url == namespace {
		return true
	}
	
	// Handle prefix matching (namespace must end with / for proper prefix matching)
	if strings.HasSuffix(namespace, "/") {
		if strings.HasPrefix(url, namespace) {
			return true
		}
	}
	
	// If namespace doesn't end with /, check if URL starts with namespace + "/"
	// This handles cases like namespace="https://example.com/people/alice" 
	// and URL="https://example.com/people/alice/posts/123"
	if strings.HasPrefix(url, namespace+"/") {
		return true
	}
	
	// Special case: if URL and namespace are the same when trailing slashes are removed
	// This handles cases like:
	// - URL: "https://example.com/people/alice" (no trailing slash)
	// - Namespace: "https://example.com/people/alice/" (with trailing slash)
	trimmedURL := strings.TrimSuffix(url, "/")
	trimmedNamespace := strings.TrimSuffix(namespace, "/")
	
	if trimmedURL == trimmedNamespace {
		return true
	}
	
	// Otherwise, treat as exact match only
	return url == namespace
}

// isSameOrigin checks if two URLs have the same origin (scheme + host)
func isSameOrigin(url1, url2 string) bool {
	u1, err := url.Parse(url1)
	if err != nil {
		return false
	}
	u2, err := url.Parse(url2)
	if err != nil {
		return false
	}
	return strings.EqualFold(u1.Scheme, u2.Scheme) && strings.EqualFold(u1.Host, u2.Host)
}

// classifyResourcePresenceError categorizes resource presence errors
func classifyResourcePresenceError(err error) string {
	errStr := err.Error()
	if contains(errStr, "resource attestation fragment URL mismatch") {
		return "fragment_url_mismatch"
	}
	if contains(errStr, "publisher claim mismatch") {
		return "publisher_claim_mismatch"
	}
	if contains(errStr, "namespace attestation URL mismatch") {
		return "namespace_url_mismatch"
	}
	if contains(errStr, "resource attestation URL origin mismatch") {
		return "origin_mismatch"
	}
	if contains(errStr, "namespace attestation URL origin mismatch") {
		return "origin_mismatch"
	}
	return "validation_failed"
}

// classifyPublisherAssociationError categorizes publisher association errors
func classifyPublisherAssociationError(err error) string {
	errStr := err.Error()
	if contains(errStr, "not covered by namespace") {
		return "url_not_under_namespace"
	}
	if contains(errStr, "namespace attestation key mismatch") {
		return "publisher_claim_mismatch"
	}
	if contains(errStr, "namespace attestation expired") {
		return "expired"
	}
	if contains(errStr, "signature invalid") {
		return "signature_invalid"
	}
	return "validation_failed"
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

// getResourcePresenceFailureDetails provides detailed failure information for Resource Presence check
func getResourcePresenceFailureDetails(err error, fragment wire.Fragment, ra wire.ResourceAttestation) map[string]interface{} {
	errStr := err.Error()
	details := map[string]interface{}{
		"fragment_url": fragment.FragmentURL,
		"ra_url":       ra.FragmentURL,
	}
	
	if contains(errStr, "resource attestation fragment URL mismatch") {
		details["expected"] = fragment.FragmentURL
		details["actual"] = ra.FragmentURL
	} else if contains(errStr, "publisher claim mismatch") {
		details["expected"] = fragment.PublisherClaim
		details["actual"] = ra.PublisherClaim
	} else if contains(errStr, "namespace attestation URL mismatch") {
		details["expected"] = fragment.NamespaceAttestationURL
		details["actual"] = ra.NamespaceAttestationURL
	} else if contains(errStr, "resource attestation URL origin mismatch") {
		details["resource_url"] = fragment.FragmentURL
		details["attestation_url"] = fragment.ResourceAttestationURL
	} else if contains(errStr, "namespace attestation URL origin mismatch") {
		details["resource_url"] = fragment.FragmentURL
		details["attestation_url"] = fragment.NamespaceAttestationURL
	}
	
	return details
}

// getResourceIntegrityFailureDetails provides detailed failure information for Resource Integrity check
func getResourceIntegrityFailureDetails(fragment wire.Fragment, ra wire.ResourceAttestation) map[string]interface{} {
	computedHash := crypto.ComputeContentHashField(fragment.CanonicalContent)
	return map[string]interface{}{
		"expected": ra.Hash,
		"actual":   computedHash,
		"content_length": len(fragment.CanonicalContent),
	}
}

// getPublisherAssociationFailureDetails provides detailed failure information for Publisher Association check
func getPublisherAssociationFailureDetails(err error, fragment wire.Fragment, ra wire.ResourceAttestation, na wire.NamespaceAttestation) map[string]interface{} {
	errStr := err.Error()
	details := map[string]interface{}{
		"fragment_url": fragment.FragmentURL,
		"namespace":    na.Payload.Namespace,
	}
	
	if contains(errStr, "not covered by namespace") {
		details["resource_url"] = fragment.FragmentURL
		details["namespace"] = na.Payload.Namespace
	} else if contains(errStr, "namespace attestation key mismatch") {
		details["expected"] = fragment.PublisherClaim
		details["actual"] = na.Key
	} else if contains(errStr, "namespace attestation expired") {
		details["expires_at"] = na.Payload.Exp
		details["current_time"] = time.Now().Unix()
	}
	
	return details
}
