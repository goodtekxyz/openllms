package vendorauth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// Tokens is the oauth material stored via the gateway (Infisical).
type Tokens struct {
	AccessToken      string
	RefreshToken     string
	IDToken          string
	ExpiresAt        string
	ChatGPTAccountID string
}

func tokensFromOAuthJSON(b []byte) (Tokens, error) {
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return Tokens{}, err
	}
	if out.Error != "" {
		msg := out.Error
		if out.ErrorDesc != "" {
			msg += ": " + out.ErrorDesc
		}
		return Tokens{}, errString(msg)
	}
	if out.AccessToken == "" {
		return Tokens{}, errString("no access_token in oauth response")
	}
	t := Tokens{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		IDToken:      out.IDToken,
	}
	if out.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().UTC().Add(time.Duration(out.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	if id := chatgptAccountID(out.IDToken); id != "" {
		t.ChatGPTAccountID = id
	} else {
		t.ChatGPTAccountID = chatgptAccountID(out.AccessToken)
	}
	return t, nil
}

type errString string

func (e errString) Error() string { return string(e) }

func chatgptAccountID(jwt string) string {
	payload := jwtPayload(jwt)
	if payload == nil {
		return ""
	}
	auth, _ := payload["https://api.openai.com/auth"].(map[string]any)
	if auth == nil {
		return ""
	}
	id, _ := auth["chatgpt_account_id"].(string)
	return strings.TrimSpace(id)
}

func jwtPayload(jwt string) map[string]any {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return nil
	}
	seg := parts[1]
	if m := len(seg) % 4; m != 0 {
		seg += strings.Repeat("=", 4-m)
	}
	raw, err := base64.URLEncoding.DecodeString(seg)
	if err != nil {
		return nil
	}
	var out map[string]any
	if json.Unmarshal(raw, &out) != nil {
		return nil
	}
	return out
}
