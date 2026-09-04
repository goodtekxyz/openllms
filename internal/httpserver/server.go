package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/goodtekxyz/openllms/internal/authn"
	"github.com/goodtekxyz/openllms/internal/config"
	"github.com/goodtekxyz/openllms/internal/githubauth"
	"github.com/goodtekxyz/openllms/internal/modelcatalog"
	"github.com/goodtekxyz/openllms/internal/proxy"
	"github.com/goodtekxyz/openllms/internal/quota"
	"github.com/goodtekxyz/openllms/internal/ratelimit"
	routerpkg "github.com/goodtekxyz/openllms/internal/router"
	"github.com/goodtekxyz/openllms/internal/secrets"
	"github.com/goodtekxyz/openllms/internal/store"
	"github.com/goodtekxyz/openllms/internal/userauth"
	"github.com/goodtekxyz/openllms/internal/vendor"
	"github.com/google/uuid"
)

type Server struct {
		cfg            config.Config
	store          *store.Store
	secret         secrets.Client
	proxy          *proxy.Engine
	limit          *ratelimit.MemoryLimiter
	models         *modelcatalog.Discoverer
	admin          AdminAuth
	user           *userauth.Manager
	mailer         Mailer
	vendorPending  *vendorPendingStore
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)
	r.Get("/control/v1/meta", s.handleMeta)
	r.Get("/auth/github", s.handleGitHubOAuthStart)
	r.Get("/auth/github/callback", s.handleGitHubOAuthCallback)
	// Public product + install (no auth) — / and /install negotiate Accept-Language → /{lang}
	r.Get("/", s.handlePublicLanding)
	r.Get("/install", s.handlePublicInstallHTML)
	r.Get("/install.sh", s.handlePublicInstallSh)
	r.Get("/install.md", s.handlePublicInstallDoc)
	r.Get("/docs/install", s.handlePublicInstallDoc)
	r.Get("/dist", s.handlePublicDistIndex)
	r.Get("/dist/{name}", s.handlePublicDist)
	r.Get("/docs/install.md", s.handlePublicInstallDoc)
	r.Get("/robots.txt", s.handleRobotsTxt)
	r.Get("/sitemap.xml", s.handleSitemapXML)
	r.Get("/llms.txt", s.handleLlmsTxt)
	r.Get("/LLMS_API.md", s.handleLLMSAPIDoc)
	r.Get("/assets/{name}", s.handlePublicAsset)
	// Cloud overlay routes (admin/billing); no-op on OSS builds.
	s.mountCloudRoutes(r)
	r.Get("/console", s.handleConsolePage)
	r.Get("/console/", s.handleConsolePage)
	r.Route("/console/api", func(cr chi.Router) {
		cr.Get("/me", s.handleConsoleMe)
		cr.Post("/logout", s.handleConsoleLogout)
		cr.Get("/overview", s.handleConsoleOverview)
		cr.Post("/trial", s.handleConsoleTrial)
		cr.Post("/quota/refresh", s.handleConsoleQuotaRefresh)
		cr.Post("/accounts", s.handleConsoleCreateAccount)
		cr.Delete("/accounts/{id}", s.handleConsoleDeleteAccount)
		cr.Post("/connect/claude/start", s.handleConsoleConnectClaudeStart)
		cr.Post("/connect/claude/complete", s.handleConsoleConnectClaudeComplete)
		cr.Post("/connect/codex/start", s.handleConsoleConnectCodexStart)
		cr.Post("/connect/codex/poll", s.handleConsoleConnectCodexPoll)
		cr.Post("/routes", s.handleConsoleCreateRoute)
		cr.Patch("/routes/{slug}", s.handleConsoleUpdateRoute)
		cr.Delete("/routes/{slug}", s.handleConsoleDeleteRoute)
		cr.Post("/routes/{slug}/accounts", s.handleConsoleAttachAccount)
		cr.Delete("/routes/{slug}/accounts/{accountId}", s.handleConsoleDetachAccount)
		cr.Get("/routes/{slug}/models", s.handleConsoleRouteModels)
		cr.Get("/keys", s.handleConsoleListKeys)
		cr.Post("/keys", s.handleConsoleCreateKey)
		cr.Post("/keys/{id}/revoke", s.handleConsoleRevokeKey)
	})
	r.Get("/{lang}/install", s.handlePublicInstallHTMLLang)
	r.Get("/{lang}", s.handlePublicLandingLang)

	// Control plane: short timeout OK
	r.Group(func(ctrl chi.Router) {
		ctrl.Use(middleware.Timeout(60 * time.Second))
		ctrl.Post("/control/v1/bootstrap", s.handleBootstrap)
		ctrl.Post("/control/v1/auth/github", s.handleGitHubAuth)

		ctrl.Group(func(pr chi.Router) {
			pr.Use(authn.WithAuth(s.store))
			pr.Get("/v1/me", s.handleMe)
			pr.Get("/control/v1/status", s.handleStatus)
			pr.Get("/control/v1/accounts", s.handleListAccounts)
			pr.Delete("/control/v1/accounts/{id}", s.handleDeleteAccount)
			pr.Post("/control/v1/project/caps", s.handleSetCaps)
			pr.Get("/control/v1/keys", s.handleListKeys)
			pr.Post("/control/v1/keys", s.handleCreateKey)
			pr.Post("/control/v1/keys/{id}/revoke", s.handleRevokeKey)
			pr.Post("/control/v1/routes", s.handleCreateRoute)
			pr.Get("/control/v1/routes", s.handleListRoutes)
			pr.Patch("/control/v1/routes/{slug}", s.handleUpdateRoute)
			pr.Delete("/control/v1/routes/{slug}", s.handleDeleteRoute)
			pr.Get("/control/v1/routes/{slug}/models", s.handleControlRouteModels)
			pr.Post("/control/v1/accounts", s.handleCreateAccount)
			pr.Post("/control/v1/accounts/{id}/quota", s.handleSetAccountQuota)
			pr.Post("/control/v1/quota/refresh", s.handleQuotaRefresh)
			pr.Post("/control/v1/routes/{slug}/accounts", s.handleAttachAccount)
			pr.Get("/control/v1/billing", s.handleControlBilling)
			pr.Post("/control/v1/billing/trial", s.handleControlBillingTrial)
		})
	})

	// Data plane: no chi Timeout — streams can exceed 130s (Caddy timeouts remain 0).
	r.Group(func(pr chi.Router) {
		pr.Use(authn.WithAuth(s.store))
		pr.Get("/r/{slug}/v1/models", s.handleRouteModels)
		pr.Post("/r/{slug}/v1/chat/completions", s.handleChatCompletions)
		pr.Post("/r/{slug}/v1/images/generations", s.handleImagesGenerations)
		pr.Post("/r/{slug}/v1/messages", s.handleAnthropicMessages)
		pr.Post("/r/{slug}/messages", s.handleAnthropicMessages)
	})

	return r
}

func (s *Server) handleMeta(w http.ResponseWriter, _ *http.Request) {
	base := s.cfg.PublicBaseURL
	if base == "" {
		base = "https://llms.goodtek.xyz"
	}
	out := map[string]any{
		"service":          "llms-gateway",
		"github_client_id": s.cfg.GitHubClientID,
		"api_base_hint":    base,
		"bootstrap":        s.cfg.BootstrapToken != "",
		"public_base_url":  base,
		"billing":          base + "/billing",
		"billing_mock":     s.cfg.BillingMock,
		"billing_enforce":  s.cfg.BillingEnforce,
		"console":          base + "/console",
		"landing":          base + "/",
		"install_html":     base + "/install",
		"install_sh":       base + "/install.sh",
		"install_doc":      base + "/install.md",
		"admin":            base + "/admin",
		"robots":           base + "/robots.txt",
		"sitemap":          base + "/sitemap.xml",
		"llms_txt":         base + "/llms.txt",
		"unified_api_doc":  base + "/LLMS_API.md",
		"api_surface":      "chat/completions",
		"landing_i18n": map[string]string{
			"ko": base + "/",
			"en": base + "/en",
			"ja": base + "/ja",
			"zh": base + "/zh",
		},
		"install_html_i18n": map[string]string{
			"ko": base + "/install",
			"en": base + "/en/install",
			"ja": base + "/ja/install",
			"zh": base + "/zh/install",
		},
	}
	for k, v := range s.distMeta() {
		out[k] = v
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "llms-gateway",
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "not_ready",
			"error":  "database_unreachable",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	if s.cfg.BootstrapToken == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "bootstrap_disabled"})
		return
	}
	if r.Header.Get("X-Bootstrap-Token") != s.cfg.BootstrapToken {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	var body struct {
		Login       string `json:"login"`
		ProjectName string `json:"project_name"`
		KeyName     string `json:"key_name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Login == "" {
		body.Login = "bootstrap"
	}
	if body.ProjectName == "" {
		body.ProjectName = "default"
	}
	if body.KeyName == "" {
		body.KeyName = "default"
	}

	projectID, plaintext, err := s.store.Bootstrap(r.Context(), body.Login, body.ProjectName, body.KeyName)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"project_id": projectID.String(),
		"api_key":    plaintext,
		"warning":    "store api_key now; it will not be shown again",
	})
}

func (s *Server) handleGitHubAuth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AccessToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "access_token required"})
		return
	}
	u, err := githubauth.FetchUser(r.Context(), body.AccessToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "github_user_failed", "detail": err.Error()})
		return
	}
	ghID := fmt.Sprintf("%d", u.ID)
	_, projectID, key, created, err := s.store.UpsertGitHubUser(r.Context(), ghID, u.Login)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if created {
		go s.notifySignup(context.Background(), u.Login, ghID, projectID.String())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"login":      u.Login,
		"project_id": projectID.String(),
		"api_key":    key,
		"created":    created,
		"warning":    "store api_key now; it will not be shown again",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	accounts, routes, err := s.store.StatusSnapshot(r.Context(), ac.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	reqs, tokens, _ := s.monthUsage(r.Context(), ac.ProjectID)
	softCap := s.planSoftCap(r.Context(), ac.ProjectID)
	var softTokPtr *int64
	if softCap > 0 {
		softTokPtr = &softCap
	}
	accOut := make([]map[string]any, 0, len(accounts))
	for _, a := range accounts {
		glyph := "●"
		if a.Health == "cooldown" {
			glyph = "○"
		} else if a.Health == "error" {
			glyph = "✖"
		}
		accOut = append(accOut, map[string]any{
			"ref": a.Vendor + ":" + a.Name, "health": a.Health, "glyph": glyph, "id": a.ID.String(),
			"quota_remaining_pct": a.QuotaRemainingPct, "quota_reset_at": a.QuotaResetAt, "quota_updated_at": a.QuotaUpdatedAt,
		})
	}
	rtOut := make([]map[string]any, 0, len(routes))
	for _, rt := range routes {
		rtOut = append(rtOut, map[string]any{
			"slug": rt.Slug, "strategy": rt.Strategy, "openai_base": "/r/" + rt.Slug + "/v1", "id": rt.ID.String(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"accounts": accOut,
		"routes":   rtOut,
		"usage_month": map[string]any{
			"requests": reqs, "tokens_total": tokens,
			"soft_cap_tokens": softTokPtr,
		},
	})
}

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	list, err := s.store.ListAccounts(r.Context(), ac.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, a := range list {
		out = append(out, map[string]any{
			"id": a.ID.String(), "vendor": a.Vendor, "name": a.Name, "health": a.Health, "base_url": a.BaseURL,
			"quota_remaining_pct": a.QuotaRemainingPct, "quota_reset_at": a.QuotaResetAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": out})
}

func (s *Server) handleSetAccountQuota(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	var body struct {
		RemainingPct *float64   `json:"remaining_pct"`
		ResetAt      *time.Time `json:"reset_at"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.store.SetAccountQuota(r.Context(), ac.ProjectID, id, body.RemainingPct, body.ResetAt); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// StartQuotaRefresh runs provider oauth usage fetch + usage-derived quota updates until ctx is done.
func (s *Server) StartQuotaRefresh(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = 5 * time.Minute
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			s.runQuotaRefresh(ctx)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

func (s *Server) runQuotaRefresh(ctx context.Context) {
	if s.secret != nil {
		ref := &quota.Refresher{Store: s.store, Secrets: s.secret}
		_, _ = ref.RefreshAllOAuth(ctx)
	}
	if s.proxy != nil && s.store != nil {
		if list, err := s.store.ListOAuthAccounts(ctx); err == nil {
			_, _, _ = s.proxy.RefreshExpiringOAuth(ctx, list, proxy.DefaultOAuthRefreshLead)
		}
	}
	if _, err := s.store.RefreshQuotasFromUsage(ctx); err != nil && ctx.Err() != nil {
		return
	}
}

func (s *Server) handleQuotaRefresh(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	var updated, failed int
	if s.secret != nil {
		ref := &quota.Refresher{Store: s.store, Secrets: s.secret}
		updated, failed = ref.RefreshProjectOAuth(r.Context(), ac.ProjectID)
	}
	var oauthRotated int
	if s.proxy != nil && s.store != nil {
		if list, err := s.store.ListOAuthAccountsByProject(r.Context(), ac.ProjectID); err == nil {
			oauthRotated, _, _ = s.proxy.RefreshExpiringOAuth(r.Context(), list, proxy.DefaultOAuthRefreshLead)
		}
	}
	n, err := s.store.RefreshQuotasFromUsageForProject(r.Context(), ac.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"oauth_updated": updated, "oauth_failed": failed, "oauth_rotated": oauthRotated, "usage_heuristic_updated": n,
	})
}

func (s *Server) handleSetCaps(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	var body struct {
		SoftCapTokens *int64   `json:"soft_cap_tokens"`
		SoftCapUSD    *float64 `json:"soft_cap_usd"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := s.store.SetProjectCaps(r.Context(), ac.ProjectID, body.SoftCapUSD, body.SoftCapTokens); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"key_id":     ac.KeyID.String(),
		"project_id": ac.ProjectID.String(),
		"key_name":   ac.KeyName,
		"key_prefix": ac.Prefix,
	})
}

func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if !s.requirePlanCapacity(w, r, ac.ProjectID, "key") {
		return
	}
	var body struct {
		Name    string `json:"name"`
		RouteID string `json:"route_id"`
		Slug    string `json:"route_slug"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		body.Name = "default"
	}
	var routeID *uuid.UUID
	if body.RouteID != "" {
		id, err := uuid.Parse(body.RouteID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid route_id"})
			return
		}
		rt, err := s.store.GetRouteByID(r.Context(), id)
		if err != nil || rt.ProjectID != ac.ProjectID {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "route not found"})
			return
		}
		routeID = &rt.ID
	} else if body.Slug != "" {
		rt, err := s.store.GetRouteBySlug(r.Context(), ac.ProjectID, body.Slug)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "route not found"})
			return
		}
		routeID = &rt.ID
	}
	plaintext, err := s.store.CreateKey(r.Context(), ac.ProjectID, body.Name, routeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"api_key": plaintext,
		"warning": "store api_key now; it will not be shown again",
	})
}

func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	list, err := s.store.ListKeys(r.Context(), ac.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, k := range list {
		row := map[string]any{
			"id":         k.ID.String(),
			"name":       k.Name,
			"key_prefix": k.Prefix,
			"created_at": k.CreatedAt,
			"revoked":    k.RevokedAt != nil,
		}
		if k.RouteID != nil {
			row["route_id"] = k.RouteID.String()
		}
		if k.RevokedAt != nil {
			row["revoked_at"] = k.RevokedAt
		}
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": out})
}

func (s *Server) handleRevokeKey(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid key id"})
		return
	}
	if err := s.store.RevokeKey(r.Context(), ac.ProjectID, id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "revoked": id.String()})
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid account id"})
		return
	}
	path, err := s.store.DeleteAccount(r.Context(), ac.ProjectID, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	secretDeleted := false
	if s.secret != nil && path != "" {
		if err := s.secret.Delete(r.Context(), path, vendor.SecretName); err == nil {
			secretDeleted = true
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "id": id.String(), "secret_deleted": secretDeleted,
	})
}

func (s *Server) handleCreateRoute(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if !s.requirePlanCapacity(w, r, ac.ProjectID, "route") {
		return
	}
	var body struct {
		Slug         string          `json:"slug"`
		Strategy     string          `json:"strategy"`
		Preset       string          `json:"preset"`
		DefaultModel string          `json:"default_model"`
		Config       json.RawMessage `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Slug == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "slug required"})
		return
	}
	if body.Preset != "" {
		st, cfg := routerpkg.ApplyPreset(body.Preset)
		body.Strategy = string(st)
		if body.Config == nil {
			b, _ := json.Marshal(cfg)
			body.Config = b
		}
	}
	if !s.requireProParallel(w, r, ac.ProjectID, body.Preset, body.Strategy) {
		return
	}
	rt, err := s.store.CreateRoute(r.Context(), ac.ProjectID, body.Slug, body.Strategy, body.DefaultModel, body.Config)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":            rt.ID.String(),
		"slug":          rt.Slug,
		"strategy":      rt.Strategy,
		"default_model": rt.DefaultModel,
		"openai_base":   "/r/" + rt.Slug + "/v1",
	})
}

func (s *Server) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	list, err := s.store.ListRoutes(r.Context(), ac.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, rt := range list {
		accs, _ := s.store.ListRouteAccounts(r.Context(), rt.ID)
		ids := make([]string, 0, len(accs))
		refs := make([]string, 0, len(accs))
		for _, a := range accs {
			ids = append(ids, a.ID.String())
			refs = append(refs, a.Vendor+":"+a.Name)
		}
		out = append(out, map[string]any{
			"id":            rt.ID.String(),
			"slug":          rt.Slug,
			"strategy":      rt.Strategy,
			"preset":        routerpkg.PresetFromRoute(rt.Strategy, rt.Config),
			"default_model": rt.DefaultModel,
			"openai_base":   "/r/" + rt.Slug + "/v1",
			"account_ids":   ids,
			"accounts":      refs,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"routes": out})
}

func (s *Server) handleUpdateRoute(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	slug := chi.URLParam(r, "slug")
	rt, err := s.store.GetRouteBySlug(r.Context(), ac.ProjectID, slug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "route not found"})
		return
	}
	var body struct {
		Strategy     string          `json:"strategy"`
		Preset       string          `json:"preset"`
		DefaultModel *string         `json:"default_model"`
		Config       json.RawMessage `json:"config"`
		AccountIDs   []string        `json:"account_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	strategy := rt.Strategy
	config := rt.Config
	if body.Preset != "" {
		st, cfg := routerpkg.ApplyPreset(body.Preset)
		strategy = string(st)
		b, _ := json.Marshal(cfg)
		config = b
	} else if body.Strategy != "" {
		strategy = body.Strategy
		if body.Config != nil {
			config = body.Config
		}
	} else if body.Config != nil {
		config = body.Config
	}
	if !s.requireProParallel(w, r, ac.ProjectID, body.Preset, strategy) {
		return
	}
	defaultModel := rt.DefaultModel
	if body.DefaultModel != nil {
		defaultModel = *body.DefaultModel
	}
	updated, err := s.store.UpdateRoute(r.Context(), ac.ProjectID, slug, strategy, defaultModel, config)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if body.AccountIDs != nil {
		ids := make([]uuid.UUID, 0, len(body.AccountIDs))
		seen := map[uuid.UUID]bool{}
		for _, aidStr := range body.AccountIDs {
			aid, err := uuid.Parse(aidStr)
			if err != nil || seen[aid] {
				continue
			}
			if _, err := s.store.GetAccount(r.Context(), ac.ProjectID, aid); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "account not found", "account_id": aidStr})
				return
			}
			seen[aid] = true
			ids = append(ids, aid)
		}
		if err := s.store.ReplaceRouteAccounts(r.Context(), updated.ID, ids); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "slug": updated.Slug, "strategy": updated.Strategy,
		"preset": routerpkg.PresetFromRoute(updated.Strategy, updated.Config),
	})
}

func (s *Server) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	slug := chi.URLParam(r, "slug")
	if err := s.store.DeleteRoute(r.Context(), ac.ProjectID, slug); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "slug": slug})
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if !s.requirePlanCapacity(w, r, ac.ProjectID, "account") {
		return
	}
	if s.secret == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "infisical_not_configured",
			"hint":  "See docs/ops/HUMAN-SETUP.md — set INFISICAL_PROJECT_ID, INFISICAL_CLIENT_ID, INFISICAL_CLIENT_SECRET",
		})
		return
	}
	var body struct {
		Vendor       string `json:"vendor"`
		Name         string `json:"name"`
		APIKey       string `json:"api_key"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresAt    string `json:"expires_at"`
		IDToken      string `json:"id_token"`
		ChatGPTAcct  string `json:"chatgpt_account_id"`
		AuthType     string `json:"auth_type"`
		BaseURL      string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Vendor == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "vendor required"})
		return
	}
	if body.Name == "" {
		body.Name = "default"
	}
	if body.AuthType == "" {
		if body.AccessToken != "" {
			body.AuthType = "oauth"
		} else {
			body.AuthType = "api_key"
		}
	}
	if body.AuthType == "api_key" && body.APIKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "api_key required for auth_type=api_key"})
		return
	}
	if body.AuthType == "oauth" && body.AccessToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "access_token required for auth_type=oauth"})
		return
	}
	if body.BaseURL == "" {
		body.BaseURL = vendor.DefaultBaseURLFor(body.Vendor, body.AuthType)
	}
	path := vendor.InfisicalPath(ac.ProjectID.String(), body.Vendor, body.Name)
	cred := secrets.CredentialJSON{
		APIKey: body.APIKey, AccessToken: body.AccessToken, RefreshToken: body.RefreshToken,
		ExpiresAt: body.ExpiresAt, IDToken: body.IDToken, ChatGPTAccountID: body.ChatGPTAcct,
	}
	raw, _ := json.Marshal(cred)
	if err := s.secret.Put(r.Context(), path, vendor.SecretName, string(raw)); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "infisical_put_failed", "detail": err.Error()})
		return
	}
	acc, err := s.store.CreateAccount(r.Context(), ac.ProjectID, body.Vendor, body.Name, body.AuthType, path, body.BaseURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":             acc.ID.String(),
		"vendor":         acc.Vendor,
		"name":           acc.Name,
		"infisical_path": acc.InfisicalPath,
		"base_url":       acc.BaseURL,
	})
}

func (s *Server) handleAttachAccount(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	slug := chi.URLParam(r, "slug")
	rt, err := s.store.GetRouteBySlug(r.Context(), ac.ProjectID, slug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "route not found"})
		return
	}
	var body struct {
		AccountID string `json:"account_id"`
		Position  int    `json:"position"`
		Weight    int    `json:"weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "account_id required"})
		return
	}
	aid, err := uuid.Parse(body.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid account_id"})
		return
	}
	if _, err := s.store.GetAccount(r.Context(), ac.ProjectID, aid); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "account not found"})
		return
	}
	if err := s.store.AttachAccount(r.Context(), rt.ID, aid, body.Position, body.Weight); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRouteModels(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	slug := chi.URLParam(r, "slug")
	rt, err := s.store.GetRouteBySlug(r.Context(), ac.ProjectID, slug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "route not found"})
		return
	}
	if err := s.store.AuthorizeRoute(ac, rt); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
		return
	}
	accounts, err := s.store.ListRouteAccounts(r.Context(), rt.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if s.models == nil {
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": []any{}})
		return
	}
	cat := s.models.ForRoute(r.Context(), rt, accounts)
	writeJSON(w, http.StatusOK, modelcatalog.OpenAIList(cat))
}

func (s *Server) handleControlRouteModels(w http.ResponseWriter, r *http.Request) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	slug := chi.URLParam(r, "slug")
	rt, err := s.store.GetRouteBySlug(r.Context(), ac.ProjectID, slug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "route not found"})
		return
	}
	if err := s.store.AuthorizeRoute(ac, rt); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
		return
	}
	accounts, err := s.store.ListRouteAccounts(r.Context(), rt.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if s.models == nil {
		writeJSON(w, http.StatusOK, modelcatalog.RouteModels{Route: slug, Strategy: rt.Strategy, Accounts: []modelcatalog.AccountModels{}, Models: []modelcatalog.UnionModel{}})
		return
	}
	writeJSON(w, http.StatusOK, s.models.ForRoute(r.Context(), rt, accounts))
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	s.handleUpstream(w, r, upstreamChat)
}

func (s *Server) handleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	s.handleUpstream(w, r, upstreamAnthropic)
}

func (s *Server) handleImagesGenerations(w http.ResponseWriter, r *http.Request) {
	s.handleUpstream(w, r, upstreamImages)
}

type upstreamKind int

const (
	upstreamChat upstreamKind = iota
	upstreamAnthropic
	upstreamImages
)

func (s *Server) handleUpstream(w http.ResponseWriter, r *http.Request, kind upstreamKind) {
	ac, ok := authn.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if !s.requirePlanEntitled(w, r, ac.ProjectID) {
		return
	}
	rpm := s.planRPMLimit(r.Context(), ac.ProjectID)
	if s.limit != nil && !s.limit.AllowWithLimit(ac.KeyID.String(), rpm) {
		w.Header().Set("Retry-After", "60")
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":     "rate_limited",
			"hint":      "per-key RPM cap for your plan",
			"rpm_limit": rpm,
			"billing":   s.billingURL(),
		})
		return
	}
	cap := s.planSoftCap(r.Context(), ac.ProjectID)
	over, used, err := s.store.OverSoftTokenCap(r.Context(), ac.ProjectID, cap)
	if err == nil && over {
		w.Header().Set("Retry-After", "3600")
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":           "soft_cap_exceeded",
			"hint":            "monthly token soft cap for your plan; upgrade or wait for calendar month reset",
			"tokens_used":     used,
			"soft_cap_tokens": cap,
			"billing":         s.billingURL(),
		})
		return
	}
	if s.secret == nil || s.proxy.Secrets == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "infisical_not_configured"})
		return
	}
	slug := chi.URLParam(r, "slug")
	rt, err := s.store.GetRouteBySlug(r.Context(), ac.ProjectID, slug)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "route not found"})
		return
	}
	if err := s.store.AuthorizeRoute(ac, rt); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad body"})
		return
	}
	var res *proxy.Result
	meta := proxy.DispatchMeta{Header: r.Header, SessionKey: proxy.SessionKey(r.Header, body)}
	switch kind {
	case upstreamAnthropic:
		res, err = s.proxy.AnthropicMessages(r.Context(), rt, body, meta)
	case upstreamImages:
		res, err = s.proxy.ImagesGenerations(r.Context(), ac.ProjectID, rt, body, meta)
	default:
		res, err = s.proxy.ChatCompletions(r.Context(), ac.ProjectID, rt, body, meta)
	}
	if err != nil && res == nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	rid := rt.ID
	aid := res.AccountID
	kid := ac.KeyID
	_ = s.store.InsertUsage(r.Context(), ac.ProjectID, &rid, &aid, &kid, res.Model, res.StatusCode, int(res.Latency.Milliseconds()), res.TokensIn, res.TokensOut, res.Error)

	if res.AccountID != uuid.Nil {
		w.Header().Set("X-LLMs-Account-Id", res.AccountID.String())
	}
	if res.Attempts > 0 {
		w.Header().Set("X-LLMs-Attempts", fmt.Sprintf("%d", res.Attempts))
	}
	if res.Strategy != "" {
		w.Header().Set("X-LLMs-Strategy", res.Strategy)
	}
	if res.SessionKey != "" {
		w.Header().Set("X-LLMs-Session", res.SessionKey)
	}
	if res.StickyHit {
		w.Header().Set("X-LLMs-Sticky", "1")
	}

	if res.Stream != nil {
		defer res.Stream.Close()
		ct := res.Header.Get("Content-Type")
		if ct == "" {
			ct = "text/event-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(res.StatusCode)
		flusher, _ := w.(http.Flusher)
		buf := make([]byte, 32*1024)
		for {
			n, readErr := res.Stream.Read(buf)
			if n > 0 {
				_, _ = w.Write(buf[:n])
				if flusher != nil {
					flusher.Flush()
				}
			}
			if readErr != nil {
				break
			}
		}
		return
	}
	ct := res.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/json"
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(res.StatusCode)
	if len(res.Body) > 0 {
		_, _ = w.Write(res.Body)
	} else if res.Error != "" {
		_ = json.NewEncoder(w).Encode(map[string]any{"error": res.Error})
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
