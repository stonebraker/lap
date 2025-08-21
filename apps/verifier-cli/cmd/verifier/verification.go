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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/canonical"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/crypto"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/wire"
)

// VerificationResult matches the JavaScript verifier.core.js return format
type VerificationResult struct {
	OK        bool                   `json:"ok"`
	Status    string                 `json:"status"` // "ok", "warn", "error"
	Code      string                 `json:"code"`   // LA_OK, LA_SIG_INVALID, etc.
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Telemetry Telemetry              `json:"telemetry"`
}

type Telemetry struct {
	URL    string `json:"url"`
	Kid    string `json:"kid"`
	IAT    *int64 `json:"iat"`
	EXP    *int64 `json:"exp"`
	Now    int64  `json:"now"`
	Policy string `json:"policy"`
}

type VerificationOptions struct {
	Policy       string        // "strict", "graceful", "offline-fallback", "auto-refresh"
	SkewSeconds  int           // Grace period for expiration (default 120)
	LastKnownAt  *int64        // For offline-fallback policy
	Timeout      time.Duration // HTTP timeout
	Verbose      bool          // Debug output
}

// VerifyResource performs comprehensive LAP verification matching verifier.core.js logic
func VerifyResource(resourceURL string, opts VerificationOptions) (*VerificationResult, error) {
	now := time.Now().Unix()
	
	// Parse and validate URL
	origURL, err := url.Parse(resourceURL)
	if err != nil || origURL.Scheme == "" || origURL.Host == "" {
		return &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_ATTESTATION_MALFORMED",
			Message: "invalid resource URL",
			Telemetry: Telemetry{
				URL:    resourceURL,
				Now:    now,
				Policy: opts.Policy,
			},
		}, nil
	}

	client := &http.Client{
		Timeout: opts.Timeout,
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

	// Step 1: Fetch resource content
	resp, err := client.Get(origURL.String())
	if err != nil {
		return &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_FETCH_FAILED",
			Message: fmt.Sprintf("fetch failed: %v", err),
			Telemetry: Telemetry{
				URL:    resourceURL,
				Now:    now,
				Policy: opts.Policy,
			},
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_FETCH_FAILED",
			Message: "failed to read response body",
			Telemetry: Telemetry{
				URL:    resourceURL,
				Now:    now,
				Policy: opts.Policy,
			},
		}, nil
	}

	finalURL := resp.Request.URL
	etag := resp.Header.Get("ETag")

	// Step 2: Canonicalize URLs
	U := canonicalizeURL(finalURL)
	AU := buildAttestationURL(U)

	if opts.Verbose {
		fmt.Printf("Canonical URL: %s\n", U)
		fmt.Printf("Attestation URL: %s\n", AU)
	}

	// Step 3: Fetch stapled attestation (simulate - in real usage this would come from HTML)
	// For CLI, we'll fetch the live attestation and use it as both stapled and live
	stapled, result := fetchAttestation(client, AU, U, now, opts.Policy)
	if result != nil {
		return result, nil
	}

	// Step 4: Signature verification (matches verifier.core.js verifySignatureSchnorr)
	if result := verifySignature(stapled, U, now, opts.Policy); result != nil {
		return result, nil
	}

	// Step 5: Content hash verification (matches verifier.core.js verifyBytes)
	if result := verifyContentHash(stapled, body, etag, U, now, opts.Policy); result != nil {
		return result, nil
	}

	// Step 6: Fetch live attestation
	live, result := fetchAttestation(client, AU, U, now, opts.Policy)
	if result != nil {
		// Handle offline-fallback policy
		if opts.Policy == "offline-fallback" && opts.LastKnownAt != nil {
			return &VerificationResult{
				OK:     true,
				Status: "warn",
				Code:   "LA_OFFLINE_STALE",
				Message: "offline; showing last known good result",
				Details: map[string]interface{}{
					"cached_at": *opts.LastKnownAt,
				},
				Telemetry: buildTelemetry(stapled.Payload, U, now, opts.Policy),
			}, nil
		}
		return result, nil
	}

	// Step 7: Cross-attestation drift checks (matches verifier.core.js compareStapledVsFetched)
	if result := checkDrift(stapled, live, U, now, opts.Policy); result != nil {
		return result, nil
	}

	// Step 8: Freshness validation with policy handling (matches verifier.core.js evaluateWindow)
	return evaluateWindowWithPolicy(live, client, AU, U, now, opts)
}

func fetchAttestation(client *http.Client, attestationURL, resourceURL string, now int64, policy string) (*wire.Attestation, *VerificationResult) {
	resp, err := client.Get(attestationURL)
	if err != nil {
		return nil, &VerificationResult{
			OK:      false,
			Status:  "error", 
			Code:    "LA_FETCH_FAILED",
			Message: fmt.Sprintf("fetch failed: %v", err),
			Telemetry: Telemetry{
				URL:    resourceURL,
				Now:    now,
				Policy: policy,
			},
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_FETCH_FAILED", 
			Message: fmt.Sprintf("fetch failed: %d", resp.StatusCode),
			Details: map[string]interface{}{
				"http_status": resp.StatusCode,
				"phase":       "bundle",
			},
			Telemetry: Telemetry{
				URL:    resourceURL,
				Now:    now,
				Policy: policy,
			},
		}
	}

	var att wire.Attestation
	if err := json.NewDecoder(resp.Body).Decode(&att); err != nil {
		return nil, &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_ATTESTATION_MALFORMED",
			Message: "invalid JSON in attestation",
			Telemetry: Telemetry{
				URL:    resourceURL,
				Now:    now,
				Policy: policy,
			},
		}
	}

	// Check for malformed attestation (missing payload)
	if att.Payload.URL == "" {
		return nil, &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_ATTESTATION_MALFORMED", 
			Message: "malformed stapled object",
			Telemetry: Telemetry{
				URL:    resourceURL,
				Now:    now,
				Policy: policy,
			},
		}
	}

	return &att, nil
}

func verifySignature(att *wire.Attestation, resourceURL string, now int64, policy string) *VerificationResult {
	if att.ResourceKey == "" || att.Sig == "" {
		return &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_SIG_INVALID",
			Message: "signature verification failed",
			Details: map[string]interface{}{
				"kid": att.Payload.KID,
			},
			Telemetry: buildTelemetry(att.Payload, resourceURL, now, policy),
		}
	}

	// Canonical payload serialization (matches verifier.core.js canonicalizePayloadForSignature)
	payloadBytes, err := canonical.MarshalPayloadCanonical(att.Payload.ToCanonical())
	if err != nil {
		return &VerificationResult{
			OK:      false,
			Status:  "error", 
			Code:    "LA_SIG_INVALID",
			Message: "signature verification failed",
			Details: map[string]interface{}{
				"kid": att.Payload.KID,
			},
			Telemetry: buildTelemetry(att.Payload, resourceURL, now, policy),
		}
	}

	digest := crypto.HashSHA256(payloadBytes)
	ok, err := crypto.VerifySchnorrHex(att.ResourceKey, att.Sig, digest)
	if err != nil || !ok {
		return &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_SIG_INVALID", 
			Message: "signature verification failed",
			Details: map[string]interface{}{
				"kid": att.Payload.KID,
			},
			Telemetry: buildTelemetry(att.Payload, resourceURL, now, policy),
		}
	}

	return nil // Success
}

func verifyContentHash(att *wire.Attestation, content []byte, etag, resourceURL string, now int64, policy string) *VerificationResult {
	// Compute hash of content
	hasher := sha256.New()
	hasher.Write(content)
	computedHash := fmt.Sprintf("sha256:%x", hasher.Sum(nil))

	if att.Payload.Hash != computedHash {
		return &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_HASH_MISMATCH",
			Message: "hash mismatch",
			Telemetry: buildTelemetry(att.Payload, resourceURL, now, policy),
		}
	}

	// Optional ETag validation
	if etag != "" && att.Payload.ETag != etag {
		return &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_HASH_MISMATCH",
			Message: "etag mismatch", 
			Telemetry: buildTelemetry(att.Payload, resourceURL, now, policy),
		}
	}

	return nil // Success
}

func checkDrift(stapled, live *wire.Attestation, resourceURL string, now int64, policy string) *VerificationResult {
	sp := stapled.Payload
	lp := live.Payload

	// URL drift check
	if lp.URL != sp.URL {
		return &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_URL_DRIFT",
			Message: "payload.url mismatch (live vs stapled)",
			Details: map[string]interface{}{
				"expected": sp.URL,
				"actual":   lp.URL,
			},
			Telemetry: buildTelemetry(sp, resourceURL, now, policy),
		}
	}

	// Attestation URL drift check  
	if lp.Attestation_URL != sp.Attestation_URL {
		return &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_ATTESTATION_URL_DRIFT",
			Message: "attestation_url mismatch (live vs stapled)",
			Details: map[string]interface{}{
				"expected": sp.Attestation_URL,
				"actual":   lp.Attestation_URL,
			},
			Telemetry: buildTelemetry(sp, resourceURL, now, policy),
		}
	}

	// Hash drift check
	if lp.Hash != sp.Hash {
		return &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_HASH_DRIFT",
			Message: "hash mismatch (live vs stapled)",
			Details: map[string]interface{}{
				"expected_sha256": strings.TrimPrefix(sp.Hash, "sha256:"),
				"actual_sha256":   strings.TrimPrefix(lp.Hash, "sha256:"),
			},
			Telemetry: buildTelemetry(sp, resourceURL, now, policy),
		}
	}

	// Resource key drift check
	if live.ResourceKey != stapled.ResourceKey {
		return &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_RESOURCE_KEY_DRIFT",
			Message: "resource_key mismatch (live vs stapled)",
			Details: map[string]interface{}{
				"expected": stapled.ResourceKey,
				"actual":   live.ResourceKey,
			},
			Telemetry: buildTelemetry(sp, resourceURL, now, policy),
		}
	}

	// KID drift check
	if lp.KID != sp.KID {
		return &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_KID_DRIFT",
			Message: "kid mismatch (live vs stapled)",
			Details: map[string]interface{}{
				"expected": sp.KID,
				"actual":   lp.KID,
			},
			Telemetry: buildTelemetry(sp, resourceURL, now, policy),
		}
	}

	return nil // No drift detected
}

func evaluateWindowWithPolicy(live *wire.Attestation, client *http.Client, attestationURL, resourceURL string, now int64, opts VerificationOptions) (*VerificationResult, error) {
	state := evaluateWindow(live.Payload, now)
	
	// Handle "notyet" state
	if state == "notyet" {
		if opts.Policy == "auto-refresh" {
			// Try to refresh
			refreshed, result := fetchAttestation(client, attestationURL, resourceURL, now, opts.Policy)
			if result != nil {
				return result, nil
			}
			
			refreshState := evaluateWindow(refreshed.Payload, now)
			if refreshState == "fresh" {
				return &VerificationResult{
					OK:      true,
					Status:  "ok",
					Code:    "LA_OK",
					Message: "verified",
					Details: map[string]interface{}{
						"fresh_until": refreshed.Payload.EXP,
					},
					Telemetry: buildTelemetry(refreshed.Payload, resourceURL, now, opts.Policy),
				}, nil
			}
			
			return &VerificationResult{
				OK:      false,
				Status:  "error",
				Code:    "LA_EXPIRED_AFTER_REFRESH",
				Message: "attestation not yet valid after refresh",
				Telemetry: buildTelemetry(refreshed.Payload, resourceURL, now, opts.Policy),
			}, nil
		}
		
		return &VerificationResult{
			OK:      false,
			Status:  "error", 
			Code:    "LA_IAT_IN_FUTURE",
			Message: "live attestation not yet valid",
			Telemetry: buildTelemetry(live.Payload, resourceURL, now, opts.Policy),
		}, nil
	}

	// Handle "expired" state
	if state == "expired" {
		// Graceful policy with skew tolerance
		if opts.Policy == "graceful" && now <= live.Payload.EXP+int64(opts.SkewSeconds) {
			return &VerificationResult{
				OK:     true,
				Status: "warn",
				Code:   "LA_EXPIRED_GRACE",
				Message: "attestation slightly expired (grace)",
				Details: map[string]interface{}{
					"expired_at": live.Payload.EXP,
					"skew":       opts.SkewSeconds,
				},
				Telemetry: buildTelemetry(live.Payload, resourceURL, now, opts.Policy),
			}, nil
		}

		// Auto-refresh policy
		if opts.Policy == "auto-refresh" {
			refreshed, result := fetchAttestation(client, attestationURL, resourceURL, now, opts.Policy)
			if result != nil {
				return result, nil
			}
			
			refreshState := evaluateWindow(refreshed.Payload, now)
			if refreshState == "fresh" {
				return &VerificationResult{
					OK:      true,
					Status:  "ok",
					Code:    "LA_OK",
					Message: "verified",
					Details: map[string]interface{}{
						"fresh_until": refreshed.Payload.EXP,
					},
					Telemetry: buildTelemetry(refreshed.Payload, resourceURL, now, opts.Policy),
				}, nil
			}
			
			return &VerificationResult{
				OK:      false,
				Status:  "error",
				Code:    "LA_EXPIRED_AFTER_REFRESH",
				Message: "attestation expired after refresh",
				Telemetry: buildTelemetry(refreshed.Payload, resourceURL, now, opts.Policy),
			}, nil
		}

		// Hard expiration
		return &VerificationResult{
			OK:      false,
			Status:  "error",
			Code:    "LA_EXPIRED",
			Message: "live attestation expired",
			Telemetry: buildTelemetry(live.Payload, resourceURL, now, opts.Policy),
		}, nil
	}

	// Success - fresh attestation
	return &VerificationResult{
		OK:      true,
		Status:  "ok",
		Code:    "LA_OK",
		Message: "verified",
		Details: map[string]interface{}{
			"fresh_until": live.Payload.EXP,
		},
		Telemetry: buildTelemetry(live.Payload, resourceURL, now, opts.Policy),
	}, nil
}

// evaluateWindow matches the JavaScript version exactly
func evaluateWindow(p wire.Payload, now int64) string {
	if p.IAT != 0 && now < p.IAT {
		return "notyet"
	}
	if p.EXP != 0 && now > p.EXP {
		return "expired" 
	}
	return "fresh"
}

func buildTelemetry(payload wire.Payload, url string, now int64, policy string) Telemetry {
	var iat, exp *int64
	if payload.IAT != 0 {
		iat = &payload.IAT
	}
	if payload.EXP != 0 {
		exp = &payload.EXP
	}
	
	return Telemetry{
		URL:    url,
		Kid:    payload.KID,
		IAT:    iat,
		EXP:    exp,
		Now:    now,
		Policy: policy,
	}
}

// Helper functions (keep existing implementations)
func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func canonicalizeURL(u *url.URL) string {
	c := *u
	c.Scheme = strings.ToLower(c.Scheme)
	host := strings.ToLower(c.Host)
	if (c.Scheme == "http" && strings.HasSuffix(host, ":80")) || (c.Scheme == "https" && strings.HasSuffix(host, ":443")) {
		host = strings.Split(host, ":")[0]
	}
	c.Host = host
	c.Path = path.Clean("/" + c.Path)
	if u.RawQuery == "" {
		return c.Scheme + "://" + c.Host + c.Path
	}
	return c.Scheme + "://" + c.Host + c.Path + "?" + c.RawQuery
}

func buildAttestationURL(U string) string {
	u, err := url.Parse(U)
	if err != nil {
		return U
	}
	p := u.Path
	p = strings.TrimSuffix(p, "/index.html")
	p = strings.TrimSuffix(p, "/index.json")
	p = strings.TrimSuffix(p, "/")
	u.Path = p + "/_lap/resource_attestation.json"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
