//go:build !cloud

package httpserver

import "github.com/go-chi/chi/v5"

func (s *Server) mountCloudRoutes(r chi.Router) {
	// Public billing status/Free UI (no Polar/Unifi processors on OSS builds).
	r.Get("/billing", s.handleBillingPage)
	r.Get("/billing/", s.handleBillingPage)
	r.Route("/billing/api", func(br chi.Router) {
		br.Post("/logout", s.handleBillingLogout)
		br.Get("/status", s.handleBillingStatus)
		br.Post("/free", s.handleBillingStartFree)
		br.Post("/trial", s.handleBillingStartTrial) // legacy alias
		br.Get("/crew-signal", s.handleBillingCrewSignalGet)
		br.Post("/crew-signal", s.handleBillingCrewSignalPost)
	})
}
