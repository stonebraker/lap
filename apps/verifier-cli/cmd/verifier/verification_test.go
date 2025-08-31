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
	"strings"
	"testing"
	"time"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/wire"
)

// Test helper functions

func createTestFragment() *wire.Fragment {
	return &wire.Fragment{
		Spec:                        "v0.2",
		FragmentURL:                 "https://example.com/people/alice/posts/1",
		PreviewContent:              "<h2>Test Post</h2><p>This is a test post content.</p>",
		CanonicalContent:            []byte("<h2>Test Post</h2><p>This is a test post content.</p>"),
		PublisherClaim:              "d2c2d625b0ccaba90d1c80bed1dde31321695929b1472b9b8e21f5705c1d1410",
		ResourceAttestationURL:      "https://example.com/people/alice/posts/1/_la_resource.json",
		NamespaceAttestationURL:     "https://example.com/people/alice/_la_namespace.json",
	}
}

func createTestResourceAttestation() *wire.ResourceAttestation {
	return &wire.ResourceAttestation{
		FragmentURL:             "https://example.com/people/alice/posts/1",
		Hash:                    "sha256:b7c58676c2c8dbe5e611f780f79f7a37e20c0af84309106778212f0ee95f6eb9",
		PublisherClaim:          "d2c2d625b0ccaba90d1c80bed1dde31321695929b1472b9b8e21f5705c1d1410",
		NamespaceAttestationURL: "https://example.com/people/alice/_la_namespace.json",
	}
}

func createTestNamespaceAttestation() *wire.NamespaceAttestation {
	return &wire.NamespaceAttestation{
		Payload: wire.NamespacePayload{
			Namespace: "https://example.com/people/alice/",
			Exp:       time.Now().Add(24 * time.Hour).Unix(),
		},
		Key: "d2c2d625b0ccaba90d1c80bed1dde31321695929b1472b9b8e21f5705c1d1410",
		Sig: "08f5ed2e48b3d2367fd08628b524b59d34552b354a76c9016498c7b6fd7018f189388c24c51e1a94155b596d8217dbe688a2ee7a948c78d4938443b977a0e9eb",
	}
}

// Test cases

func TestVerifyResource_InvalidURL(t *testing.T) {
	opts := VerificationOptions{
		Timeout: 5 * time.Second,
		Verbose: false,
	}

	result, err := VerifyResource("invalid-url", opts)
	if err != nil {
		t.Fatalf("VerifyResource failed: %v", err)
	}

	// Should fail at resource_presence check
	if result.Verified {
		t.Error("Expected verification to fail")
	}

	if result.ResourcePresence != "fail" {
		t.Errorf("Expected resource_presence to be 'fail', got: %s", result.ResourcePresence)
	}

	if result.Failure == nil {
		t.Error("Expected failure details")
	} else {
		if result.Failure.Check != "resource_presence" {
			t.Errorf("Expected failure check to be 'resource_presence', got: %s", result.Failure.Check)
		}
		if result.Failure.Reason != "malformed" {
			t.Errorf("Expected failure reason to be 'malformed', got: %s", result.Failure.Reason)
		}
	}
}

func TestParseFragmentFromHTML(t *testing.T) {
	// Test HTML parsing with valid fragment
	html := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<article data-la-spec="v0.2" data-la-fragment-url="https://example.com/people/alice/posts/1">
<section class="la-preview">
<h2>Test Post</h2>
<p>This is a test post content.</p>
</section>
<link rel="canonical" type="text/html" 
data-la-publisher-claim="d2c2d625b0ccaba90d1c80bed1dde31321695929b1472b9b8e21f5705c1d1410"
data-la-resource-attestation-url="https://example.com/people/alice/posts/1/_la_resource.json"
data-la-namespace-attestation-url="https://example.com/people/alice/_la_namespace.json"
href="data:text/html;base64,PGgyPlRlc3QgUG9zdDwvaDI+PHA+VGhpcyBpcyBhIHRlc3QgcG9zdCBjb250ZW50LjwvcD4="
hidden />
</article>
</body>
</html>`

	fragment, err := parseFragmentFromHTML(html, "https://example.com/people/alice/posts/1")
	if err != nil {
		t.Fatalf("parseFragmentFromHTML failed: %v", err)
	}

	// Verify fragment fields
	if fragment.Spec != "v0.2" {
		t.Errorf("Expected spec to be 'v0.2', got: %s", fragment.Spec)
	}

	if fragment.FragmentURL != "https://example.com/people/alice/posts/1" {
		t.Errorf("Expected fragment URL to be 'https://example.com/people/alice/posts/1', got: %s", fragment.FragmentURL)
	}

	if fragment.PublisherClaim != "d2c2d625b0ccaba90d1c80bed1dde31321695929b1472b9b8e21f5705c1d1410" {
		t.Errorf("Expected publisher claim to match, got: %s", fragment.PublisherClaim)
	}

	if fragment.ResourceAttestationURL != "https://example.com/people/alice/posts/1/_la_resource.json" {
		t.Errorf("Expected resource attestation URL to match, got: %s", fragment.ResourceAttestationURL)
	}

	if fragment.NamespaceAttestationURL != "https://example.com/people/alice/_la_namespace.json" {
		t.Errorf("Expected namespace attestation URL to match, got: %s", fragment.NamespaceAttestationURL)
	}

	// Verify canonical content
	expectedContent := "<h2>Test Post</h2><p>This is a test post content.</p>"
	if string(fragment.CanonicalContent) != expectedContent {
		t.Errorf("Expected canonical content to match, got: %s", string(fragment.CanonicalContent))
	}

	if fragment.PreviewContent != expectedContent {
		t.Errorf("Expected preview content to match, got: %s", fragment.PreviewContent)
	}
}

func TestParseFragmentFromHTML_NoFragment(t *testing.T) {
	// Test HTML parsing without fragment
	html := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<h1>No fragment here</h1>
</body>
</html>`

	_, err := parseFragmentFromHTML(html, "https://example.com/people/alice/posts/1")
	if err == nil {
		t.Error("Expected error when no fragment found")
	}

	if !strings.Contains(err.Error(), "no fragment found with data-la-fragment-url attribute") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestParseFragmentFromHTML_MalformedFragment(t *testing.T) {
	// Test HTML parsing with malformed fragment (missing required attributes)
	html := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<article data-la-spec="v0.2" data-la-fragment-url="https://example.com/people/alice/posts/1">
<section class="la-preview">
<h2>Test Post</h2>
</section>
</article>
</body>
</html>`

	_, err := parseFragmentFromHTML(html, "https://example.com/people/alice/posts/1")
	if err == nil {
		t.Error("Expected error when fragment is malformed")
	}

	if !strings.Contains(err.Error(), "missing data-la-publisher-claim") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestVerificationOptions(t *testing.T) {
	opts := VerificationOptions{
		Timeout: 10 * time.Second,
		Verbose: true,
	}

	if opts.Timeout != 10*time.Second {
		t.Errorf("Expected timeout to be 10s, got: %v", opts.Timeout)
	}

	if !opts.Verbose {
		t.Error("Expected verbose to be true")
	}
}
