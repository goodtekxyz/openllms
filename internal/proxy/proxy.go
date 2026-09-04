package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/goodtekxyz/openllms/internal/router"
	"github.com/goodtekxyz/openllms/internal/secrets"
	"github.com/goodtekxyz/openllms/internal/store"
	"github.com/goodtekxyz/openllms/internal/vendor"
	"github.com/google/uuid"
)

type Result struct {
	StatusCode int
	Body       []byte
	Latency    time.Duration
	TokensIn   int
	TokensOut  int
	AccountID  uuid.UUID
	Model      string
	Error      string
	Stream     io.ReadCloser // when non-nil, caller must drain/close; Body empty
	Header     http.Header
	Attempts   int // how many accounts tried (incl. success)
	Strategy   string
	SessionKey string
	StickyHit  bool
}

// AccountStore is the subset of store needed for routing.
type AccountStore interface {
	ListRouteAccounts(ctx context.Context, routeID uuid.UUID) ([]store.Account, error)
	SetCooldown(ctx context.Context, accountID uuid.UUID, until time.Time) error
	GetRouteBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*store.Route, error)
}

type Engine struct {
	Store    AccountStore
	Secrets  secrets.Client
	HTTP     *http.Client
	Selector *router.Selector
	Affinity *Affinity
	Log      *slog.Logger

	refreshGates *refreshGates

	pendingMu    sync.Mutex
	pendingCreds map[string]secrets.CredentialJSON
}

func (e *Engine) affinity() *Affinity {
	if e.Affinity == nil {
		e.Affinity = NewAffinity(DefaultAffinityTTL)
	}
	return e.Affinity
}

func (e *Engine) selector() *router.Selector {
	if e.Selector == nil {
		e.Selector = router.NewSelector()
	}
	return e.Selector
}

type DispatchMeta struct {
	SessionKey string
	Header     http.Header
}

func (e *Engine) ChatCompletions(ctx context.Context, projectID uuid.UUID, rt *store.Route, body []byte, meta DispatchMeta) (*Result, error) {
	return e.dispatchWithFallbacks(ctx, projectID, rt, body, meta, e.callOpenAI)
}

func (e *Engine) AnthropicMessages(ctx context.Context, rt *store.Route, body []byte, meta DispatchMeta) (*Result, error) {
	return e.dispatchWithFallbacks(ctx, rt.ProjectID, rt, body, meta, e.callAnthropic)
}

// ImagesGenerations proxies OpenAI-compatible POST /v1/images/generations through the route pool.
// OpenAI API-key and Codex/OpenAI OAuth use ChatGPT/OpenAI /responses + image_generation.
func (e *Engine) ImagesGenerations(ctx context.Context, projectID uuid.UUID, rt *store.Route, body []byte, meta DispatchMeta) (*Result, error) {
	return e.dispatchWithFallbacks(ctx, projectID, rt, body, meta, e.callImages, filterImageCapableAccounts)
}

func (e *Engine) CallImagesAccountForTest(ctx context.Context, acc *store.Account, body []byte) (*Result, error) {
	return e.callImages(ctx, acc, body)
}

func filterImageCapableAccounts(accounts []store.Account) []store.Account {
	out := make([]store.Account, 0, len(accounts))
	for i := range accounts {
		if accountSupportsImages(&accounts[i]) {
			out = append(out, accounts[i])
		}
	}
	return out
}

func accountSupportsImages(acc *store.Account) bool {
	if acc == nil {
		return false
	}
	v := strings.ToLower(acc.Vendor)
	if v == "claude" || v == "anthropic" {
		return false
	}
	return true
}

func fallbackSlugs(cfg []byte) []string {
	var c struct {
		FallbackSlugs []string `json:"fallback_slugs"`
	}
	_ = json.Unmarshal(cfg, &c)
	return c.FallbackSlugs
}

func (e *Engine) dispatchWithFallbacks(ctx context.Context, projectID uuid.UUID, rt *store.Route, body []byte, meta DispatchMeta, call callFn, accountFilter ...func([]store.Account) []store.Account) (*Result, error) {
	res, err := e.dispatch(ctx, rt, body, meta, call, accountFilter...)
	if err != nil {
		return nil, err
	}
	if res != nil && res.StatusCode > 0 && !router.IsRetryableStatus(res.StatusCode) {
		return res, nil
	}
	// Primary pool exhausted or only retryable left — try fallback routes.
	for _, slug := range fallbackSlugs(rt.Config) {
		if e.Store == nil || slug == "" || slug == rt.Slug {
			continue
		}
		frt, err := e.Store.GetRouteBySlug(ctx, projectID, slug)
		if err != nil || frt == nil {
			continue
		}
		fres, ferr := e.dispatch(ctx, frt, body, meta, call, accountFilter...)
		if ferr != nil {
			return nil, ferr
		}
		if fres != nil && fres.StatusCode > 0 && !router.IsRetryableStatus(fres.StatusCode) {
			if res != nil {
				fres.Attempts += res.Attempts
			}
			return fres, nil
		}
		if fres != nil {
			res = fres
		}
	}
	return res, nil
}

type callFn func(ctx context.Context, acc *store.Account, body []byte) (*Result, error)

func parallelAllowOAuth(cfg []byte) bool {
	var c struct {
		ParallelAllowOAuth bool `json:"parallel_allow_oauth"`
	}
	_ = json.Unmarshal(cfg, &c)
	return c.ParallelAllowOAuth
}

func filterParallelAccounts(accounts []store.Account, allowOAuth bool) []store.Account {
	if allowOAuth {
		return accounts
	}
	out := make([]store.Account, 0, len(accounts))
	for _, a := range accounts {
		if strings.EqualFold(a.AuthType, "oauth") {
			continue
		}
		out = append(out, a)
	}
	return out
}

func (e *Engine) dispatchParallel(ctx context.Context, rt *store.Route, accounts []store.Account, body []byte, meta DispatchMeta, sessionKey string, sticky uuid.UUID, call callFn) (*Result, error) {
	pool := filterParallelAccounts(accounts, parallelAllowOAuth(rt.Config))
	if len(pool) == 0 {
		// Safe fallback: sequential on original list
		rt2 := *rt
		rt2.Strategy = string(router.Sequential)
		return e.dispatch(ctx, &rt2, body, meta, call)
	}
	if sticky != uuid.Nil {
		pool = router.PreferAccount(pool, sticky)
	}
	type raced struct {
		res *Result
		err error
	}
	ch := make(chan raced, len(pool))
	ctxs := make([]context.CancelFunc, 0, len(pool))
	for i := range pool {
		acc := pool[i]
		cctx, cancel := context.WithCancel(ctx)
		ctxs = append(ctxs, cancel)
		go func(a store.Account) {
			res, err := call(cctx, &a, body)
			if err != nil && res == nil {
				ch <- raced{err: err}
				return
			}
			if res == nil {
				ch <- raced{res: &Result{StatusCode: http.StatusBadGateway, Error: "empty", AccountID: a.ID}}
				return
			}
			if res.StatusCode > 0 && !router.IsRetryableStatus(res.StatusCode) {
				if res.Stream != nil {
					out, retryable, perr := awaitCommit(res.Stream)
					if retryable {
						ch <- raced{res: &Result{StatusCode: http.StatusBadGateway, Error: "ttft_failed", AccountID: a.ID}}
						if perr != nil {
							_ = perr
						}
						return
					}
					res.Stream = out
				}
				ch <- raced{res: res}
				return
			}
			if res.Stream != nil {
				_ = res.Stream.Close()
			}
			ch <- raced{res: res}
		}(acc)
	}

	var last *Result
	remaining := len(pool)
	for remaining > 0 {
		select {
		case <-ctx.Done():
			for _, c := range ctxs {
				c()
			}
			return nil, ctx.Err()
		case r := <-ch:
			remaining--
			if r.err != nil {
				continue
			}
			if r.res == nil {
				continue
			}
			last = r.res
			if r.res.StatusCode > 0 && !router.IsRetryableStatus(r.res.StatusCode) {
				for _, c := range ctxs {
					c()
				}
				r.res.Attempts = len(pool)
				r.res.Strategy = string(router.Parallel)
				r.res.SessionKey = sessionKey
				r.res.StickyHit = sticky != uuid.Nil && sticky == r.res.AccountID
				e.affinity().Set(sessionKey, r.res.AccountID)
				return r.res, nil
			}
		}
	}
	for _, c := range ctxs {
		c()
	}
	if last != nil {
		last.Attempts = len(pool)
		last.Strategy = string(router.Parallel)
		last.SessionKey = sessionKey
	}
	return last, nil
}

func (e *Engine) dispatch(ctx context.Context, rt *store.Route, body []byte, meta DispatchMeta, call callFn, accountFilter ...func([]store.Account) []store.Account) (*Result, error) {
	if e.Store == nil {
		return &Result{StatusCode: http.StatusInternalServerError, Error: "no_store"}, nil
	}
	accounts, err := e.Store.ListRouteAccounts(ctx, rt.ID)
	if err != nil {
		return nil, err
	}
	if len(accountFilter) > 0 && accountFilter[0] != nil {
		accounts = accountFilter[0](accounts)
	}
	if len(accounts) == 0 {
		hint := "attach accounts to this route"
		if len(accountFilter) > 0 && accountFilter[0] != nil {
			hint = "for images, attach an OpenAI-compatible API key account (not Claude/Codex OAuth)"
		}
		body, _ := json.Marshal(map[string]string{"error": "route has no accounts", "hint": hint})
		return &Result{
			StatusCode: http.StatusBadRequest,
			Body:       body,
			Error:      "no_accounts",
		}, nil
	}

	strategy := router.Strategy(rt.Strategy)
	if strategy == "" {
		strategy = router.Sequential
	}

	sessionKey := meta.SessionKey
	if sessionKey == "" {
		sessionKey = SessionKey(meta.Header, body)
	}
	sticky, stickyOK := e.affinity().Get(sessionKey)

	if strategy == router.Parallel {
		return e.dispatchParallel(ctx, rt, accounts, body, meta, sessionKey, sticky, call)
	}

	sel := e.selector()
	tried := map[uuid.UUID]struct{}{}
	var last *Result
	attempts := 0

	for {
		candidates := router.Exclude(accounts, tried)
		if len(candidates) == 0 {
			if last != nil {
				last.Attempts = attempts
				last.Strategy = string(strategy)
				last.SessionKey = sessionKey
			}
			return last, nil
		}
		if stickyOK && sticky != uuid.Nil {
			candidates = router.PreferAccount(candidates, sticky)
		}
		acc := sel.Pick(rt.ID, strategy, candidates)
		if acc == nil {
			if last != nil {
				last.Attempts = attempts
				last.Strategy = string(strategy)
				last.SessionKey = sessionKey
			}
			return last, nil
		}
		tried[acc.ID] = struct{}{}
		attempts++

		res, err := call(ctx, acc, body)
		if err != nil && res == nil {
			return nil, err
		}
		if res == nil {
			continue
		}
		res.Attempts = attempts
		res.Strategy = string(strategy)
		res.SessionKey = sessionKey
		res.StickyHit = stickyOK && sticky == acc.ID
		last = res

		if res.StatusCode > 0 && !router.IsRetryableStatus(res.StatusCode) {
			if res.Stream != nil {
				out, retryable, perr := awaitCommit(res.Stream)
				if retryable {
					sel.NoteSoftFail(acc.ID)
					dur := router.CooldownForStatus(http.StatusBadGateway, 0)
					_ = e.Store.SetCooldown(ctx, acc.ID, time.Now().Add(dur))
					last = &Result{
						StatusCode: http.StatusBadGateway,
						Error:      "ttft_failed",
						AccountID:  acc.ID,
						Model:      res.Model,
						Header:     res.Header,
						Attempts:   attempts,
						Strategy:   string(strategy),
						SessionKey: sessionKey,
					}
					if perr != nil {
						last.Error = perr.Error()
					}
					continue
				}
				res.Stream = out
			}
			sel.ClearSoftFail(acc.ID)
			e.affinity().Set(sessionKey, acc.ID)
			return res, nil
		}
		if router.IsRetryableStatus(res.StatusCode) || res.StatusCode == 0 {
			sel.NoteSoftFail(acc.ID)
			dur := router.CooldownFromHeaders(res.StatusCode, res.Header, 0)
			_ = e.Store.SetCooldown(ctx, acc.ID, time.Now().Add(dur))
			if res.Stream != nil {
				_ = res.Stream.Close()
				res.Stream = nil
			}
			continue
		}
		sel.ClearSoftFail(acc.ID)
		e.affinity().Set(sessionKey, acc.ID)
		return res, nil
	}
}

func (e *Engine) CallAccountForTest(ctx context.Context, acc *store.Account, body []byte) (*Result, error) {
	return e.callOpenAI(ctx, acc, body)
}

func (e *Engine) CallAnthropicAccountForTest(ctx context.Context, acc *store.Account, body []byte) (*Result, error) {
	return e.callAnthropic(ctx, acc, body)
}

func (e *Engine) callOpenAI(ctx context.Context, acc *store.Account, body []byte) (*Result, error) {
	if isCodexOAuth(acc) || (isOpenAIAPIKey(acc) && chatRequestHasTools(body)) {
		return e.callResponsesBridge(ctx, acc, body)
	}
	if isClaudeAccount(acc) {
		return e.callAnthropicBridge(ctx, acc, body)
	}
	return e.doUpstream(ctx, acc, body, resolveOpenAIURL(acc), "Authorization", "Bearer ", true)
}

func (e *Engine) callImages(ctx context.Context, acc *store.Account, body []byte) (*Result, error) {
	if !accountSupportsImages(acc) {
		return &Result{
			StatusCode: http.StatusBadRequest,
			Body:       []byte(`{"error":"images_unsupported_account"}`),
			Error:      "images_unsupported_account",
			AccountID:  acc.ID,
		}, nil
	}
	if isCodexOAuth(acc) || isOpenAIAPIKey(acc) {
		return e.callResponsesImages(ctx, acc, body)
	}
	return e.doUpstream(ctx, acc, body, resolveImagesURL(acc), "Authorization", "Bearer ", true)
}

func (e *Engine) callResponsesImages(ctx context.Context, acc *store.Account, body []byte) (*Result, error) {
	start := time.Now()
	respBody, model, err := imagesToCodexResponses(body)
	if err != nil {
		return &Result{StatusCode: http.StatusBadRequest, Error: err.Error(), AccountID: acc.ID, Latency: time.Since(start)}, nil
	}
	url := resolveResponsesURL(acc)
	n := imagesRequestCount(body)

	cred, token, err := e.loadCredential(ctx, acc)
	if err != nil {
		return &Result{StatusCode: http.StatusBadGateway, Error: err.Error(), AccountID: acc.ID, Latency: time.Since(start)}, err
	}

	var data []map[string]string
	for i := 0; i < n; i++ {
		res, err := e.execCodexImagesOnce(ctx, acc, respBody, url, cred, token, model, start)
		if err != nil || res == nil {
			return res, err
		}
		if res.StatusCode == http.StatusUnauthorized && strings.EqualFold(acc.AuthType, "oauth") && strings.TrimSpace(cred.RefreshToken) != "" {
			newCred, rerr := e.refreshOAuthCredential(ctx, acc, cred)
			if rerr == nil {
				cred, token = newCred, newCred.BearerToken()
				res, err = e.execCodexImagesOnce(ctx, acc, respBody, url, cred, token, model, start)
				if err != nil || res == nil {
					return res, err
				}
			}
		}
		if res.StatusCode >= 300 {
			return res, nil
		}
		var parsed struct {
			Data []struct {
				B64JSON string `json:"b64_json"`
			} `json:"data"`
		}
		if json.Unmarshal(res.Body, &parsed) != nil || len(parsed.Data) == 0 {
			return &Result{StatusCode: http.StatusBadGateway, Error: "codex_images_parse", AccountID: acc.ID, Model: model, Latency: time.Since(start)}, nil
		}
		for _, d := range parsed.Data {
			if d.B64JSON != "" {
				data = append(data, map[string]string{"b64_json": d.B64JSON})
			}
		}
	}
	out, _ := json.Marshal(map[string]any{"created": time.Now().Unix(), "data": data})
	return &Result{
		StatusCode: http.StatusOK,
		Body:       out,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Latency:    time.Since(start),
		AccountID:  acc.ID,
		Model:      model,
	}, nil
}

func (e *Engine) execCodexImagesOnce(ctx context.Context, acc *store.Account, body []byte, url string, cred secrets.CredentialJSON, token, model string, start time.Time) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	applyUpstreamAuth(req, acc, cred, token, true, "Authorization", "Bearer ")

	client := e.HTTP
	if client == nil {
		client = &http.Client{Timeout: 0}
	}
	res, err := client.Do(req)
	if err != nil {
		return &Result{StatusCode: http.StatusBadGateway, Error: err.Error(), AccountID: acc.ID, Model: model, Latency: time.Since(start)}, nil
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return &Result{
			StatusCode: res.StatusCode, Body: b, Header: res.Header.Clone(),
			AccountID: acc.ID, Model: model, Latency: time.Since(start),
			Error: "codex_upstream",
		}, nil
	}
	out, err := aggregateCodexImagesSSE(res.Body)
	if err != nil {
		return &Result{StatusCode: http.StatusBadGateway, Error: err.Error(), AccountID: acc.ID, Model: model, Latency: time.Since(start)}, nil
	}
	return &Result{
		StatusCode: http.StatusOK,
		Body:       out,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Latency:    time.Since(start),
		AccountID:  acc.ID,
		Model:      model,
	}, nil
}

func isCodexOAuth(acc *store.Account) bool {
	if acc == nil || !strings.EqualFold(acc.AuthType, "oauth") {
		return false
	}
	v := strings.ToLower(acc.Vendor)
	return v == "codex" || v == "openai"
}

func isClaudeAccount(acc *store.Account) bool {
	if acc == nil {
		return false
	}
	v := strings.ToLower(acc.Vendor)
	return v == "claude" || v == "anthropic"
}

func isOpenAIAPIKey(acc *store.Account) bool {
	if acc == nil || !strings.EqualFold(acc.AuthType, "api_key") {
		return false
	}
	v := strings.ToLower(acc.Vendor)
	return v == "openai" || v == "codex"
}

func resolveResponsesURL(acc *store.Account) string {
	base := acc.BaseURL
	if base == "" {
		base = vendor.DefaultBaseURLFor(acc.Vendor, acc.AuthType)
	}
	return strings.TrimRight(base, "/") + "/responses"
}

func resolveAnthropicMessagesURL(acc *store.Account) string {
	base := strings.TrimRight(acc.BaseURL, "/")
	if base == "" {
		base = vendor.DefaultBaseURLFor(acc.Vendor, acc.AuthType)
	}
	if strings.HasSuffix(base, "/v1") {
		base = strings.TrimSuffix(base, "/v1")
	}
	return base + "/v1/messages"
}

func (e *Engine) callResponsesBridge(ctx context.Context, acc *store.Account, body []byte) (*Result, error) {
	start := time.Now()
	respBody, clientStream, model, err := chatToCodexResponses(body)
	if err != nil {
		return &Result{StatusCode: http.StatusBadRequest, Error: err.Error(), AccountID: acc.ID, Latency: time.Since(start)}, nil
	}
	url := resolveResponsesURL(acc)
	return e.doUpstreamOnce(ctx, acc, respBody, url, "Authorization", "Bearer ", true, model, clientStream, start, true, false)
}

func (e *Engine) callAnthropicBridge(ctx context.Context, acc *store.Account, body []byte) (*Result, error) {
	start := time.Now()
	respBody, clientStream, model, err := chatToAnthropicMessages(body)
	if err != nil {
		return &Result{StatusCode: http.StatusBadRequest, Error: err.Error(), AccountID: acc.ID, Latency: time.Since(start)}, nil
	}
	url := resolveAnthropicMessagesURL(acc)
	return e.doUpstreamOnce(ctx, acc, respBody, url, "x-api-key", "", false, model, clientStream, start, false, true)
}

func (e *Engine) callAnthropic(ctx context.Context, acc *store.Account, body []byte) (*Result, error) {
	url := resolveAnthropicMessagesURL(acc)
	return e.doUpstream(ctx, acc, body, url, "x-api-key", "", false)
}

func resolveOpenAIURL(acc *store.Account) string {
	base := acc.BaseURL
	if base == "" {
		base = vendor.DefaultBaseURLFor(acc.Vendor, acc.AuthType)
	}
	base = strings.TrimRight(base, "/")
	return base + "/chat/completions"
}

func resolveImagesURL(acc *store.Account) string {
	base := acc.BaseURL
	if base == "" {
		base = vendor.DefaultBaseURLFor(acc.Vendor, acc.AuthType)
	}
	base = strings.TrimRight(base, "/")
	return base + "/images/generations"
}

func (e *Engine) doUpstream(ctx context.Context, acc *store.Account, body []byte, url, authHeader, bearerPrefix string, openaiAuth bool) (*Result, error) {
	start := time.Now()
	var peek struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	_ = json.Unmarshal(body, &peek)
	return e.doUpstreamOnce(ctx, acc, body, url, authHeader, bearerPrefix, openaiAuth, peek.Model, peek.Stream, start, false, false)
}

// doUpstreamOnce performs one upstream call; on oauth 401 with refresh_token, refreshes once and retries.
func (e *Engine) doUpstreamOnce(ctx context.Context, acc *store.Account, body []byte, url, authHeader, bearerPrefix string, openaiAuth bool, model string, stream bool, start time.Time, codexResponses bool, anthropicBridge bool) (*Result, error) {
	cred, token, err := e.loadCredential(ctx, acc)
	if err != nil {
		return &Result{StatusCode: http.StatusBadGateway, Error: err.Error(), AccountID: acc.ID, Latency: time.Since(start)}, err
	}
	res, err := e.execUpstream(ctx, acc, body, url, authHeader, bearerPrefix, openaiAuth, cred, token, model, stream, start, codexResponses, anthropicBridge)
	if err != nil || res == nil {
		return res, err
	}
	if res.StatusCode != http.StatusUnauthorized || !strings.EqualFold(acc.AuthType, "oauth") || strings.TrimSpace(cred.RefreshToken) == "" {
		return res, nil
	}
	// Drain failed attempt body if still open.
	if res.Stream != nil {
		_ = res.Stream.Close()
		res.Stream = nil
	}
	newCred, rerr := e.refreshOAuthCredential(ctx, acc, cred)
	if rerr != nil {
		return res, nil // surface original 401
	}
	return e.execUpstream(ctx, acc, body, url, authHeader, bearerPrefix, openaiAuth, newCred, newCred.BearerToken(), model, stream, start, codexResponses, anthropicBridge)
}

func (e *Engine) loadCredential(ctx context.Context, acc *store.Account) (secrets.CredentialJSON, string, error) {
	raw, err := e.Secrets.Get(ctx, acc.InfisicalPath, vendor.SecretName)
	if err != nil {
		return secrets.CredentialJSON{}, "", fmt.Errorf("secret_fetch")
	}
	var cred secrets.CredentialJSON
	if err := json.Unmarshal([]byte(raw), &cred); err != nil {
		return secrets.CredentialJSON{}, "", fmt.Errorf("bad_secret")
	}
	token := cred.BearerToken()
	if token == "" {
		return secrets.CredentialJSON{}, "", fmt.Errorf("credential missing api_key/access_token")
	}
	return cred, token, nil
}

func (e *Engine) execUpstream(ctx context.Context, acc *store.Account, body []byte, url, authHeader, bearerPrefix string, openaiAuth bool, cred secrets.CredentialJSON, token, model string, stream bool, start time.Time, codexResponses bool, anthropicBridge bool) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if codexResponses || (anthropicBridge && stream) {
		req.Header.Set("Accept", "text/event-stream")
	}
	applyUpstreamAuth(req, acc, cred, token, openaiAuth, authHeader, bearerPrefix)

	client := e.HTTP
	if client == nil {
		client = &http.Client{Timeout: 0} // stream-friendly
	}
	res, err := client.Do(req)
	if err != nil {
		return &Result{StatusCode: http.StatusBadGateway, Error: err.Error(), AccountID: acc.ID, Model: model, Latency: time.Since(start)}, nil
	}

	if anthropicBridge {
		id := "chatcmpl-" + acc.ID.String()[:8]
		opts := anthropicBridgeOpts{
			model:     model,
			id:        id,
			accountID: acc.ID.String(),
			provider:  acc.Vendor,
		}
		if res.StatusCode >= 300 {
			b, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			return &Result{
				StatusCode: res.StatusCode, Body: b, Header: res.Header.Clone(),
				AccountID: acc.ID, Model: model, Latency: time.Since(start),
				Error: "anthropic_upstream",
			}, nil
		}
		if stream {
			pipe := startAnthropicChatPipe(res.Body, opts)
			return &Result{
				StatusCode: res.StatusCode,
				Stream:     pipe,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Latency:    time.Since(start),
				AccountID:  acc.ID,
				Model:      model,
			}, nil
		}
		defer res.Body.Close()
		respBody, _ := io.ReadAll(res.Body)
		var msg map[string]any
		if json.Unmarshal(respBody, &msg) != nil {
			return &Result{StatusCode: http.StatusBadGateway, Error: "anthropic_parse", AccountID: acc.ID, Model: model, Latency: time.Since(start)}, nil
		}
		out, tin, tout, err := convertAnthropicMessageToChat(msg, opts)
		if err != nil {
			return &Result{StatusCode: http.StatusBadGateway, Error: err.Error(), AccountID: acc.ID, Model: model, Latency: time.Since(start)}, nil
		}
		return &Result{
			StatusCode: http.StatusOK,
			Body:       out,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Latency:    time.Since(start),
			TokensIn:   tin,
			TokensOut:  tout,
			AccountID:  acc.ID,
			Model:      model,
		}, nil
	}

	if codexResponses {
		id := "chatcmpl-" + acc.ID.String()[:8]
		opts := codexBridgeOpts{
			model:     model,
			id:        id,
			accountID: acc.ID.String(),
			provider:  acc.Vendor,
		}
		if res.StatusCode >= 300 {
			b, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			return &Result{
				StatusCode: res.StatusCode, Body: b, Header: res.Header.Clone(),
				AccountID: acc.ID, Model: model, Latency: time.Since(start),
				Error: "codex_upstream",
			}, nil
		}
		if stream {
			pipe := startCodexChatPipe(res.Body, opts)
			return &Result{
				StatusCode: res.StatusCode,
				Stream:     pipe,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Latency:    time.Since(start),
				AccountID:  acc.ID,
				Model:      model,
			}, nil
		}
		defer res.Body.Close()
		out, tin, tout, err := aggregateCodexSSE(res.Body, opts)
		if err != nil {
			return &Result{StatusCode: http.StatusBadGateway, Error: err.Error(), AccountID: acc.ID, Model: model, Latency: time.Since(start)}, nil
		}
		return &Result{
			StatusCode: http.StatusOK,
			Body:       out,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Latency:    time.Since(start),
			TokensIn:   tin,
			TokensOut:  tout,
			AccountID:  acc.ID,
			Model:      model,
		}, nil
	}

	hdr := res.Header.Clone()
	if stream && res.StatusCode < 300 {
		return &Result{
			StatusCode: res.StatusCode,
			Stream:     res.Body,
			Header:     hdr,
			Latency:    time.Since(start),
			AccountID:  acc.ID,
			Model:      model,
		}, nil
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	tin, tout := parseUsage(respBody)
	return &Result{
		StatusCode: res.StatusCode,
		Body:       respBody,
		Header:     hdr,
		Latency:    time.Since(start),
		TokensIn:   tin,
		TokensOut:  tout,
		AccountID:  acc.ID,
		Model:      model,
	}, nil
}

func applyUpstreamAuth(req *http.Request, acc *store.Account, cred secrets.CredentialJSON, token string, openaiAuth bool, authHeader, bearerPrefix string) {
	oauth := strings.EqualFold(acc.AuthType, "oauth")
	v := strings.ToLower(acc.Vendor)
	switch {
	case oauth && (v == "claude" || v == "anthropic"):
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	case oauth && (v == "codex" || v == "openai"):
		req.Header.Set("Authorization", "Bearer "+token)
		if cred.ChatGPTAccountID != "" {
			req.Header.Set("ChatGPT-Account-Id", cred.ChatGPTAccountID)
		}
		req.Header.Set("OpenAI-Beta", "responses=experimental")
		req.Header.Set("originator", "codex_cli_rs")
		req.Header.Set("User-Agent", "codex_cli_rs/0.0.1")
		req.Header.Set("Accept", "text/event-stream")
	case openaiAuth:
		req.Header.Set(authHeader, bearerPrefix+token)
	default:
		req.Header.Set(authHeader, token)
		req.Header.Set("anthropic-version", "2023-06-01")
	}
}

func parseUsage(body []byte) (int, int) {
	var u struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			InputTokens      int `json:"input_tokens"`
			OutputTokens     int `json:"output_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(body, &u)
	tin := u.Usage.PromptTokens
	if tin == 0 {
		tin = u.Usage.InputTokens
	}
	tout := u.Usage.CompletionTokens
	if tout == 0 {
		tout = u.Usage.OutputTokens
	}
	return tin, tout
}
