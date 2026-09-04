package secrets

import "context"

// Client stores upstream credentials out-of-band (Infisical or local file vault).
// DB holds paths only.
type Client interface {
	Put(ctx context.Context, path, name, value string) error
	Get(ctx context.Context, path, name string) (string, error)
	Delete(ctx context.Context, path, name string) error
}

// CacheInvalidator drops a process-local secret cache entry so the next Get
// hits the durable store. Optional — memory/file clients need not implement it.
type CacheInvalidator interface {
	Invalidate(path, name string)
}

// CredentialJSON is the Infisical secret value for accounts.
type CredentialJSON struct {
	APIKey           string `json:"api_key,omitempty"`
	AccessToken      string `json:"access_token,omitempty"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
	IDToken          string `json:"id_token,omitempty"`
	ChatGPTAccountID string `json:"chatgpt_account_id,omitempty"`
}

func (c CredentialJSON) BearerToken() string {
	if c.APIKey != "" {
		return c.APIKey
	}
	return c.AccessToken
}
