package auth

import (
	"strings"
	"testing"
	"time"
)

var testKey = []byte("01234567890123456789012345678901") // 32 bytes

func TestMintAndVerifyRoundtrip(t *testing.T) {
	tok, err := MintWSToken(testKey, 42, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	uid, err := VerifyWSToken(testKey, tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if uid != 42 {
		t.Fatalf("uid = %d, want 42", uid)
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	tok, err := MintWSToken(testKey, 42, -10*time.Second) // already expired
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyWSToken(testKey, tok); err == nil {
		t.Fatal("expected expired error, got nil")
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	tok, err := MintWSToken(testKey, 42, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	// flip a bit in the payload section
	parts := strings.Split(tok, ".")
	if len(parts) != 2 {
		t.Fatalf("token format: %s", tok)
	}
	payload := []byte(parts[0])
	payload[0] ^= 0x01
	tampered := string(payload) + "." + parts[1]
	if _, err := VerifyWSToken(testKey, tampered); err == nil {
		t.Fatal("expected tamper error, got nil")
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	tok, err := MintWSToken(testKey, 42, 60*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	tampered := tok[:len(tok)-1] + "X"
	if _, err := VerifyWSToken(testKey, tampered); err == nil {
		t.Fatal("expected sig error, got nil")
	}
}

func TestVerifyRejectsMalformedToken(t *testing.T) {
	if _, err := VerifyWSToken(testKey, "not-a-token"); err == nil {
		t.Fatal("expected malformed error, got nil")
	}
}
