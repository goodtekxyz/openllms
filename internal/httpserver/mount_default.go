//go:build !cloud

package httpserver

import "github.com/go-chi/chi/v5"

func (s *Server) mountCloudRoutes(r chi.Router) {
	// Public billing status/trial UI (no Polar/Unifi processors).
	r.Get("/billing", s.handleBillingPage)
	r.Get("/billing/", s.handleBillingPage)
	r.Route("/billing/api", func(br chi.Router) {
		br.Post("/logout", s.handleBillingLogout)
		br.Get("/status", s.handleBillingStatus)
		br.Post("/trial", s.handleBillingStartTrial)
	})
}
