package crypto

import "testing"

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
