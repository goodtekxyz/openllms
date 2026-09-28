//go:build !cloud

package config

// Self-hosted default: the local gateway (deploy/oss binds 127.0.0.1:8080).
const defaultPublicBaseURL = "http://127.0.0.1:8080"
