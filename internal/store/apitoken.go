package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// APIToken is a managed Bearer credential for scripts (the secret is never stored).
type APIToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	TokenHash  string     `json:"-"`
	CreatedBy  string     `json:"created_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// HashAPIToken is the stored form of a raw token (SHA-256 hex).
func HashAPIToken(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

// NewAPITokenSecret returns a one-time raw token, its hash, and a public prefix.
func NewAPITokenSecret() (raw, hash, prefix string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("generate api token: %w", err)
	}
	raw = "smdm_" + hex.EncodeToString(b)
	hash = HashAPIToken(raw)
	prefix = raw
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	return raw, hash, prefix, nil
}
