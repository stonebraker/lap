// Copyright 2025 Jason Stonebraker
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/verify"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/wire"
)

// processFragmentVerification processes a complete HTML fragment and performs LAP v0.2 verification
func processFragmentVerification(htmlContent string, actualFetchURL string) (*verify.VerificationResult, error) {
	// Parse the fragment from the HTML content
	fragment, err := parseFragmentFromHTML(htmlContent, actualFetchURL)
	if err != nil {
		return &verify.VerificationResult{
			Verified:         false,
			ResourcePresence: "fail",
			Failure: &verify.FailureDetails{
				Check:   "resource_presence",
				Reason:  "malformed",
				Message: fmt.Sprintf("failed to parse fragment: %v", err),
			},
			Context: &verify.VerificationContext{
				VerifiedAt: time.Now().Unix(),
			},
		}, nil
	}

	// Create HTTP client for fetching attestations
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}
			prev := via[len(via)-1]
			if !sameOrigin(prev.URL, req.URL) {
				return fmt.Errorf("cross-origin redirect not allowed")
			}
			if len(via) > 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// Fetch the Resource Attestation
	resourceAttestation, err := fetchResourceAttestation(client, fragment.ResourceAttestationURL)
	if err != nil {
		return &verify.VerificationResult{
			Verified:         false,
			ResourcePresence: "fail",
			Failure: &verify.FailureDetails{
				Check:   "resource_presence",
				Reason:  "fetch_failed",
				Message: fmt.Sprintf("failed to fetch resource attestation: %v", err),
				Details: map[string]interface{}{
					"resource_attestation_url": fragment.ResourceAttestationURL,
				},
			},
			Context: &verify.VerificationContext{
				ResourceAttestationURL:  fragment.ResourceAttestationURL,
				NamespaceAttestationURL: fragment.NamespaceAttestationURL,
				VerifiedAt:             time.Now().Unix(),
			},
		}, nil
	}

	// Validate Resource Attestation has required fields
	resourceAttestation, err = validateRequiredResourceAttestationFields(*resourceAttestation)
	if err != nil {
		return &verify.VerificationResult{
			Verified:         false,
			ResourcePresence: "fail",
			Failure: &verify.FailureDetails{
				Check:   "resource_presence",
				Reason:  "malformed",
				Message: fmt.Sprintf("failed to validate resource attestation fields: %v", err),
				Details: map[string]interface{}{
					"resource_attestation_url": fragment.ResourceAttestationURL,
				},
			},
			Context: &verify.VerificationContext{
				ResourceAttestationURL:  fragment.ResourceAttestationURL,
				NamespaceAttestationURL: fragment.NamespaceAttestationURL,
				VerifiedAt:             time.Now().Unix(),
			},
		}, nil
	}

	// Fetch the Namespace Attestation
	namespaceAttestation, err := fetchNamespaceAttestation(client, fragment.NamespaceAttestationURL)
	if err != nil {
		return &verify.VerificationResult{
			Verified:             false,
			ResourcePresence:     "pass",
			ResourceIntegrity:    "pass",
			PublisherAssociation: "fail",
			Failure: &verify.FailureDetails{
				Check:   "publisher_association",
				Reason:  "fetch_failed",
				Message: fmt.Sprintf("failed to fetch namespace attestation: %v", err),
				Details: map[string]interface{}{
					"namespace_attestation_url": fragment.NamespaceAttestationURL,
				},
			},
			Context: &verify.VerificationContext{
				ResourceAttestationURL:  fragment.ResourceAttestationURL,
				NamespaceAttestationURL: fragment.NamespaceAttestationURL,
				VerifiedAt:             time.Now().Unix(),
			},
		}, nil
	}

	// Perform v0.2 verification using the verify package
	result := verify.VerifyFragment(*fragment, *resourceAttestation, *namespaceAttestation)

	// Update context with URLs
	result.Context.ResourceAttestationURL = fragment.ResourceAttestationURL
	result.Context.NamespaceAttestationURL = fragment.NamespaceAttestationURL

	return &result, nil
}

// parseFragmentFromHTML extracts a LAP fragment from HTML content
// This is adapted from the verifier CLI implementation
func parseFragmentFromHTML(htmlContent string, actualFetchURL string) (*wire.Fragment, error) {
	// Use the actual fetch URL as the fragment URL, not the one claimed in the HTML
	fragmentURL := actualFetchURL
	if fragmentURL == "" {
		// Fallback to extracting from HTML if no actual fetch URL provided
		needle := `data-la-fragment-url="`
		idx := strings.Index(htmlContent, needle)
		if idx < 0 {
			return nil, fmt.Errorf("no fragment found with data-la-fragment-url attribute")
		}

		// Extract the actual fragment URL from the HTML
		fragmentURLStart := idx + len(needle)
		fragmentURLEnd := strings.Index(htmlContent[fragmentURLStart:], `"`)
		if fragmentURLEnd < 0 {
			return nil, fmt.Errorf("fragment structure malformed: incomplete data-la-fragment-url attribute")
		}
		fragmentURL = htmlContent[fragmentURLStart : fragmentURLStart+fragmentURLEnd]
	}

	// Find the start of the article element
	// Look for any fragment in the HTML (we'll use the first one found)
	needle := `data-la-fragment-url="`
	idx := strings.Index(htmlContent, needle)
	if idx < 0 {
		return nil, fmt.Errorf("no fragment found with data-la-fragment-url attribute")
	}

	// Find the start of the article element
	start := strings.LastIndex(htmlContent[:idx], "<article")
	if start < 0 {
		return nil, fmt.Errorf("fragment structure malformed: no <article> tag found")
	}

	// Find the end of the article element
	rest := htmlContent[start:]
	depth := 0
	i := 0
	for i < len(rest) {
		if rest[i] == '<' {
			if strings.HasPrefix(rest[i:], "<article") {
				depth++
			} else if strings.HasPrefix(rest[i:], "</article") {
				depth--
				endTag := strings.Index(rest[i:], ">")
				if endTag >= 0 {
					i += endTag + 1
				} else {
					break
				}
				if depth == 0 {
					endAbs := start + i
					articleHTML := htmlContent[start:endAbs]
					return parseFragmentFromArticle(articleHTML, fragmentURL)
				}
				continue
			}
			end := strings.Index(rest[i:], ">")
			if end >= 0 {
				i += end + 1
				continue
			}
			break
		}
		i++
	}

	return nil, fmt.Errorf("fragment structure malformed: incomplete <article> tag")
}

// parseFragmentFromArticle parses a fragment from an article HTML element
func parseFragmentFromArticle(articleHTML, resourceURL string) (*wire.Fragment, error) {
	fragment := &wire.Fragment{
		Spec:        "v0.2",
		FragmentURL: resourceURL,
	}

	// Extract publisher claim
	if idx := strings.Index(articleHTML, `data-la-publisher-claim="`); idx >= 0 {
		start := idx + len(`data-la-publisher-claim="`)
		end := strings.Index(articleHTML[start:], `"`)
		if end >= 0 {
			fragment.PublisherClaim = articleHTML[start : start+end]
		}
	}

	// Extract resource attestation URL
	if idx := strings.Index(articleHTML, `data-la-resource-attestation-url="`); idx >= 0 {
		start := idx + len(`data-la-resource-attestation-url="`)
		end := strings.Index(articleHTML[start:], `"`)
		if end >= 0 {
			fragment.ResourceAttestationURL = articleHTML[start : start+end]
		}
	}

	// Extract namespace attestation URL
	if idx := strings.Index(articleHTML, `data-la-namespace-attestation-url="`); idx >= 0 {
		start := idx + len(`data-la-namespace-attestation-url="`)
		end := strings.Index(articleHTML[start:], `"`)
		if end >= 0 {
			fragment.NamespaceAttestationURL = articleHTML[start : start+end]
		}
	}

	// Extract canonical content from href
	if idx := strings.Index(articleHTML, `href="data:text/html;base64,`); idx >= 0 {
		start := idx + len(`href="data:text/html;base64,`)
		end := strings.Index(articleHTML[start:], `"`)
		if end >= 0 {
			base64Content := articleHTML[start : start+end]
			canonicalBytes, err := base64.StdEncoding.DecodeString(base64Content)
			if err != nil {
				return nil, fmt.Errorf("failed to decode base64 content: %v", err)
			}
			fragment.CanonicalContent = canonicalBytes
			fragment.PreviewContent = string(canonicalBytes)
		}
	}

	// Validate required fields
	if fragment.PublisherClaim == "" {
		return nil, fmt.Errorf("missing data-la-publisher-claim")
	}
	if fragment.ResourceAttestationURL == "" {
		return nil, fmt.Errorf("missing data-la-resource-attestation-url")
	}
	if fragment.NamespaceAttestationURL == "" {
		return nil, fmt.Errorf("missing data-la-namespace-attestation-url")
	}
	if len(fragment.CanonicalContent) == 0 {
		return nil, fmt.Errorf("missing canonical content in href")
	}

	return fragment, nil
}

// fetchResourceAttestation fetches and parses a Resource Attestation
func fetchResourceAttestation(client *http.Client, url string) (*wire.ResourceAttestation, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch failed with status %d", resp.StatusCode)
	}

	var attestation wire.ResourceAttestation
	if err := json.NewDecoder(resp.Body).Decode(&attestation); err != nil {
		return nil, fmt.Errorf("invalid JSON in attestation: %v", err)
	}

	return &attestation, nil
}

// validateRequiredResourceAttestationFields validates that a Resource Attestation has all required fields
func validateRequiredResourceAttestationFields(attestation wire.ResourceAttestation) (*wire.ResourceAttestation, error) {
	// Validate required fields
	if attestation.FragmentURL == "" {
		return nil, fmt.Errorf("malformed attestation: missing fragment_url field")
	}
	if attestation.Hash == "" {
		return nil, fmt.Errorf("malformed attestation: missing hash field")
	}
	if attestation.PublisherClaim == "" {
		return nil, fmt.Errorf("malformed attestation: missing publisher_claim field")
	}
	if attestation.NamespaceAttestationURL == "" {
		return nil, fmt.Errorf("malformed attestation: missing namespace_attestation_url field")
	}

	return &attestation, nil
}

// fetchNamespaceAttestation fetches and parses a Namespace Attestation
func fetchNamespaceAttestation(client *http.Client, url string) (*wire.NamespaceAttestation, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch failed with status %d", resp.StatusCode)
	}

	var attestation wire.NamespaceAttestation
	if err := json.NewDecoder(resp.Body).Decode(&attestation); err != nil {
		return nil, fmt.Errorf("invalid JSON in attestation: %v", err)
	}

	// Validate required fields
	if attestation.Payload.Namespace == "" {
		return nil, fmt.Errorf("malformed attestation: missing payload.namespace field")
	}
	if attestation.Key == "" {
		return nil, fmt.Errorf("malformed attestation: missing key field")
	}
	if attestation.Sig == "" {
		return nil, fmt.Errorf("malformed attestation: missing sig field")
	}

	return &attestation, nil
}

// sameOrigin checks if two URLs have the same origin (scheme + host)
func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}