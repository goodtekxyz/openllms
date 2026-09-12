package health

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/goodtekxyz/openllms/internal/secrets"
	"github.com/goodtekxyz/openllms/internal/store"
	"github.com/goodtekxyz/openllms/internal/vendor"
	"github.com/google/uuid"
)

type Store interface {
	ListAllAccounts(ctx context.Context) ([]store.Account, error)
	MarkAccountHealthy(ctx context.Context, accountID uuid.UUID) error
	MarkAccountUnhealthy(ctx context.Context, accountID uuid.UUID, until time.Time) error
}

type Prober struct {
	Store   Store
	Secrets secrets.Client
	HTTP    *http.Client
	CoolFor time.Duration
}

func (p *Prober) client() *http.Client {
	if p.HTTP != nil {
		return p.HTTP
	}
	return &http.Client{Timeout: 8 * time.Second}
}

func (p *Prober) cool() time.Duration {
	if p.CoolFor > 0 {
		return p.CoolFor
	}
	return 60 * time.Second
}

// ProbeOnce checks each account's OpenAI-compatible /models endpoint.
func (p *Prober) ProbeOnce(ctx context.Context) (ok, bad int, err error) {
	if p.Store == nil || p.Secrets == nil {
		return 0, 0, nil
	}
	accounts, err := p.Store.ListAllAccounts(ctx)
	if err != nil {
		return 0, 0, err
	}
	for _, a := range accounts {
		if err := ctx.Err(); err != nil {
			return ok, bad, err
		}
		if probeAccount(ctx, p.client(), p.Secrets, &a) {
			_ = p.Store.MarkAccountHealthy(ctx, a.ID)
			ok++
		} else {
			_ = p.Store.MarkAccountUnhealthy(ctx, a.ID, time.Now().Add(p.cool()))
			bad++
		}
	}
	return ok, bad, nil
}

func (p *Prober) Start(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = 2 * time.Minute
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			_, _, _ = p.ProbeOnce(ctx)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

func probeAccount(ctx context.Context, client *http.Client, sec secrets.Client, a *store.Account) bool {
	raw, err := sec.Get(ctx, a.InfisicalPath, vendor.SecretName)
	if err != nil {
		return false
	}
	var cred secrets.CredentialJSON
	if json.Unmarshal([]byte(raw), &cred) != nil {
		return false
	}
	token := cred.BearerToken()
	if token == "" {
		return false
	}
	base := strings.TrimRight(a.BaseURL, "/")
	if base == "" {
		base = vendor.DefaultBaseURLFor(a.Vendor, a.AuthType)
	}
	url := base + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode > 0 && res.StatusCode < 500 && res.StatusCode != 401 && res.StatusCode != 403
}
