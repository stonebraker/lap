// Package artifacts provides demo utilities for LAP artifact management.
package artifacts

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/crypto"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/wire"
)

// CreateResourceAttestation creates a v0.2 Resource Attestation for the given content
func CreateResourceAttestation(inPath, resURL, base, publisherClaim, namespaceAttestationURL, outPath string) error {
	// Read input file
	body, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inPath, err)
	}

	// Build payload URL with optional base override
	var u url.URL
	if base != "" {
		baseURL, err := url.Parse(base)
		if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
			return fmt.Errorf("invalid base: %s", base)
		}
		u = *baseURL
		// take path+query from resURL (absolute or relative)
		rawU, err := url.Parse(resURL)
		if err != nil {
			return fmt.Errorf("invalid url: %s", resURL)
		}
		if rawU.Path != "" {
			u.Path = rawU.Path
		}
		u.RawQuery = rawU.RawQuery
	} else {
		rawU, err := url.Parse(resURL)
		if err != nil || rawU.Scheme == "" || rawU.Host == "" {
			return fmt.Errorf("invalid url (expect absolute when base not set): %s", resURL)
		}
		u = *rawU
	}

	// Canonicalize scheme/host (lower) and strip default ports
	u.Scheme = strings.ToLower(u.Scheme)
	hu := strings.ToLower(u.Host)
	if (u.Scheme == "http" && strings.HasSuffix(hu, ":80")) || (u.Scheme == "https" && strings.HasSuffix(hu, ":443")) {
		hu = strings.Split(hu, ":")[0]
	}
	u.Host = hu
	payloadURL := u.String()

	// Create v0.2 Resource Attestation
	att := wire.ResourceAttestation{
		FragmentURL:             payloadURL,
		Hash:                    crypto.ComputeContentHashField(body),
		PublisherClaim:          publisherClaim,
		NamespaceAttestationURL: namespaceAttestationURL,
	}

	// Determine output path
	if outPath == "" {
		dir := filepath.Dir(inPath)
		outPath = filepath.Join(dir, "_la_resource.json")
	}

	// Create output directory and file
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	
	return WriteJSON0600(outPath, att)
}
