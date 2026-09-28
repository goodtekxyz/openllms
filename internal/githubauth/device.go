package githubauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// DeviceStart begins GitHub device flow.
func DeviceStart(ctx context.Context, clientID string) (deviceCode, userCode, verifyURL string, interval int, err error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", "read:user")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/device/code", strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", 0, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return "", "", "", 0, fmt.Errorf("device code: %s %s", res.Status, b)
	}
	var out struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		Interval        int    `json:"interval"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", "", "", 0, err
	}
	if out.Interval <= 0 {
		out.Interval = 5
	}
	return out.DeviceCode, out.UserCode, out.VerificationURI, out.Interval, nil
}

func DevicePoll(ctx context.Context, clientID, deviceCode string) (accessToken string, err error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("device_code", deviceCode)
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", fmt.Errorf("token response not json (http %s, %d bytes)", res.Status, len(b))
	}
	if out.Error != "" {
		return "", fmt.Errorf("%s", out.Error)
	}
	if out.AccessToken == "" {
		snip := string(b)
		if len(snip) > 200 {
			snip = snip[:200]
		}
		return "", fmt.Errorf("no token: %s", snip)
	}
	return out.AccessToken, nil
}

func FetchUser(ctx context.Context, accessToken string) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("github user: %s %s", res.Status, b)
	}
	var u User
	if err := json.Unmarshal(b, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func PollUntilToken(ctx context.Context, clientID, deviceCode string, intervalSec int) (string, error) {
	if intervalSec <= 0 {
		intervalSec = 5
	}
	fmt.Fprintln(os.Stderr, "Waiting for GitHub authorization — after the code, click Authorize.")
	interval := time.Duration(intervalSec) * time.Second
	deadline := time.Now().Add(15 * time.Minute)
	next := time.Now().Add(interval) // GitHub: wait `interval` before the first poll
	lastNote := time.Time{}
	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("device code expired — run llms login again")
		}
		if wait := time.Until(next); wait > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(wait):
			}
		}
		tok, err := DevicePoll(ctx, clientID, deviceCode)
		if err == nil {
			return tok, nil
		}
		switch err.Error() {
		case "authorization_pending":
			if time.Since(lastNote) >= 15*time.Second {
				fmt.Fprintln(os.Stderr, "Still waiting for Authorize on GitHub…")
				lastNote = time.Now()
			}
			next = time.Now().Add(interval)
		case "slow_down":
			interval += 5 * time.Second
			fmt.Fprintf(os.Stderr, "GitHub asked to slow down; polling every %s\n", interval)
			next = time.Now().Add(interval)
		case "expired_token":
			return "", fmt.Errorf("device code expired — run llms login again")
		case "access_denied":
			return "", fmt.Errorf("authorization cancelled on GitHub — run llms login again")
		default:
			return "", err
		}
	}
}
