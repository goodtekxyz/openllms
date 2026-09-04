package vendorauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestClaudeStartURL(t *testing.T) {
	p, err := ClaudeStart()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(p.AuthURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("code") != "true" || q.Get("client_id") != ClaudeClientID {
		t.Fatalf("query %v", q)
	}
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Fatal("pkce")
	}
	if p.Verifier == "" || p.State == "" {
		t.Fatal("session")
	}
	if p.State != p.Verifier {
		t.Fatalf("Anthropic requires state==verifier; state=%q verifier=%q", p.State, p.Verifier)
	}
	if q.Get("state") != p.Verifier {
		t.Fatalf("auth URL state %q != verifier", q.Get("state"))
	}
}

func TestClaudeExchange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/x-www-form-urlencoded") {
			t.Errorf("content-type %q", ct)
		}
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "abc" || r.Form.Get("state") != "st" {
			t.Errorf("form %v", r.Form)
		}
		if r.Form.Get("code_verifier") != "ver" || r.Form.Get("client_id") != ClaudeClientID {
			t.Errorf("form %v", r.Form)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at", "refresh_token": "rt", "expires_in": 3600,
		})
	}))
	defer srv.Close()
	prev := ClaudeTokenURL
	ClaudeTokenURL = srv.URL
	t.Cleanup(func() { ClaudeTokenURL = prev })
	HTTPClient = srv.Client()
	t.Cleanup(func() { HTTPClient = nil })

	tok, err := ClaudeExchange(context.Background(), "abc#st", "ver", "st")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "at" || tok.RefreshToken != "rt" {
		t.Fatalf("%+v", tok)
	}
}

func TestSplitClaudeCode(t *testing.T) {
	c, s := splitClaudeCode("  foo#bar  ")
	if c != "foo" || s != "bar" {
		t.Fatalf("%q %q", c, s)
	}
	c, s = splitClaudeCode("code=abc#xyz")
	if c != "abc" || s != "xyz" {
		t.Fatalf("code= %q %q", c, s)
	}
	c, s = splitClaudeCode("https://console.anthropic.com/oauth/code/callback?code=URLCODE&state=URLSTATE")
	if c != "URLCODE" || s != "URLSTATE" {
		t.Fatalf("url %q %q", c, s)
	}
	c, s = splitClaudeCode("abc&state=st2")
	if c != "abc" || s != "st2" {
		t.Fatalf("amp %q %q", c, s)
	}
}

func TestCodexDeviceStartAndPoll(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/usercode", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_auth_id": "did", "user_code": "AAAA-BBBB", "interval": 1,
		})
	})
	n := 0
	mux.HandleFunc("/poll", func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 1 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"authorization_code": "ac", "code_verifier": "cv",
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("code") != "ac" || r.Form.Get("code_verifier") != "cv" {
			t.Errorf("form %v", r.Form)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "codex-at", "refresh_token": "codex-rt"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	CodexDeviceUserCodeURL = srv.URL + "/usercode"
	CodexDeviceTokenURL = srv.URL + "/poll"
	CodexTokenURL = srv.URL + "/token"
	HTTPClient = srv.Client()
	CodexPollTimeout = time.Minute
	t.Cleanup(func() { HTTPClient = nil })

	p, err := CodexStart(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p.UserCode != "AAAA-BBBB" || p.DeviceAuthID != "did" {
		t.Fatalf("%+v", p)
	}
	p.Interval = time.Millisecond
	tok, err := CodexPollUntil(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "codex-at" {
		t.Fatalf("%+v", tok)
	}
}

func TestChatGPTAccountIDFromJWT(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "acct-1"},
	})
	jwt := "h." + base64.RawURLEncoding.EncodeToString(payload) + ".s"
	if chatgptAccountID(jwt) != "acct-1" {
		t.Fatal(chatgptAccountID(jwt))
	}
}
