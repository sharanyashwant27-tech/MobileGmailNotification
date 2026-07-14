package crypto_test

import (
	"bytes"
	"testing"

	"github.com/yashs/mobile-gmail-notification/pkg/crypto"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	plain := []byte("ya29.oauth-access-token-never-a-password")

	cipher, err := crypto.Encrypt(key, plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Equal(cipher, plain) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	out, err := crypto.Decrypt(key, cipher)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("got %q want %q", out, plain)
	}
}

func TestEncryptRejectsBadKey(t *testing.T) {
	_, err := crypto.Encrypt([]byte("short"), []byte("data"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestHashTokenDeterministic(t *testing.T) {
	a := crypto.HashToken("refresh-token")
	b := crypto.HashToken("refresh-token")
	if a != b {
		t.Fatal("hash should be deterministic")
	}
	if a == crypto.HashToken("other") {
		t.Fatal("different inputs should hash differently")
	}
}

func TestRandomToken(t *testing.T) {
	a, err := crypto.RandomToken(16)
	if err != nil {
		t.Fatal(err)
	}
	b, err := crypto.RandomToken(16)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("random tokens should differ")
	}
	if len(a) != 32 {
		t.Fatalf("expected 32 hex chars, got %d", len(a))
	}
}
