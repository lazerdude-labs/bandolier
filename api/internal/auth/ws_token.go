package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazerdude-labs/bandolier/api/internal/vault"
)

// wsTokenPayload is the JSON-serialized portion of a WS auth token.
type wsTokenPayload struct {
	UserID int64 `json:"user_id"`
	Exp    int64 `json:"exp"` // unix seconds
}

// MintWSToken issues a short-lived HMAC-signed token for the given user.
// Format: base64url(payload).base64url(hmac-sha256(payload, key)). The
// payload is JSON {user_id, exp}. ttl is the validity window from now.
func MintWSToken(key []byte, userID int64, ttl time.Duration) (string, error) {
	body, err := json.Marshal(wsTokenPayload{
		UserID: userID,
		Exp:    time.Now().Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	pEnc := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(pEnc))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return pEnc + "." + sig, nil
}

// VerifyWSToken validates the token's HMAC and expiry, returning the user_id
// on success. Errors are returned without leaking which check failed (timing
// considerations less critical here than for, say, password verification).
var (
	errMalformedToken = errors.New("ws: malformed token")
	errInvalidSig     = errors.New("ws: invalid signature")
	errExpiredToken   = errors.New("ws: token expired")
)

func VerifyWSToken(key []byte, token string) (int64, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return 0, errMalformedToken
	}
	pEnc, sigEnc := parts[0], parts[1]
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(pEnc))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(sigEnc)) {
		return 0, errInvalidSig
	}
	body, err := base64.RawURLEncoding.DecodeString(pEnc)
	if err != nil {
		return 0, errMalformedToken
	}
	var p wsTokenPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return 0, errMalformedToken
	}
	if p.Exp <= time.Now().Unix() {
		return 0, errExpiredToken
	}
	return p.UserID, nil
}

// EnsureWSSigningKey reads the WS signing key from Vault at
// `auth/ws_signing_key`, generating + writing it on first call. Idempotent.
// Covers first-run AND existing post-Phase-4 stacks where setup ran without
// writing the key.
func EnsureWSSigningKey(ctx context.Context, v *vault.Client) ([]byte, error) {
	const path = "auth/ws_signing_key"
	if data, err := v.Get(ctx, path); err == nil && data != nil {
		if s, ok := data["key"].(string); ok && s != "" {
			b, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return nil, fmt.Errorf("decode ws signing key: %w", err)
			}
			return b, nil
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate ws signing key: %w", err)
	}
	if err := v.Put(ctx, path, map[string]any{
		"key": base64.StdEncoding.EncodeToString(key),
	}); err != nil {
		return nil, fmt.Errorf("persist ws signing key: %w", err)
	}
	return key, nil
}
