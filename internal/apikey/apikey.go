package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const Prefix = "sk-gt-"

// Generate creates a new plaintext API key and its storage fields.
func Generate() (plaintext, prefix, hash string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(raw)
	plaintext = Prefix + secret
	prefix = plaintext[:min(12, len(plaintext))]
	hash = Hash(plaintext)
	return plaintext, prefix, hash, nil
}

func Hash(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func EqualHash(plaintext, storedHash string) bool {
	got := Hash(plaintext)
	return subtle.ConstantTimeCompare([]byte(got), []byte(storedHash)) == 1
}

func ParseBearer(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", fmt.Errorf("missing Authorization")
	}
	const p = "Bearer "
	if !strings.HasPrefix(header, p) {
		return "", fmt.Errorf("Authorization must be Bearer")
	}
	tok := strings.TrimSpace(header[len(p):])
	if tok == "" || !strings.HasPrefix(tok, Prefix) {
		return "", fmt.Errorf("invalid api key")
	}
	return tok, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
