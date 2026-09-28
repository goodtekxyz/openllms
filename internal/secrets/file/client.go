// Package file stores secrets on the local filesystem (OSS / self-host default).
package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Client implements secrets.Client using one file per secret under Root.
// Layout: {Root}/{path...}/{name}.secret (path segments are sanitized).
type Client struct {
	Root string

	mu sync.Mutex
}

// New returns a file-backed client. root must be an absolute or relative directory.
// The directory is created on first Put if missing.
func New(root string) (*Client, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("secrets file root is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Client{Root: abs}, nil
}

func (c *Client) filePath(path, name string) (string, error) {
	rel, err := safeRel(path, name)
	if err != nil {
		return "", err
	}
	full := filepath.Join(c.Root, rel)
	// Ensure full stays under Root (defense in depth).
	relToRoot, err := filepath.Rel(c.Root, full)
	if err != nil || strings.HasPrefix(relToRoot, "..") {
		return "", fmt.Errorf("secret path escapes root")
	}
	return full, nil
}

func safeRel(path, name string) (string, error) {
	path = strings.TrimSpace(path)
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("secret name is empty")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || name == "." || name == ".." {
		return "", fmt.Errorf("invalid secret name")
	}
	var parts []string
	for _, p := range strings.Split(path, "/") {
		p = strings.TrimSpace(p)
		if p == "" || p == "." {
			continue
		}
		if p == ".." || strings.Contains(p, "\\") {
			return "", fmt.Errorf("invalid secret path segment")
		}
		parts = append(parts, p)
	}
	parts = append(parts, name+".secret")
	return filepath.Join(parts...), nil
}

func (c *Client) Put(_ context.Context, path, name, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	fp, err := c.filePath(path, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fp), 0o700); err != nil {
		return err
	}
	tmp := fp + ".tmp"
	if err := os.WriteFile(tmp, []byte(value), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, fp)
}

func (c *Client) Get(_ context.Context, path, name string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fp, err := c.filePath(path, name)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("secret not found")
		}
		return "", err
	}
	return string(b), nil
}

// Delete removes the secret if present. Missing keys are not an error.
func (c *Client) Delete(_ context.Context, path, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	fp, err := c.filePath(path, name)
	if err != nil {
		return err
	}
	err = os.Remove(fp)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
