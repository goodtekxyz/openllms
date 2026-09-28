package httpserver

import (
	"net/http"
)

// headAsGet lets chi routes registered with Get also answer HEAD.
// Search Console and many crawlers probe with HEAD; chi returns 405 otherwise.
func headAsGet(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			r2 := r.Clone(r.Context())
			r2.Method = http.MethodGet
			next.ServeHTTP(&headResponseWriter{ResponseWriter: w}, r2)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type headResponseWriter struct {
	http.ResponseWriter
}

func (h *headResponseWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (h *headResponseWriter) Flush() {
	if f, ok := h.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
