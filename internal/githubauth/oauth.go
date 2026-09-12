package githubauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	AuthorizeEndpoint = "https://github.com/login/oauth/authorize"
	TokenEndpoint     = "https://github.com/login/oauth/access_token"
	DefaultScope      = "read:user"
)

// AuthorizeURL builds the GitHub browser OAuth authorization URL.
func AuthorizeURL(clientID, redirectURI, state, scope string) string {
	if scope == "" {
		scope = DefaultScope
	}
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", scope)
	q.Set("state", state)
	return AuthorizeEndpoint + "?" + q.Encode()
}

// ExchangeCode trades an authorization code for an access token (confidential client).
func ExchangeCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (accessToken string, err error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	if redirectURI != "" {
		form.Set("redirect_uri", redirectURI)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var out struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", fmt.Errorf("token response not json (http %s): %s", res.Status, truncate(string(b), 200))
	}
	if out.Error != "" {
		if out.ErrorDescription != "" {
			return "", fmt.Errorf("%s: %s", out.Error, out.ErrorDescription)
		}
		return "", fmt.Errorf("%s", out.Error)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("no access_token: %s", truncate(string(b), 200))
	}
	return out.AccessToken, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
