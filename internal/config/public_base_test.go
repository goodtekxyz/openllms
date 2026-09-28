//go:build !cloud

package config

import "testing"

func TestDefaultPublicBaseURLIsLocalForSelfHost(t *testing.T) {
	if defaultPublicBaseURL != "http://127.0.0.1:8080" {
		t.Fatalf("self-hosted default must not point at the hosted cloud: %q", defaultPublicBaseURL)
	}
}
