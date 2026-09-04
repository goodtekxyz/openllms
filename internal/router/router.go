package router

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goodtekxyz/openllms/internal/store"
	"github.com/google/uuid"
)

type Strategy string

const (
	Sequential Strategy = "sequential"
	RoundRobin Strategy = "round_robin"
	Weighted   Strategy = "weighted"
	QuotaAware Strategy = "quota_aware"
	Parallel   Strategy = "parallel"
)

// ApplyPreset maps friendly names to strategy + optional weight hints in route config.
func ApplyPreset(name string) (Strategy, map[string]any) {
	switch name {
	case "failover":
		return Sequential, map[string]any{"preset": "failover"}
	case "balance":
		return RoundRobin, map[string]any{"preset": "balance"}
	case "prefer-primary":
		return Weighted, map[string]any{"preset": "prefer-primary", "primary_weight": 80, "secondary_weight": 20}
	case "quota-first", "quota_aware":
		return QuotaAware, map[string]any{"preset": "quota-first"}
	case "parallel", "race":
		return Parallel, map[string]any{"preset": "parallel"}
	default:
		return Sequential, map[string]any{}
	}
}

// PresetFromRoute recovers the friendly preset name from strategy + config.
func PresetFromRoute(strategy string, configJSON []byte) string {
	var cfg map[string]any
	_ = json.Unmarshal(configJSON, &cfg)
	if p, ok := cfg["preset"].(string); ok && p != "" {
		return p
	}
	switch Strategy(strategy) {
	case RoundRobin:
		return "balance"
	case Weighted:
		return "prefer-primary"
	case QuotaAware:
		return "quota-first"
	case Parallel:
		return "parallel"
	default:
		return "failover"
	}
}

type Selector struct {
	mu       sync.Mutex
	rr       map[uuid.UUID]*uint64 // routeID -> counter
	rand     *randSource
	softFail map[uuid.UUID]time.Time // accountID -> last soft failure
}

type randSource struct {
	mu sync.Mutex
	n  uint64
}

func (r *randSource) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	r.mu.Lock()
	r.n++
	v := r.n
	r.mu.Unlock()
	// Mix for decent distribution without importing math/rand races in tests.
	v ^= v << 13
	v ^= v >> 7
	v ^= v << 17
	return int(v % uint64(n))
}

func NewSelector() *Selector {
	return &Selector{
		rr:       map[uuid.UUID]*uint64{},
		rand:     &randSource{n: uint64(time.Now().UnixNano())},
		softFail: map[uuid.UUID]time.Time{},
	}
}

const SoftFailWindow = 30 * time.Second

// NoteSoftFail records a recent failure for soft deprioritization (not hard exclude).
func (s *Selector) NoteSoftFail(accountID uuid.UUID) {
	s.mu.Lock()
	s.softFail[accountID] = time.Now()
	s.mu.Unlock()
}

func (s *Selector) ClearSoftFail(accountID uuid.UUID) {
	s.mu.Lock()
	delete(s.softFail, accountID)
	s.mu.Unlock()
}

func (s *Selector) recentlyFailed(accountID uuid.UUID, now time.Time) bool {
	t, ok := s.softFail[accountID]
	if !ok {
		return false
	}
	return now.Sub(t) < SoftFailWindow
}

// PreferAccount moves prefer to the front when present (soft sticky).
func PreferAccount(accounts []store.Account, prefer uuid.UUID) []store.Account {
	if prefer == uuid.Nil || len(accounts) < 2 {
		return accounts
	}
	out := make([]store.Account, 0, len(accounts))
	var found *store.Account
	for i := range accounts {
		if accounts[i].ID == prefer {
			cp := accounts[i]
			found = &cp
			continue
		}
		out = append(out, accounts[i])
	}
	if found == nil {
		return accounts
	}
	return append([]store.Account{*found}, out...)
}

// Exclude removes accounts whose IDs are in tried.
func Exclude(accounts []store.Account, tried map[uuid.UUID]struct{}) []store.Account {
	if len(tried) == 0 {
		return accounts
	}
	out := make([]store.Account, 0, len(accounts))
	for _, a := range accounts {
		if _, ok := tried[a.ID]; ok {
			continue
		}
		out = append(out, a)
	}
	return out
}

// PreferFresh puts accounts without recent soft failures first (stable within each group).
func (s *Selector) PreferFresh(accounts []store.Account) []store.Account {
	if len(accounts) < 2 {
		return accounts
	}
	now := time.Now()
	fresh := make([]store.Account, 0, len(accounts))
	stale := make([]store.Account, 0, len(accounts))
	for _, a := range accounts {
		if s.recentlyFailed(a.ID, now) {
			stale = append(stale, a)
		} else {
			fresh = append(fresh, a)
		}
	}
	return append(fresh, stale...)
}

func (s *Selector) Pick(routeID uuid.UUID, strategy Strategy, accounts []store.Account) *store.Account {
	if len(accounts) == 0 {
		return nil
	}
	accounts = s.PreferFresh(accounts)
	switch strategy {
	case RoundRobin:
		return s.pickRR(routeID, accounts)
	case Weighted:
		return s.pickWeighted(accounts)
	case QuotaAware:
		return s.pickQuotaAware(accounts)
	default:
		return &accounts[0]
	}
}

func (s *Selector) pickQuotaAware(accounts []store.Account) *store.Account {
	best := &accounts[0]
	bestPct := quotaPct(accounts[0])
	for i := 1; i < len(accounts); i++ {
		p := quotaPct(accounts[i])
		if p > bestPct {
			best = &accounts[i]
			bestPct = p
		}
	}
	return best
}

func quotaPct(a store.Account) float64 {
	if a.QuotaRemainingPct == nil {
		return -1 // unknown last vs known
	}
	return *a.QuotaRemainingPct
}

func (s *Selector) pickRR(routeID uuid.UUID, accounts []store.Account) *store.Account {
	s.mu.Lock()
	c, ok := s.rr[routeID]
	if !ok {
		var z uint64
		c = &z
		s.rr[routeID] = c
	}
	s.mu.Unlock()
	n := atomic.AddUint64(c, 1) - 1
	return &accounts[int(n%uint64(len(accounts)))]
}

func (s *Selector) pickWeighted(accounts []store.Account) *store.Account {
	total := 0
	for _, a := range accounts {
		w := a.Weight
		if w <= 0 {
			w = 1
		}
		total += w
	}
	r := s.rand.Intn(total)
	for i := range accounts {
		w := accounts[i].Weight
		if w <= 0 {
			w = 1
		}
		if r < w {
			return &accounts[i]
		}
		r -= w
	}
	return &accounts[len(accounts)-1]
}

func IsRetryableStatus(code int) bool {
	return code == 429 || code >= 500
}

// CooldownForStatus returns hard-cooldown duration. backoffLevel increases for repeated 429s (0-based).
func CooldownForStatus(httpStatus int, backoffLevel int) time.Duration {
	switch httpStatus {
	case 401:
		return 5 * time.Minute
	case 402, 403:
		return 30 * time.Minute
	case 429:
		return ExponentialBackoff(backoffLevel)
	case 500, 502, 504:
		return 10 * time.Second
	case 503:
		return 30 * time.Second
	default:
		if httpStatus >= 400 {
			return 30 * time.Second
		}
		return 5 * time.Second
	}
}

func ExponentialBackoff(level int) time.Duration {
	if level < 0 {
		level = 0
	}
	d := time.Duration(math.Pow(2, float64(level))) * time.Second
	if d > 2*time.Minute {
		d = 2 * time.Minute
	}
	return d
}

// CooldownFromHeaders prefers Retry-After (seconds or HTTP date); else status defaults.
func CooldownFromHeaders(status int, hdr http.Header, backoffLevel int) time.Duration {
	if hdr != nil {
		if ra := strings.TrimSpace(hdr.Get("Retry-After")); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil && secs >= 0 {
				d := time.Duration(secs) * time.Second
				if d < time.Second {
					d = time.Second
				}
				if d > 2*time.Hour {
					d = 2 * time.Hour
				}
				return d
			}
			if t, err := http.ParseTime(ra); err == nil {
				d := time.Until(t)
				if d < time.Second {
					d = time.Second
				}
				if d > 2*time.Hour {
					d = 2 * time.Hour
				}
				return d
			}
		}
	}
	return CooldownForStatus(status, backoffLevel)
}

const DefaultCooldown = 30 * time.Second
