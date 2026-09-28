package httpserver

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// handlePublicDist serves prebuilt CLI binaries from DistDir at /dist/{name}.
// Names are basenames only (llms_darwin_arm64, VERSION, COMMIT).
func (s *Server) handlePublicDist(w http.ResponseWriter, r *http.Request) {
	dir := strings.TrimSpace(s.cfg.DistDir)
	if dir == "" {
		http.NotFound(w, r)
		return
	}
	name := chi.URLParam(r, "name")
	if name == "" {
		name = filepath.Base(strings.TrimPrefix(r.URL.Path, "/dist/"))
	}
	if !safeDistName(name) {
		http.NotFound(w, r)
		return
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		http.Error(w, "dist unavailable", http.StatusServiceUnavailable)
		return
	}
	absFile, err := filepath.Abs(filepath.Join(absDir, name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rel, err := filepath.Rel(absDir, absFile)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}
	st, err := os.Stat(absFile)
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	http.ServeFile(w, r, absFile)
}

func (s *Server) handlePublicDistIndex(w http.ResponseWriter, r *http.Request) {
	dir := strings.TrimSpace(s.cfg.DistDir)
	if dir == "" {
		http.NotFound(w, r)
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	base := s.cfg.PublicBaseURL
	if base == "" {
		base = "https://llms.goodtek.xyz"
	}
	var b strings.Builder
	b.WriteString("# llms CLI dist\n\n")
	for _, e := range entries {
		if e.IsDir() || !safeDistName(e.Name()) {
			continue
		}
		b.WriteString(base)
		b.WriteString("/dist/")
		b.WriteString(e.Name())
		b.WriteByte('\n')
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_, _ = w.Write([]byte(b.String()))
}

func safeDistName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.' {
			continue
		}
		return false
	}
	return true
}

func (s *Server) distMeta() map[string]any {
	dir := strings.TrimSpace(s.cfg.DistDir)
	base := s.cfg.PublicBaseURL
	if base == "" {
		base = "https://llms.goodtek.xyz"
	}
	out := map[string]any{
		"cli_dist": base + "/dist",
	}
	if dir == "" {
		out["cli_dist_ready"] = false
		return out
	}
	ver, _ := os.ReadFile(filepath.Join(dir, "VERSION"))
	commit, _ := os.ReadFile(filepath.Join(dir, "COMMIT"))
	out["cli_version"] = strings.TrimSpace(string(ver))
	out["cli_commit"] = strings.TrimSpace(string(commit))
	_, err := os.Stat(filepath.Join(dir, "llms_linux_arm64"))
	out["cli_dist_ready"] = err == nil
	return out
}
