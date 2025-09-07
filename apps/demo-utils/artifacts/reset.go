// Package artifacts provides demo utilities for LAP artifact management.
package artifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/canonical"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/crypto"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/wire"
)

// ResetArtifacts resets all LAP artifacts for Alice's posts
func ResetArtifacts(base, root, keysDir string) error {
	// Load the publisher key from the keys directory
	aliceKeyPath := filepath.Join(keysDir, "alice_publisher_key.json")
	
	var publisherKey string
	var privateKey string
	if data, err := os.ReadFile(aliceKeyPath); err == nil {
		var stored StoredKey
		if json.Unmarshal(data, &stored) == nil && stored.PubKeyXOnly != "" {
			publisherKey = stored.PubKeyXOnly
			privateKey = stored.PrivKeyHex
		}
	}
	
	if publisherKey == "" || privateKey == "" {
		return fmt.Errorf("could not load publisher key from %s - please create this key first using: lapctl keygen -name alice -out %s", aliceKeyPath, aliceKeyPath)
	}

	// Step 1: Create new namespace attestation
	fmt.Fprintf(os.Stderr, "Creating new namespace attestation...\n")
	namespaceAttestationURL := fmt.Sprintf("%s/people/alice/_la_namespace.json", base)
	
	// Parse private key
	priv, err := crypto.ParsePrivateKeyHex(privateKey)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}
	
	// Create v0.2 Namespace Attestation
	payload := wire.NamespacePayload{
		Namespace: fmt.Sprintf("%s/people/alice/", base),
		Exp:       time.Now().AddDate(1, 0, 0).Unix(),
	}

	// Marshal to canonical JSON for signing
	payloadBytes, err := canonical.MarshalNamespacePayloadCanonical(payload.ToCanonical())
	if err != nil {
		return fmt.Errorf("canonical marshal: %w", err)
	}

	// Hash the payload
	digest := crypto.HashSHA256(payloadBytes)

	// Sign the digest
	sigHex, err := crypto.SignSchnorrHex(priv, digest)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	// Create the full attestation object
	attestation := wire.NamespaceAttestation{
		Payload: payload,
		Key:     publisherKey,
		Sig:     sigHex,
	}

	// Write the namespace attestation
	// If root ends with "frc", go up one level to place NA at alice level
	var naOutputPath string
	if strings.HasSuffix(root, "frc") {
		naOutputPath = filepath.Join(filepath.Dir(root), "_la_namespace.json")
	} else {
		naOutputPath = filepath.Join(root, "_la_namespace.json")
	}
	if err := os.MkdirAll(filepath.Dir(naOutputPath), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(naOutputPath), err)
	}
	
	if err := WriteJSON0600(naOutputPath, attestation); err != nil {
		return fmt.Errorf("write %s: %w", naOutputPath, err)
	}
	
	fmt.Fprintf(os.Stderr, "Created namespace attestation at %s\n", naOutputPath)
	fmt.Fprintf(os.Stderr, "Valid until %s\n", time.Unix(payload.Exp, 0).Format(time.RFC3339))

	// Step 2: Process each post
	fmt.Fprintf(os.Stderr, "Updating posts 1..3...\n")
	
	// Process each post
	for postNum := 1; postNum <= 3; postNum++ {
		postDir := filepath.Join(root, "posts", strconv.Itoa(postNum))
		inPath := filepath.Join(postDir, "content.htmx")
		outPath := filepath.Join(postDir, "index.htmx")
		
		// Construct URLs for this post
		fragmentURL := fmt.Sprintf("%s/people/alice/frc/posts/%d", base, postNum)
		resourceAttestationURL := fmt.Sprintf("%s/people/alice/frc/posts/%d/_la_resource.json", base, postNum)
		
		// Generate resource attestation first
		fmt.Fprintf(os.Stderr, "generating resource attestation for post %d...\n", postNum)
		raOutputPath := filepath.Join(postDir, "_la_resource.json")
		err := CreateResourceAttestation(inPath, fragmentURL, "", publisherKey, namespaceAttestationURL, raOutputPath)
		if err != nil {
			return fmt.Errorf("error generating RA for post %d: %w", postNum, err)
		}
		
		// Generate fragment
		fmt.Fprintf(os.Stderr, "generating fragment for post %d...\n", postNum)
		err = CreateFragment(inPath, fragmentURL, "", publisherKey, resourceAttestationURL, namespaceAttestationURL, outPath)
		if err != nil {
			return fmt.Errorf("error generating fragment for post %d: %w", postNum, err)
		}
	}
	
	// Step 3: Update the host file with all three fragments
	hostPath := filepath.Join(root, "posts", "index.htmx")
	if _, err := os.Stat(hostPath); err == nil {
		fmt.Fprintf(os.Stderr, "updating host file %s...\n", hostPath)
		
		hostHTML, err := os.ReadFile(hostPath)
		if err != nil {
			return fmt.Errorf("read host file: %w", err)
		}
		
		// Read each fragment and update the host file
		for postNum := 1; postNum <= 3; postNum++ {
			fragmentPath := filepath.Join(root, "posts", strconv.Itoa(postNum), "index.htmx")
			fragmentData, err := os.ReadFile(fragmentPath)
			if err != nil {
				return fmt.Errorf("read fragment %d: %w", postNum, err)
			}
			
			// The fragment is HTML, not JSON, so we can use it directly
			replacementHTML := string(fragmentData)
			
			// Update host file
			fragmentURL := fmt.Sprintf("%s/people/alice/frc/posts/%d", base, postNum)
			updatedHTML, updated := ReplaceArticleByDataLaFragmentURL(string(hostHTML), fragmentURL, replacementHTML)
			if updated {
				hostHTML = []byte(updatedHTML)
				fmt.Fprintf(os.Stderr, "updated post %d in host file\n", postNum)
			} else {
				fmt.Fprintf(os.Stderr, "warning: could not find post %d in host file\n", postNum)
			}
		}
		
		// Create backup before writing updated host file
		if err := os.WriteFile(hostPath+".bak", hostHTML, 0644); err != nil {
			return fmt.Errorf("error creating backup: %w", err)
		}
		
		// Write updated host file
		if err := os.WriteFile(hostPath, hostHTML, 0644); err != nil {
			return fmt.Errorf("write host file: %w", err)
		}
		
		fmt.Fprintf(os.Stderr, "successfully updated host file %s\n", hostPath)
	} else {
		fmt.Fprintf(os.Stderr, "host file %s not found, skipping host update\n", hostPath)
	}
	
	fmt.Fprintf(os.Stderr, "Successfully reset all LAP artifacts for alice\n")
	return nil
}
