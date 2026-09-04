//go:build !cloud

package httpserver

func (s *Server) paymentRails() map[string]any {
	return map[string]any{
		"polar": false,
		"unifi": false,
		"mock":  s.cfg.BillingMock,
	}
}
