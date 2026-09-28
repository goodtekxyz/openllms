//go:build !cloud

package httpserver

import (
	"net/http"
	"time"

	"github.com/goodtekxyz/openllms/internal/config"
	"github.com/goodtekxyz/openllms/internal/modelcatalog"
	"github.com/goodtekxyz/openllms/internal/proxy"
	"github.com/goodtekxyz/openllms/internal/ratelimit"
	"github.com/goodtekxyz/openllms/internal/secrets"
	"github.com/goodtekxyz/openllms/internal/store"
	"github.com/goodtekxyz/openllms/internal/userauth"
)

// New constructs the OSS gateway server (no Cloud overlay).
func New(st *store.Store, cfg config.Config, secret secrets.Client) *Server {
	userSecret := cfg.BillingSessionSecret
	if userSecret == "" {
		userSecret = cfg.AdminSessionSecret
	}
	user := &userauth.Manager{
		Secret: []byte(userSecret),
		TTL:    12 * time.Hour,
		Secure: cfg.AdminCookieSecure,
	}
	return &Server{
		cfg:           cfg,
		store:         st,
		secret:        secret,
		proxy:         &proxy.Engine{Store: st, Secrets: secret, HTTP: &http.Client{Timeout: 120 * time.Second}},
		limit:         ratelimit.NewMemory(cfg.RateLimitPerMin, time.Minute),
		models:        modelcatalog.New(secret, &http.Client{Timeout: 12 * time.Second}, modelcatalog.DefaultCacheTTL),
		admin:         disabledAdmin{},
		user:          user,
		mailer:        noopMailer{},
		vendorPending: newVendorPendingStore(),
	}
}
