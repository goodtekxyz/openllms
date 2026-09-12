package userauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const CookieName = "llms_user"

type Session struct {
	Login     string `json:"login"`
	GitHubID  string `json:"github_id"`
	ExpiresAt int64  `json:"exp"`
}

// Manager issues signed cookies for any GitHub user (billing / self-serve).
type Manager struct {
	Secret []byte
	TTL    time.Duration
	Secure bool
}

func (m *Manager) Enabled() bool {
	return len(m.Secret) > 0
}

func (m *Manager) Issue(w http.ResponseWriter, login, githubID string) error {
	if !m.Enabled() {
		return fmt.Errorf("user_session_disabled")
	}
	ttl := m.TTL
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	sess := Session{
		Login:     login,
		GitHubID:  githubID,
		ExpiresAt: time.Now().Add(ttl).Unix(),
	}
	raw, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, m.Secret)
	_, _ = mac.Write(raw)
	sig := mac.Sum(nil)
	val := base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(sig)
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   m.Secure,
		MaxAge:   int(ttl.Seconds()),
	})
	return nil
}

func (m *Manager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Secure:   m.Secure,
	})
}

func (m *Manager) Session(r *http.Request) (*Session, error) {
	if !m.Enabled() {
		return nil, fmt.Errorf("user_session_disabled")
	}
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return nil, fmt.Errorf("no_session")
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("bad_cookie")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("bad_cookie")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("bad_cookie")
	}
	mac := hmac.New(sha256.New, m.Secret)
	_, _ = mac.Write(raw)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, fmt.Errorf("bad_sig")
	}
	var sess Session
	if err := json.Unmarshal(raw, &sess); err != nil {
		return nil, err
	}
	if time.Now().Unix() > sess.ExpiresAt {
		return nil, fmt.Errorf("expired")
	}
	return &sess, nil
}
