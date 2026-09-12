//go:build !cloud

package httpserver

import "context"

func (s *Server) notifySignup(ctx context.Context, login, githubID, projectID string) {
	_ = ctx
	_ = login
	_ = githubID
	_ = projectID
}
