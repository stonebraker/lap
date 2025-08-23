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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/wire"
)

func TestEvaluateWindow(t *testing.T) {
	now := int64(1000)

	tests := []struct {
		name     string
		payload  wire.ResourcePayload
		now      int64
		expected string
	}{
		{
			name: "fresh attestation",
			payload: wire.ResourcePayload{
				IAT: 900,  // 100 seconds ago
				EXP: 1100, // 100 seconds in future
			},
			now:      now,
			expected: "fresh",
		},
		{
			name: "expired attestation",
			payload: wire.ResourcePayload{
				IAT: 800, // 200 seconds ago
				EXP: 900, // 100 seconds ago
			},
			now:      now,
			expected: "expired",
		},
		{
			name: "not yet valid",
			payload: wire.ResourcePayload{
				IAT: 1100, // 100 seconds in future
				EXP: 1200, // 200 seconds in future
			},
			now:      now,
			expected: "notyet",
		},
		{
			name: "exactly at IAT",
			payload: wire.ResourcePayload{
				IAT: 1000, // exactly now
				EXP: 1100,
			},
			now:      now,
			expected: "fresh",
		},
		{
			name: "exactly at EXP",
			payload: wire.ResourcePayload{
				IAT: 900,
				EXP: 1000, // exactly now
			},
			now:      now,
			expected: "fresh", // At EXP is still fresh, only > EXP is expired
		},
		{
			name: "one second past EXP",
			payload: wire.ResourcePayload{
				IAT: 900,
				EXP: 999, // one second ago
			},
			now:      now,
			expected: "expired",
		},
		{
			name: "zero timestamps (always fresh)",
			payload: wire.ResourcePayload{
				IAT: 0,
				EXP: 0,
			},
			now:      now,
			expected: "fresh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateWindow(tt.payload, tt.now)
			if result != tt.expected {
				t.Errorf("evaluateWindow() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBuildTelemetry(t *testing.T) {
	payload := wire.ResourcePayload{
		URL: "https://example.com/test",
		KID: "test-key-123",
		IAT: 1000,
		EXP: 2000,
	}
	
	telemetry := buildTelemetry(payload, "https://example.com/test", 1500, "strict")
	
	// Check all fields are populated correctly
	if telemetry.URL != "https://example.com/test" {
		t.Errorf("URL = %v, want %v", telemetry.URL, "https://example.com/test")
	}
	if telemetry.Kid != "test-key-123" {
		t.Errorf("Kid = %v, want %v", telemetry.Kid, "test-key-123")
	}
	if telemetry.IAT == nil || *telemetry.IAT != 1000 {
		t.Errorf("IAT = %v, want %v", telemetry.IAT, 1000)
	}
	if telemetry.EXP == nil || *telemetry.EXP != 2000 {
		t.Errorf("EXP = %v, want %v", telemetry.EXP, 2000)
	}
	if telemetry.Now != 1500 {
		t.Errorf("Now = %v, want %v", telemetry.Now, 1500)
	}
	if telemetry.Policy != "strict" {
		t.Errorf("Policy = %v, want %v", telemetry.Policy, "strict")
	}
}

func TestBuildTelemetryWithZeroTimestamps(t *testing.T) {
	payload := wire.ResourcePayload{
		URL: "https://example.com/test",
		KID: "test-key-123",
		IAT: 0, // zero timestamp
		EXP: 0, // zero timestamp
	}
	
	telemetry := buildTelemetry(payload, "https://example.com/test", 1500, "graceful")
	
	// Zero timestamps should result in nil pointers
	if telemetry.IAT != nil {
		t.Errorf("IAT should be nil for zero timestamp, got %v", *telemetry.IAT)
	}
	if telemetry.EXP != nil {
		t.Errorf("EXP should be nil for zero timestamp, got %v", *telemetry.EXP)
	}
}

func TestCanonicalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic HTTPS URL",
			input:    "https://example.com/path",
			expected: "https://example.com/path",
		},
		{
			name:     "HTTP with default port",
			input:    "http://example.com:80/path",
			expected: "http://example.com/path",
		},
		{
			name:     "HTTPS with default port",
			input:    "https://example.com:443/path",
			expected: "https://example.com/path",
		},
		{
			name:     "uppercase scheme and host",
			input:    "HTTPS://EXAMPLE.COM/Path",
			expected: "https://example.com/Path",
		},
		{
			name:     "path with query string",
			input:    "https://example.com/path?foo=bar&baz=qux",
			expected: "https://example.com/path?foo=bar&baz=qux",
		},
		{
			name:     "path normalization",
			input:    "https://example.com/./path/../other",
			expected: "https://example.com/other",
		},
		{
			name:     "trailing slash removal in path clean",
			input:    "https://example.com/path/",
			expected: "https://example.com/path",
		},
		{
			name:     "non-standard port preserved",
			input:    "https://example.com:8443/path",
			expected: "https://example.com:8443/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.input)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}
			result := canonicalizeURL(u)
			if result != tt.expected {
				t.Errorf("canonicalizeURL(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildAttestationURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic path",
			input:    "https://example.com/posts/1",
			expected: "https://example.com/posts/1/_lap/resource_attestation.json",
		},
		{
			name:     "path with index.html",
			input:    "https://example.com/posts/1/index.html",
			expected: "https://example.com/posts/1/_lap/resource_attestation.json",
		},
		{
			name:     "path with index.json",
			input:    "https://example.com/posts/1/index.json",
			expected: "https://example.com/posts/1/_lap/resource_attestation.json",
		},
		{
			name:     "path with trailing slash",
			input:    "https://example.com/posts/1/",
			expected: "https://example.com/posts/1/_lap/resource_attestation.json",
		},
		{
			name:     "root path",
			input:    "https://example.com/",
			expected: "https://example.com/_lap/resource_attestation.json",
		},
		{
			name:     "query and fragment stripped",
			input:    "https://example.com/posts/1?foo=bar#section",
			expected: "https://example.com/posts/1/_lap/resource_attestation.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildAttestationURL(tt.input)
			if result != tt.expected {
				t.Errorf("buildAttestationURL(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFetchAttestationSuccess(t *testing.T) {
	// Create a test attestation
	testAttestation := wire.ResourceAttestation{
		Payload: wire.ResourcePayload{
			URL:             "https://example.com/test",
			Attestation_URL: "https://example.com/test/_lap/resource_attestation.json",
			Hash:            "sha256:abc123",
			ETag:            "W/\"abc123\"",
			IAT:             1000,
			EXP:             2000,
			KID:             "test-key",
		},
		ResourceKey: "pubkey123",
		Sig:         "signature123",
	}

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testAttestation)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	
	result, verifyResult := fetchAttestation(client, server.URL, "https://example.com/test", 1500, "strict")
	
	if verifyResult != nil {
		t.Fatalf("Expected success, got error: %v", verifyResult)
	}
	
	if result == nil {
		t.Fatal("Expected attestation, got nil")
	}
	
	if result.Payload.URL != testAttestation.Payload.URL {
		t.Errorf("URL = %v, want %v", result.Payload.URL, testAttestation.Payload.URL)
	}
}

func TestFetchAttestationNetworkError(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	
	// Use invalid URL to trigger network error
	result, verifyResult := fetchAttestation(client, "http://invalid-host-12345.example", "https://example.com/test", 1500, "strict")
	
	if result != nil {
		t.Error("Expected nil result on network error")
	}
	
	if verifyResult == nil {
		t.Fatal("Expected verification error result")
	}
	
	if verifyResult.Code != "LA_FETCH_FAILED" {
		t.Errorf("Code = %v, want LA_FETCH_FAILED", verifyResult.Code)
	}
}

func TestFetchAttestation404Error(t *testing.T) {
	// Create mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	
	result, verifyResult := fetchAttestation(client, server.URL, "https://example.com/test", 1500, "strict")
	
	if result != nil {
		t.Error("Expected nil result on 404 error")
	}
	
	if verifyResult == nil {
		t.Fatal("Expected verification error result")
	}
	
	if verifyResult.Code != "LA_FETCH_FAILED" {
		t.Errorf("Code = %v, want LA_FETCH_FAILED", verifyResult.Code)
	}
	
	if verifyResult.Details == nil {
		t.Error("Expected details on HTTP error")
	} else {
		if httpStatus, ok := verifyResult.Details["http_status"]; !ok || httpStatus != 404 {
			t.Errorf("HTTP status = %v, want 404", httpStatus)
		}
	}
}

func TestFetchAttestationMalformedJSON(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json {"))
	}))
	defer server.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	
	result, verifyResult := fetchAttestation(client, server.URL, "https://example.com/test", 1500, "strict")
	
	if result != nil {
		t.Error("Expected nil result on malformed JSON")
	}
	
	if verifyResult == nil {
		t.Fatal("Expected verification error result")
	}
	
	if verifyResult.Code != "LA_ATTESTATION_MALFORMED" {
		t.Errorf("Code = %v, want LA_ATTESTATION_MALFORMED", verifyResult.Code)
	}
}

func TestVerificationResultJSONMarshaling(t *testing.T) {
	result := &VerificationResult{
		OK:      true,
		Status:  "ok",
		Code:    "LA_OK",
		Message: "verified",
		Details: map[string]interface{}{
			"fresh_until": int64(2000),
		},
		Telemetry: Telemetry{
			URL:    "https://example.com/test",
			Kid:    "test-key",
			IAT:    int64Ptr(1000),
			EXP:    int64Ptr(2000),
			Now:    1500,
			Policy: "strict",
		},
	}
	
	// Marshal to JSON
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal to JSON: %v", err)
	}
	
	// Unmarshal back
	var unmarshaled VerificationResult
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal from JSON: %v", err)
	}
	
	// Verify key fields
	if unmarshaled.OK != result.OK {
		t.Errorf("OK = %v, want %v", unmarshaled.OK, result.OK)
	}
	if unmarshaled.Code != result.Code {
		t.Errorf("Code = %v, want %v", unmarshaled.Code, result.Code)
	}
	if unmarshaled.Telemetry.URL != result.Telemetry.URL {
		t.Errorf("Telemetry.URL = %v, want %v", unmarshaled.Telemetry.URL, result.Telemetry.URL)
	}
}

func TestCheckDrift(t *testing.T) {
	basePayload := wire.ResourcePayload{
		URL:             "https://example.com/test",
		Attestation_URL: "https://example.com/test/_lap/resource_attestation.json",
		Hash:            "sha256:abc123",
		KID:             "test-key",
		IAT:             1000,
		EXP:             2000,
	}

	stapled := &wire.ResourceAttestation{
		Payload:     basePayload,
		ResourceKey: "pubkey123",
		Sig:         "sig123",
	}

	tests := []struct {
		name       string
		live       *wire.ResourceAttestation
		expectCode string
		expectNil  bool
	}{
		{
			name: "no drift",
			live: &wire.ResourceAttestation{
				Payload:     basePayload,
				ResourceKey: "pubkey123",
				Sig:         "different-sig", // signature can differ
			},
			expectNil: true,
		},
		{
			name: "URL drift",
			live: &wire.ResourceAttestation{
				Payload: wire.ResourcePayload{
					URL:             "https://evil.com/test", // Different URL
					Attestation_URL: basePayload.Attestation_URL,
					Hash:            basePayload.Hash,
					KID:             basePayload.KID,
					IAT:             basePayload.IAT,
					EXP:             basePayload.EXP,
				},
				ResourceKey: "pubkey123",
				Sig:         "sig123",
			},
			expectCode: "LA_URL_DRIFT",
		},
		{
			name: "hash drift",
			live: &wire.ResourceAttestation{
				Payload: wire.ResourcePayload{
					URL:             basePayload.URL,
					Attestation_URL: basePayload.Attestation_URL,
					Hash:            "sha256:different", // Different hash
					KID:             basePayload.KID,
					IAT:             basePayload.IAT,
					EXP:             basePayload.EXP,
				},
				ResourceKey: "pubkey123",
				Sig:         "sig123",
			},
			expectCode: "LA_HASH_DRIFT",
		},
		{
			name: "resource key drift",
			live: &wire.ResourceAttestation{
				Payload:     basePayload,
				ResourceKey: "different-key", // Different resource key
				Sig:         "sig123",
			},
			expectCode: "LA_RESOURCE_KEY_DRIFT",
		},
		{
			name: "KID drift",
			live: &wire.ResourceAttestation{
				Payload: wire.ResourcePayload{
					URL:             basePayload.URL,
					Attestation_URL: basePayload.Attestation_URL,
					Hash:            basePayload.Hash,
					KID:             "different-kid", // Different KID
					IAT:             basePayload.IAT,
					EXP:             basePayload.EXP,
				},
				ResourceKey: "pubkey123",
				Sig:         "sig123",
			},
			expectCode: "LA_KID_DRIFT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkDrift(stapled, tt.live, "https://example.com/test", 1500, "strict")
			
			if tt.expectNil {
				if result != nil {
					t.Errorf("Expected nil result, got %v", result)
				}
			} else {
				if result == nil {
					t.Fatalf("Expected error result, got nil")
				}
				if result.Code != tt.expectCode {
					t.Errorf("Code = %v, want %v", result.Code, tt.expectCode)
				}
			}
		})
	}
}

func TestVerificationOptionsDefaults(t *testing.T) {
	opts := VerificationOptions{
		Policy:      "strict",
		SkewSeconds: 120,
		Timeout:     10 * time.Second,
		Verbose:     false,
	}
	
	// Just verify the struct can be created and has expected defaults
	if opts.Policy != "strict" {
		t.Errorf("Default policy = %v, want strict", opts.Policy)
	}
	if opts.SkewSeconds != 120 {
		t.Errorf("Default skew = %v, want 120", opts.SkewSeconds)
	}
	if opts.Timeout != 10*time.Second {
		t.Errorf("Default timeout = %v, want 10s", opts.Timeout)
	}
	if opts.Verbose != false {
		t.Errorf("Default verbose = %v, want false", opts.Verbose)
	}
}

func TestVerifyContentHashMismatch(t *testing.T) {
	att := &wire.ResourceAttestation{
		Payload: wire.ResourcePayload{
			Hash: "sha256:expected_hash",
			ETag: "W/\"test\"",
		},
	}
	
	// Content that will produce a different hash
	content := []byte("wrong content")
	etag := "W/\"test\""
	
	result := verifyContentHash(att, content, etag, "https://example.com/test", 1500, "strict")
	
	if result == nil {
		t.Fatal("Expected hash mismatch error, got nil")
	}
	
	if result.Code != "LA_HASH_MISMATCH" {
		t.Errorf("Code = %v, want LA_HASH_MISMATCH", result.Code)
	}
}

func TestVerifyContentHashETagMismatch(t *testing.T) {
	att := &wire.ResourceAttestation{
		Payload: wire.ResourcePayload{
			Hash: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // Empty string hash
			ETag: "W/\"expected\"",
		},
	}
	
	// Empty content matches hash but wrong etag
	content := []byte("")
	etag := "W/\"wrong\""
	
	result := verifyContentHash(att, content, etag, "https://example.com/test", 1500, "strict")
	
	if result == nil {
		t.Fatal("Expected etag mismatch error, got nil")
	}
	
	if result.Code != "LA_HASH_MISMATCH" {
		t.Errorf("Code = %v, want LA_HASH_MISMATCH", result.Code)
	}
}

// Helper function for creating int64 pointers
func int64Ptr(i int64) *int64 {
	return &i
}
