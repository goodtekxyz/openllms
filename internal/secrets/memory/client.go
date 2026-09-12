package memory

import (
	"context"
	"fmt"
	"sync"
)

// Client is an in-process secrets store for tests (implements secrets.Client).
type Client struct {
	mu   sync.Mutex
	data map[string]string
}

func New() *Client {
	return &Client{data: map[string]string{}}
}

func key(path, name string) string { return path + "\x00" + name }

func (c *Client) Put(_ context.Context, path, name, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key(path, name)] = value
	return nil
}

func (c *Client) Get(_ context.Context, path, name string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[key(path, name)]
	if !ok {
		return "", fmt.Errorf("secret not found")
	}
	return v, nil
}

// Delete removes the secret if present. Missing keys are not an error.
func (c *Client) Delete(_ context.Context, path, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key(path, name))
	return nil
}
