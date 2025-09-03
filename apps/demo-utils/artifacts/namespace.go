// Package artifacts provides demo utilities for LAP artifact management.
package artifacts

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/canonical"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/crypto"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/wire"
)

// CreateNamespaceAttestation creates a v0.2 Namespace Attestation
func CreateNamespaceAttestation(namespace, expStr, privHexFlag, outDir, keysDir string, rotate bool) (string, error) {
	// Parse or set expiration timestamp
	var exp int64
	var err error
	if expStr != "" {
		exp, err = strconv.ParseInt(expStr, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid exp: %w", err)
		}
	} else {
		// Default to 1 year from now
		exp = time.Now().AddDate(1, 0, 0).Unix()
	}

	// Get or generate private key
	var priv *btcec.PrivateKey
	var pubHex string

	if privHexFlag != "" {
		priv, err = crypto.ParsePrivateKeyHex(privHexFlag)
		if err != nil {
			return "", fmt.Errorf("invalid privkey: %w", err)
		}
		pub := priv.PubKey()
		pubHex = hex.EncodeToString(schnorr.SerializePubKey(pub))
	} else {
		// Check if this is for Alice's namespace and use her specific key
		if strings.Contains(namespace, "/people/alice/") {
			aliceKeyPath := filepath.Join(keysDir, "alice_publisher_key.json")
			if data, err := os.ReadFile(aliceKeyPath); err == nil {
				var stored StoredKey
				if json.Unmarshal(data, &stored) == nil {
					priv, err = crypto.ParsePrivateKeyHex(stored.PrivKeyHex)
					if err == nil {
						pubHex = stored.PubKeyXOnly
					}
				}
			}
		}
		
		// If not Alice or Alice key not found, try to load existing key from keys directory
		if priv == nil {
			keyPath := filepath.Join(keysDir, "namespace_key.json")
			if !rotate {
				if data, err := os.ReadFile(keyPath); err == nil {
					var stored StoredKey
					if json.Unmarshal(data, &stored) == nil {
						priv, err = crypto.ParsePrivateKeyHex(stored.PrivKeyHex)
						if err == nil {
							pubHex = stored.PubKeyXOnly
						}
					}
				}
			}

			// Generate new key if none exists or rotate requested
			if priv == nil {
				priv, pubHex, err = crypto.GenerateKeyPair()
				if err != nil {
					return "", fmt.Errorf("generate keypair: %w", err)
				}

				// Store the new key
				stored := StoredKey{
					PrivKeyHex:    hex.EncodeToString(priv.Serialize()),
					PubKeyXOnly:   pubHex,
					CreatedAtUnix: time.Now().Unix(),
				}
				if err := os.MkdirAll(keysDir, 0700); err != nil {
					return "", fmt.Errorf("mkdir %s: %w", keysDir, err)
				}
				if err := WriteJSON0600(keyPath, stored); err != nil {
					return "", fmt.Errorf("write %s: %w", keyPath, err)
				}
			}
		}
	}

	// Create v0.2 Namespace Attestation
	payload := wire.NamespacePayload{
		Namespace: namespace,
		Exp:       exp,
	}

	// Marshal to canonical JSON for signing
	payloadBytes, err := canonical.MarshalNamespacePayloadCanonical(payload.ToCanonical())
	if err != nil {
		return "", fmt.Errorf("canonical marshal: %w", err)
	}

	// Hash the payload
	digest := crypto.HashSHA256(payloadBytes)

	// Sign the digest
	sigHex, err := crypto.SignSchnorrHex(priv, digest)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}

	// Create the full attestation object
	attestation := wire.NamespaceAttestation{
		Payload: payload,
		Key:     pubHex,
		Sig:     sigHex,
	}

	// Determine output directory and path
	if outDir == "" {
		outDir = "."
	}
	
	// Create the full output path
	outputPath := filepath.Join(outDir, "_la_namespace.json")

	// Create parent directory if it doesn't exist
	parentDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", parentDir, err)
	}

	// Write the attestation
	if err := WriteJSON0600(outputPath, attestation); err != nil {
		return "", fmt.Errorf("write %s: %w", outputPath, err)
	}

	return outputPath, nil
}
