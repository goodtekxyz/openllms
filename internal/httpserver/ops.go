package httpserver

import (
	"context"
	"fmt"
	"net/http"
)

// AdminSession is the operator identity used by Cloud admin routes.
type AdminSession struct {
	Login    string
	GitHubID string
}

// AdminAuth is optional Cloud operator auth. OSS builds use a disabled stub.
type AdminAuth interface {
	Enabled() bool
	Issue(w http.ResponseWriter, login, githubID string) error
	Clear(w http.ResponseWriter)
	Session(r *http.Request) (*AdminSession, error)
}

// Mailer is optional Cloud ops mail. OSS builds use a no-op stub.
type Mailer interface {
	Configured() bool
	Send(ctx context.Context, to, subject, textBody string) error
}

type disabledAdmin struct{}

func (disabledAdmin) Enabled() bool { return false }
func (disabledAdmin) Issue(http.ResponseWriter, string, string) error {
	return fmt.Errorf("admin_disabled")
}
func (disabledAdmin) Clear(http.ResponseWriter) {}
func (disabledAdmin) Session(*http.Request) (*AdminSession, error) {
	return nil, fmt.Errorf("admin_disabled")
}

type noopMailer struct{}

func (noopMailer) Configured() bool { return false }
func (noopMailer) Send(context.Context, string, string, string) error {
	return fmt.Errorf("mailer_not_configured")
}
