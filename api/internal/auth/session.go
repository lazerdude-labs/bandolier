package auth

import (
	"crypto/rand"
	"encoding/hex"
)

func newSessionID() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

const SessionCookieName = "bandolier_session"
const SessionTTLSeconds = 8 * 60 * 60 // 8 hours
