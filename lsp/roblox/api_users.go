package roblox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	usersAPIBaseURL = "https://users.roblox.com"
	errRateLimited  = errors.New("roblox api rate limited")
)

type robloxAPIClient struct {
	httpClient *http.Client

	mu              sync.Mutex
	csrfToken       string
	rateLimitedTill time.Time
}

func newRobloxAPIClient() *robloxAPIClient {
	return &robloxAPIClient{
		httpClient: &http.Client{},
	}
}

func (c *robloxAPIClient) doJSON(
	ctx context.Context,
	cfg ProviderConfig,
	method string,
	rawURL string,
	requestBody interface{},
	responseBody interface{},
	withCookie bool,
	allowCSRFRetry bool,
) (http.Header, error) {
	effectiveCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && cfg.Timeout > 0 {
		var cancel context.CancelFunc
		effectiveCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	var payload []byte
	var err error
	if requestBody != nil {
		payload, err = json.Marshal(requestBody)
		if err != nil {
			return nil, err
		}
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := c.waitRateLimit(effectiveCtx); err != nil {
			return nil, err
		}
		status, headers, body, reqErr := c.doJSONOnce(effectiveCtx, cfg, method, rawURL, payload, withCookie, c.getCSRFToken())
		if reqErr != nil {
			lastErr = reqErr
			if attempt == 0 && effectiveCtx.Err() == nil {
				if sleepErr := sleepWithContext(effectiveCtx, 250*time.Millisecond); sleepErr == nil {
					continue
				}
			}
			return headers, reqErr
		}

		if status == http.StatusForbidden && allowCSRFRetry {
			token := strings.TrimSpace(headers.Get("x-csrf-token"))
			if token != "" && token != c.getCSRFToken() {
				c.setCSRFToken(token)
				status, headers, body, reqErr = c.doJSONOnce(effectiveCtx, cfg, method, rawURL, payload, withCookie, token)
				if reqErr != nil {
					lastErr = reqErr
					if attempt == 0 && effectiveCtx.Err() == nil {
						if sleepErr := sleepWithContext(effectiveCtx, 250*time.Millisecond); sleepErr == nil {
							continue
						}
					}
					return headers, reqErr
				}
			}
		}

		if status == http.StatusTooManyRequests {
			c.setRateLimit(2 * time.Second)
			lastErr = errRateLimited
			if attempt == 0 && effectiveCtx.Err() == nil {
				if sleepErr := sleepWithContext(effectiveCtx, 500*time.Millisecond); sleepErr == nil {
					continue
				}
			}
			return headers, errRateLimited
		}
		if status >= http.StatusInternalServerError && attempt == 0 {
			lastErr = parseRobloxAPIError(status, body)
			if sleepErr := sleepWithContext(effectiveCtx, 350*time.Millisecond); sleepErr == nil {
				continue
			}
		}
		if status >= http.StatusBadRequest {
			return headers, parseRobloxAPIError(status, body)
		}

		if responseBody != nil && len(body) > 0 {
			if err := json.Unmarshal(body, responseBody); err != nil {
				return headers, fmt.Errorf("decode response failed: %w", err)
			}
		}
		return headers, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("roblox request failed")
}

func (c *robloxAPIClient) doJSONOnce(
	ctx context.Context,
	cfg ProviderConfig,
	method string,
	rawURL string,
	payload []byte,
	withCookie bool,
	csrfToken string,
) (int, http.Header, []byte, error) {
	reader := bytes.NewReader(payload)
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if cfg.UserAgent != "" {
		req.Header.Set("User-Agent", cfg.UserAgent)
	}
	if len(payload) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if withCookie && cfg.Roblosecurity != "" {
		req.Header.Set("Cookie", ".ROBLOSECURITY="+cfg.Roblosecurity)
	}
	if csrfToken != "" {
		req.Header.Set("x-csrf-token", csrfToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return resp.StatusCode, resp.Header, nil, err
	}
	return resp.StatusCode, resp.Header, body, nil
}

func (c *robloxAPIClient) setCSRFToken(token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	c.mu.Lock()
	c.csrfToken = token
	c.mu.Unlock()
}

func (c *robloxAPIClient) getCSRFToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.csrfToken
}

func (c *robloxAPIClient) setRateLimit(duration time.Duration) {
	if duration <= 0 {
		return
	}
	c.mu.Lock()
	c.rateLimitedTill = time.Now().Add(duration)
	c.mu.Unlock()
}

func (c *robloxAPIClient) waitRateLimit(ctx context.Context) error {
	c.mu.Lock()
	until := c.rateLimitedTill
	c.mu.Unlock()
	wait := time.Until(until)
	if wait <= 0 {
		return nil
	}
	return sleepWithContext(ctx, wait)
}

func sleepWithContext(ctx context.Context, wait time.Duration) error {
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type robloxErrorResponse struct {
	Errors []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func parseRobloxAPIError(status int, body []byte) error {
	if len(body) > 0 {
		var robloxErr robloxErrorResponse
		if err := json.Unmarshal(body, &robloxErr); err == nil && len(robloxErr.Errors) > 0 {
			msg := strings.TrimSpace(robloxErr.Errors[0].Message)
			if msg != "" {
				return fmt.Errorf("roblox api status=%d message=%s", status, msg)
			}
		}
	}
	return fmt.Errorf("roblox api status=%d", status)
}

type usersByIDResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

func (c *robloxAPIClient) fetchUserByID(ctx context.Context, cfg ProviderConfig, userID int64) (*UserProfile, error) {
	url := fmt.Sprintf("%s/v1/users/%d", usersAPIBaseURL, userID)
	var resp usersByIDResponse
	if _, err := c.doJSON(ctx, cfg, http.MethodGet, url, nil, &resp, false, false); err != nil {
		return nil, err
	}
	if resp.ID <= 0 {
		return nil, fmt.Errorf("roblox user %d not found", userID)
	}
	return &UserProfile{
		ID:          resp.ID,
		Name:        resp.Name,
		DisplayName: resp.DisplayName,
	}, nil
}

type usernameLookupRequest struct {
	Usernames          []string `json:"usernames"`
	ExcludeBannedUsers bool     `json:"excludeBannedUsers"`
}

type usernameLookupResponse struct {
	Data []struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	} `json:"data"`
}

func (c *robloxAPIClient) resolveUserByUsername(ctx context.Context, cfg ProviderConfig, username string) (*UserProfile, error) {
	url := fmt.Sprintf("%s/v1/usernames/users", usersAPIBaseURL)
	req := usernameLookupRequest{
		Usernames:          []string{username},
		ExcludeBannedUsers: true,
	}
	var resp usernameLookupResponse
	if _, err := c.doJSON(ctx, cfg, http.MethodPost, url, req, &resp, false, false); err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 || resp.Data[0].ID <= 0 {
		return nil, fmt.Errorf("roblox username %q not found", username)
	}
	return &UserProfile{
		ID:          resp.Data[0].ID,
		Name:        resp.Data[0].Name,
		DisplayName: resp.Data[0].DisplayName,
	}, nil
}
