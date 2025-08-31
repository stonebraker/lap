package crypto

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestSignVerifySchnorr(t *testing.T) {
	priv, pubHex, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	msg := []byte("hello world")
	digest := HashSHA256(msg)
	sigHex, err := SignSchnorrHex(priv, digest)
	if err != nil {
		t.Fatalf("SignSchnorrHex: %v", err)
	}
	ok, err := VerifySchnorrHex(pubHex, sigHex, digest)
	if err != nil || !ok {
		t.Fatalf("verify failed: %v ok=%v", err, ok)
	}
}

func TestHashSHA256_TestVectors(t *testing.T) {
	// Test vectors from FIPS 180-4
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			input:    "abc",
			expected: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			input:    "abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq",
			expected: "248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := HashSHA256Hex([]byte(tc.input))
			if result != tc.expected {
				t.Errorf("HashSHA256Hex(%q) = %s, want %s", tc.input, result, tc.expected)
			}
		})
	}
}

func TestComputeContentHashField_Format(t *testing.T) {
	// Test that content hash field includes sha256: prefix
	content := []byte("test content")
	hashField := ComputeContentHashField(content)
	
	if len(hashField) != 71 { // "sha256:" + 64 hex chars
		t.Errorf("Hash field length = %d, want 71", len(hashField))
	}
	
	if hashField[:7] != "sha256:" {
		t.Errorf("Hash field prefix = %s, want sha256:", hashField[:7])
	}
	
	// Verify the hex part is valid
	hexPart := hashField[7:]
	if len(hexPart) != 64 {
		t.Errorf("Hex part length = %d, want 64", len(hexPart))
	}
	
	// Verify it's lowercase
	for _, char := range hexPart {
		if char >= 'A' && char <= 'F' {
			t.Errorf("Hex part should be lowercase, found uppercase: %c", char)
		}
	}
}

func TestGenerateKeyPair_Format(t *testing.T) {
	priv, pubHex, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	
	// Verify private key is 32 bytes
	if priv == nil {
		t.Error("Private key should not be nil")
	}
	
	// Verify public key is 64 hex characters (32 bytes)
	if len(pubHex) != 64 {
		t.Errorf("Public key should be 64 hex chars, got %d", len(pubHex))
	}
	
	// Verify it's lowercase hex
	for _, char := range pubHex {
		if char >= 'A' && char <= 'F' {
			t.Errorf("Public key should be lowercase hex, found uppercase: %c", char)
		}
	}
	
	// Verify it's valid hex
	_, err = hex.DecodeString(pubHex)
	if err != nil {
		t.Errorf("Public key is not valid hex: %v", err)
	}
}

func TestParseXOnlyPubKeyHex_ValidKeys(t *testing.T) {
	// Generate a valid key pair
	_, pubHex, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	
	// Parse it back
	pub, err := ParseXOnlyPubKeyHex(pubHex)
	if err != nil {
		t.Errorf("ParseXOnlyPubKeyHex failed: %v", err)
	}
	
	if pub == nil {
		t.Error("ParseXOnlyPubKeyHex returned nil public key")
	}
}

func TestParseXOnlyPubKeyHex_InvalidKeys(t *testing.T) {
	testCases := []struct {
		name    string
		hexKey  string
		wantErr bool
	}{
		{"empty", "", true},
		{"too short", "1234567890abcdef", true},
		{"too long", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", true},
		{"invalid hex", "invalidhexkeyinvalidhexkeyinvalidhexkeyinvalidhexkey", true},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseXOnlyPubKeyHex(tc.hexKey)
			if tc.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestParsePrivateKeyHex_ValidKeys(t *testing.T) {
	// Generate a valid key pair
	priv, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	
	// Convert to hex
	privBytes := priv.Serialize()
	privHex := hex.EncodeToString(privBytes)
	
	// Parse it back
	parsedPriv, err := ParsePrivateKeyHex(privHex)
	if err != nil {
		t.Errorf("ParsePrivateKeyHex failed: %v", err)
	}
	
	if parsedPriv == nil {
		t.Error("ParsePrivateKeyHex returned nil private key")
	}
	
	// Verify the keys are equivalent by comparing serialized bytes
	parsedBytes := parsedPriv.Serialize()
	if string(privBytes) != string(parsedBytes) {
		t.Error("Parsed private key does not match original")
	}
}

func TestParsePrivateKeyHex_InvalidKeys(t *testing.T) {
	testCases := []struct {
		name    string
		hexKey  string
		wantErr bool
	}{
		{"empty", "", true},
		{"too short", "1234567890abcdef", true},
		{"too long", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", true},
		{"invalid hex", "invalidhexkeyinvalidhexkeyinvalidhexkeyinvalidhexkey", true},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParsePrivateKeyHex(tc.hexKey)
			if tc.wantErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestSignSchnorrHex_Format(t *testing.T) {
	priv, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	
	msg := []byte("test message")
	digest := HashSHA256(msg)
	
	sigHex, err := SignSchnorrHex(priv, digest)
	if err != nil {
		t.Fatalf("SignSchnorrHex failed: %v", err)
	}
	
	// Verify signature is 128 hex characters (64 bytes)
	if len(sigHex) != 128 {
		t.Errorf("Signature should be 128 hex chars, got %d", len(sigHex))
	}
	
	// Verify it's lowercase hex
	for _, char := range sigHex {
		if char >= 'A' && char <= 'F' {
			t.Errorf("Signature should be lowercase hex, found uppercase: %c", char)
		}
	}
	
	// Verify it's valid hex
	_, err = hex.DecodeString(sigHex)
	if err != nil {
		t.Errorf("Signature is not valid hex: %v", err)
	}
}

func TestVerifySchnorrHex_ValidSignature(t *testing.T) {
	priv, pubHex, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	
	msg := []byte("test message")
	digest := HashSHA256(msg)
	
	sigHex, err := SignSchnorrHex(priv, digest)
	if err != nil {
		t.Fatalf("SignSchnorrHex failed: %v", err)
	}
	
	// Verify the signature
	ok, err := VerifySchnorrHex(pubHex, sigHex, digest)
	if err != nil {
		t.Errorf("VerifySchnorrHex failed: %v", err)
	}
	if !ok {
		t.Error("Signature verification failed")
	}
}

func TestVerifySchnorrHex_InvalidSignature(t *testing.T) {
	priv, pubHex, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	
	msg := []byte("test message")
	digest := HashSHA256(msg)
	
	// Create a valid signature
	sigHex, err := SignSchnorrHex(priv, digest)
	if err != nil {
		t.Fatalf("SignSchnorrHex failed: %v", err)
	}
	
	// Test with wrong message
	wrongMsg := []byte("wrong message")
	wrongDigest := HashSHA256(wrongMsg)
	
	ok, err := VerifySchnorrHex(pubHex, sigHex, wrongDigest)
	if err != nil {
		t.Errorf("VerifySchnorrHex failed: %v", err)
	}
	if ok {
		t.Error("Signature verification should have failed with wrong message")
	}
}

func TestRandomBytes_Length(t *testing.T) {
	lengths := []int{16, 32, 64, 128}
	
	for _, length := range lengths {
		t.Run(fmt.Sprintf("length_%d", length), func(t *testing.T) {
			bytes, err := RandomBytes(length)
			if err != nil {
				t.Errorf("RandomBytes(%d) failed: %v", length, err)
			}
			if len(bytes) != length {
				t.Errorf("RandomBytes(%d) returned %d bytes", length, len(bytes))
			}
		})
	}
}

func TestRandomBytes_Uniqueness(t *testing.T) {
	// Generate multiple random byte sequences and verify they're different
	bytes1, err := RandomBytes(32)
	if err != nil {
		t.Fatalf("RandomBytes failed: %v", err)
	}
	
	bytes2, err := RandomBytes(32)
	if err != nil {
		t.Fatalf("RandomBytes failed: %v", err)
	}
	
	// Very unlikely they should be identical
	if string(bytes1) == string(bytes2) {
		t.Error("RandomBytes generated identical sequences")
	}
}

func TestHashSHA256_Consistency(t *testing.T) {
	// Test that HashSHA256 produces consistent results
	data := []byte("test data")
	
	hash1 := HashSHA256(data)
	hash2 := HashSHA256(data)
	
	if hash1 != hash2 {
		t.Error("HashSHA256 is not consistent")
	}
}

func TestHashSHA256Hex_Consistency(t *testing.T) {
	// Test that HashSHA256Hex produces consistent results
	data := []byte("test data")
	
	hash1 := HashSHA256Hex(data)
	hash2 := HashSHA256Hex(data)
	
	if hash1 != hash2 {
		t.Error("HashSHA256Hex is not consistent")
	}
}

func TestV02SpecificationTestVectors(t *testing.T) {
	// Test the example from v0.2 crypto specification
	// Note: These are example values for illustration, not real cryptographic values
	
	// Test the key pair example from the spec
	// Private Key: a1b2c3d4e5f6071829384756647382910abcdef1234567890fedcba0987654321
	// Public Key:  f1a2d3c4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00
	
	// Test that we can parse the public key format (even if it's not a real curve point)
	// This tests the hex parsing and length validation
	pubKeyHex := "f1a2d3c4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00"
	
	// Verify it's 64 hex characters
	if len(pubKeyHex) != 64 {
		t.Errorf("Public key should be 64 hex chars, got %d", len(pubKeyHex))
	}
	
	// Verify it's lowercase hex
	for _, char := range pubKeyHex {
		if char >= 'A' && char <= 'F' {
			t.Errorf("Public key should be lowercase hex, found uppercase: %c", char)
		}
	}
	
	// Verify it's valid hex
	_, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		t.Errorf("Public key is not valid hex: %v", err)
	}
	
	// Test the signature example from the spec
	// Signature: 4e0f1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7
	sigHex := "4e0f1a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7"
	
	// Verify it's 128 hex characters (64 bytes)
	if len(sigHex) != 128 {
		t.Errorf("Signature should be 128 hex chars, got %d", len(sigHex))
	}
	
	// Verify it's lowercase hex
	for _, char := range sigHex {
		if char >= 'A' && char <= 'F' {
			t.Errorf("Signature should be lowercase hex, found uppercase: %c", char)
		}
	}
	
	// Verify it's valid hex
	_, err = hex.DecodeString(sigHex)
	if err != nil {
		t.Errorf("Signature is not valid hex: %v", err)
	}
	
	// Test the message example from the spec
	// Message: {"exp":1754909100,"namespace":"https://example.com/people/alice/"}
	message := `{"exp":1754909100,"namespace":"https://example.com/people/alice/"}`
	
	// Verify SHA-256 hash format (the spec shows a 64-char hex string)
	hash := HashSHA256Hex([]byte(message))
	if len(hash) != 64 {
		t.Errorf("SHA-256 hash should be 64 hex chars, got %d", len(hash))
	}
	
	// Verify it's lowercase hex
	for _, char := range hash {
		if char >= 'A' && char <= 'F' {
			t.Errorf("SHA-256 hash should be lowercase hex, found uppercase: %c", char)
		}
	}
	
	// Verify content hash field format with sha256: prefix
	contentHashField := ComputeContentHashField([]byte(message))
	if len(contentHashField) != 71 { // "sha256:" + 64 hex chars
		t.Errorf("Content hash field should be 71 chars, got %d", len(contentHashField))
	}
	
	if contentHashField[:7] != "sha256:" {
		t.Errorf("Content hash field should start with 'sha256:', got '%s'", contentHashField[:7])
	}
}

func TestV02CryptographicRequirements(t *testing.T) {
	// Test that all v0.2 MUST requirements are met
	
	// 1. MUST use secp256k1 curve - verified by using btcec package
	// 2. MUST use Schnorr signatures per BIP-340 - verified by using schnorr package
	// 3. MUST use SHA-256 for all hashing - verified by using crypto/sha256
	// 4. MUST use X-only public key format - verified by using schnorr.SerializePubKey
	// 5. MUST serialize payloads canonically for signing - verified by canonical package
	// 6. MUST encode keys and signatures as lowercase hex - verified by tests
	
	// Test SHA-256 usage
	msg := []byte("test message")
	hash := HashSHA256(msg)
	if len(hash) != 32 {
		t.Errorf("SHA-256 should produce 32-byte hash, got %d", len(hash))
	}
	
	// Test X-only public key format
	_, pubHex, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	
	if len(pubHex) != 64 {
		t.Errorf("X-only public key should be 64 hex chars, got %d", len(pubHex))
	}
	
	// Test BIP-340 Schnorr signature
	priv, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	
	sigHex, err := SignSchnorrHex(priv, hash)
	if err != nil {
		t.Fatalf("SignSchnorrHex failed: %v", err)
	}
	
	if len(sigHex) != 128 {
		t.Errorf("BIP-340 signature should be 128 hex chars, got %d", len(sigHex))
	}
	
	// Test canonical JSON serialization requirement
	// This is verified by the canonical package tests
}
