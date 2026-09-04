package vendorauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Public Codex CLI OAuth client (no secret).
const CodexClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

var (
	CodexDeviceUserCodeURL  = "https://auth.openai.com/api/accounts/deviceauth/usercode"
	CodexDeviceTokenURL     = "https://auth.openai.com/api/accounts/deviceauth/token"
	CodexDeviceVerifyURL    = "https://auth.openai.com/codex/device"
	CodexTokenURL           = "https://auth.openai.com/oauth/token"
	CodexDeviceRedirectURI  = "https://auth.openai.com/deviceauth/callback"
	CodexPollTimeout        = 15 * time.Minute
	CodexDefaultPollSeconds = 5
)

type CodexPending struct {
	UserCode     string
	DeviceAuthID string
	VerifyURL    string
	Interval     time.Duration
}

func CodexStart(ctx context.Context) (CodexPending, error) {
	body, _ := json.Marshal(map[string]string{"client_id": CodexClientID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, CodexDeviceUserCodeURL, bytes.NewReader(body))
	if err != nil {
		return CodexPending{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := httpClient().Do(req)
	if err != nil {
		return CodexPending{}, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return CodexPending{}, fmt.Errorf("codex device code: %s %s", res.Status, truncate(b, 300))
	}
	var out struct {
		DeviceAuthID string          `json:"device_auth_id"`
		UserCode     string          `json:"user_code"`
		UserCodeAlt  string          `json:"usercode"`
		Interval     json.RawMessage `json:"interval"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return CodexPending{}, err
	}
	code := strings.TrimSpace(out.UserCode)
	if code == "" {
		code = strings.TrimSpace(out.UserCodeAlt)
	}
	if code == "" || strings.TrimSpace(out.DeviceAuthID) == "" {
		return CodexPending{}, fmt.Errorf("codex device flow missing user_code/device_auth_id")
	}
	return CodexPending{
		UserCode:     code,
		DeviceAuthID: strings.TrimSpace(out.DeviceAuthID),
		VerifyURL:    CodexDeviceVerifyURL,
		Interval:     parseInterval(out.Interval),
	}, nil
}

func CodexPollUntil(ctx context.Context, p CodexPending) (Tokens, error) {
	interval := p.Interval
	if interval <= 0 {
		interval = time.Duration(CodexDefaultPollSeconds) * time.Second
	}
	fmt.Fprintln(os.Stderr, "Waiting for ChatGPT authorization — after the code, click Authorize.")
	deadline := time.Now().Add(CodexPollTimeout)
	lastNote := time.Time{}
	next := time.Now().Add(interval)
	for {
		if time.Now().After(deadline) {
			return Tokens{}, fmt.Errorf("codex device code expired — run llms add again")
		}
		if wait := time.Until(next); wait > 0 {
			select {
			case <-ctx.Done():
				return Tokens{}, ctx.Err()
			case <-time.After(wait):
			}
		}
		tok, pending, err := codexPollOnce(ctx, p)
		if err != nil {
			return Tokens{}, err
		}
		if !pending {
			return tok, nil
		}
		if time.Since(lastNote) >= 15*time.Second {
			fmt.Fprintln(os.Stderr, "Still waiting for Authorize on ChatGPT…")
			lastNote = time.Now()
		}
		next = time.Now().Add(interval)
	}
}

func CodexPollOnce(ctx context.Context, p CodexPending) (Tokens, bool, error) {
	return codexPollOnce(ctx, p)
}

func codexPollOnce(ctx context.Context, p CodexPending) (Tokens, bool, error) {
	body, _ := json.Marshal(map[string]string{
		"device_auth_id": p.DeviceAuthID,
		"user_code":      p.UserCode,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, CodexDeviceTokenURL, bytes.NewReader(body))
	if err != nil {
		return Tokens{}, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := httpClient().Do(req)
	if err != nil {
		return Tokens{}, false, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusNotFound {
		return Tokens{}, true, nil
	}
	if res.StatusCode >= 300 {
		return Tokens{}, false, fmt.Errorf("codex device poll: %s %s", res.Status, truncate(b, 300))
	}
	var out struct {
		AuthorizationCode string `json:"authorization_code"`
		CodeVerifier      string `json:"code_verifier"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return Tokens{}, false, err
	}
	if strings.TrimSpace(out.AuthorizationCode) == "" || strings.TrimSpace(out.CodeVerifier) == "" {
		return Tokens{}, true, nil
	}
	tok, err := codexExchange(ctx, out.AuthorizationCode, out.CodeVerifier)
	return tok, false, err
}

func codexExchange(ctx context.Context, code, verifier string) (Tokens, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", CodexClientID)
	form.Set("code", code)
	form.Set("redirect_uri", CodexDeviceRedirectURI)
	form.Set("code_verifier", verifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, CodexTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := httpClient().Do(req)
	if err != nil {
		return Tokens{}, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return Tokens{}, fmt.Errorf("codex token: %s %s", res.Status, truncate(b, 300))
	}
	return tokensFromOAuthJSON(b)
}

func parseInterval(raw json.RawMessage) time.Duration {
	def := time.Duration(CodexDefaultPollSeconds) * time.Second
	if len(raw) == 0 {
		return def
	}
	var asString string
	if json.Unmarshal(raw, &asString) == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(asString)); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	var asInt int
	if json.Unmarshal(raw, &asInt) == nil && asInt > 0 {
		return time.Duration(asInt) * time.Second
	}
	return def
}

func truncate(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n]
	}
	return s
}

func httpClient() *http.Client {
	if HTTPClient != nil {
		return HTTPClient
	}
	return http.DefaultClient
}

// HTTPClient is overridable in tests.
var HTTPClient *http.Client
