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

package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// HashSHA256 computes the SHA-256 digest of the provided bytes and returns the 32-byte digest.
func HashSHA256(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// HashSHA256Hex returns the lowercase hex encoding of the SHA-256 digest.
func HashSHA256Hex(data []byte) string {
	res := HashSHA256(data)
	return hex.EncodeToString(res[:])
}

// ComputeContentHashField returns the value for the payload Hash field: "sha256:<hex64>".
func ComputeContentHashField(data []byte) string {
	return "sha256:" + HashSHA256Hex(data)
}

// GenerateKeyPair creates a new secp256k1 private key and returns it along with its x-only public key (64 hex chars).
func GenerateKeyPair() (*btcec.PrivateKey, string, error) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, "", err
	}
	pub := priv.PubKey()
	// BIP-340 x-only serialization is 32 bytes X coordinate only
	xonly := schnorr.SerializePubKey(pub)
	return priv, hex.EncodeToString(xonly), nil
}

// ParseXOnlyPubKeyHex parses a 64-hex x-only public key into a btcec.PublicKey.
func ParseXOnlyPubKeyHex(hexKey string) (*btcec.PublicKey, error) {
	bytesKey, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, err
	}
	if len(bytesKey) != 32 {
		return nil, errors.New("x-only pubkey must be 32 bytes")
	}
	pk, err := schnorr.ParsePubKey(bytesKey)
	if err != nil {
		return nil, err
	}
	return pk, nil
}

// ParsePrivateKeyHex parses a 32-byte hex-encoded private key into a *btcec.PrivateKey.
func ParsePrivateKeyHex(hexKey string) (*btcec.PrivateKey, error) {
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, err
	}
	if len(b) != 32 {
		return nil, errors.New("private key must be 32 bytes")
	}
	priv, _ := btcec.PrivKeyFromBytes(b)
	return priv, nil
}

// SignSchnorrHex signs the 32-byte message digest with the provided private key and returns hex-encoded 64-byte signature.
func SignSchnorrHex(priv *btcec.PrivateKey, digest32 [32]byte) (string, error) {
	sig, err := schnorr.Sign(priv, digest32[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sig.Serialize()), nil
}

// VerifySchnorrHex verifies a hex-encoded signature against a hex x-only pubkey and 32-byte digest.
func VerifySchnorrHex(pubHex string, sigHex string, digest32 [32]byte) (bool, error) {
	pk, err := ParseXOnlyPubKeyHex(pubHex)
	if err != nil {
		return false, err
	}
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return false, err
	}
	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		return false, err
	}
	return sig.Verify(digest32[:], pk), nil
}

// RandomBytes returns n cryptographically secure random bytes.
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
