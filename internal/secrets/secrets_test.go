package secrets_test

import (
	"testing"

	"github.com/goodtekxyz/openllms/internal/secrets"
)

func TestBearerToken(t *testing.T) {
	if (secrets.CredentialJSON{APIKey: "a"}).BearerToken() != "a" {
		t.Fatal("api_key")
	}
	if (secrets.CredentialJSON{AccessToken: "b"}).BearerToken() != "b" {
		t.Fatal("access_token")
	}
}
