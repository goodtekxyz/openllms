package httpserver

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Public assets — no auth. Machines use .md/.sh; humans use HTML (ko/en/ja/zh).
//
//go:embed static/install.sh static/INSTALL.md static/LLMS_API.md static/assets static/i18n static/partials static/admin.html static/billing.html static/console.html static/robots.txt static/sitemap.xml static/llms.txt
var publicStatic embed.FS

var publicHTMLLangs = map[string]struct{}{
	"ko": {}, "en": {}, "ja": {}, "zh": {},
}

func normalizePublicLang(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang == "" {
		return "ko"
	}
	if _, ok := publicHTMLLangs[lang]; ok {
		return lang
	}
	return ""
}

func (s *Server) handlePublicAsset(w http.ResponseWriter, r *http.Request) {
	name := path.Base(chi.URLParam(r, "name"))
	if name == "." || name == "/" || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	ct := "application/octet-stream"
	switch {
	case strings.HasSuffix(name, ".css"):
		ct = "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		ct = "text/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".png"):
		ct = "image/png"
	case strings.HasSuffix(name, ".svg"):
		ct = "image/svg+xml"
	case strings.HasSuffix(name, ".jpg"), strings.HasSuffix(name, ".jpeg"):
		ct = "image/jpeg"
	case strings.HasSuffix(name, ".webp"):
		ct = "image/webp"
	}
	serveEmbedded(w, r, "static/assets/"+name, ct, "inline")
}

func (s *Server) handlePublicInstallSh(w http.ResponseWriter, r *http.Request) {
	serveEmbedded(w, r, "static/install.sh", "text/x-shellscript; charset=utf-8", "attachment; filename=\"install.sh\"")
}

func (s *Server) handlePublicInstallDoc(w http.ResponseWriter, r *http.Request) {
	serveEmbedded(w, r, "static/INSTALL.md", "text/markdown; charset=utf-8", "inline; filename=\"INSTALL.md\"")
}

func (s *Server) handleRobotsTxt(w http.ResponseWriter, r *http.Request) {
	serveEmbedded(w, r, "static/robots.txt", "text/plain; charset=utf-8", "inline")
}

func (s *Server) handleSitemapXML(w http.ResponseWriter, r *http.Request) {
	serveEmbedded(w, r, "static/sitemap.xml", "application/xml; charset=utf-8", "inline")
}

func (s *Server) handleLlmsTxt(w http.ResponseWriter, r *http.Request) {
	serveEmbedded(w, r, "static/llms.txt", "text/plain; charset=utf-8", "inline")
}

func (s *Server) handleLLMSAPIDoc(w http.ResponseWriter, r *http.Request) {
	serveEmbedded(w, r, "static/LLMS_API.md", "text/markdown; charset=utf-8", "inline; filename=\"LLMS_API.md\"")
}

func serveEmbedded(w http.ResponseWriter, r *http.Request, name, contentType, disposition string) {
	b, err := fs.ReadFile(publicStatic, name)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	if disposition != "" {
		w.Header().Set("Content-Disposition", disposition)
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_, _ = w.Write(b)
}
