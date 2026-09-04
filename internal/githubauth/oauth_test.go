package githubauth

import (
	"net/url"
	"strings"
	"testing"
)

func TestAuthorizeURL(t *testing.T) {
	u := AuthorizeURL("cid", "https://llms.example/auth/github/callback", "st", "")
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != "github.com" || !strings.HasSuffix(parsed.Path, "/login/oauth/authorize") {
		t.Fatalf("host/path: %s", u)
	}
	q := parsed.Query()
	if q.Get("client_id") != "cid" || q.Get("state") != "st" || q.Get("scope") != DefaultScope {
		t.Fatalf("query: %v", q)
	}
	if q.Get("redirect_uri") != "https://llms.example/auth/github/callback" {
		t.Fatalf("redirect_uri: %s", q.Get("redirect_uri"))
	}
}
