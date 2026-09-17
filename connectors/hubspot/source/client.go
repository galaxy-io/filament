package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/time/rate"

	"github.com/galaxy-io/filament"
)

// Pin each API family explicitly; versions are not negotiated on an error.
const apiVersion = "2026-09"

type client struct {
	http    *http.Client
	baseURL string
	token   string
	limiter *rate.Limiter
	observe filament.SourceObserver
}

func newClient(token string) *client {
	return &client{
		http: &http.Client{Timeout: 60 * time.Second, Transport: http.DefaultTransport.(*http.Transport).Clone()}, baseURL: "https://api.hubapi.com",
		token: token, limiter: rate.NewLimiter(5, 1),
	}
}

type apiError struct {
	Status        int    `json:"-"`
	Category      string `json:"category"`
	CorrelationID string `json:"correlationId"`
}

func (e *apiError) Error() string {
	// API messages may contain submitted values. Keep diagnostics structural.
	return fmt.Sprintf("HubSpot HTTP %d (%s, correlation %s)", e.Status, e.Category, e.CorrelationID)
}

func (c *client) request(ctx context.Context, resource, method, path string, input, output any) error {
	var body []byte
	if input != nil {
		var err error
		body, err = json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}
	for attempt := range 10 {
		if err := c.limiter.Wait(ctx); err != nil {
			return err
		}
		resp, err := c.send(ctx, method, path, body)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if err := json.Unmarshal(data, output); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			return nil
		}
		failure := &apiError{Status: resp.StatusCode}
		_ = json.Unmarshal(data, failure)
		if attempt == 9 || (resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500) {
			return failure
		}
		delay := retryDelay(resp.Header.Get("Retry-After"), attempt, time.Now())
		if c.observe != nil && resp.StatusCode == http.StatusTooManyRequests {
			c.observe(filament.SourceProgress{Kind: filament.SourceProgressRateLimited, Resource: resource, RetryAfter: delay})
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func (c *client) send(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}

func retryDelay(value string, attempt int, now time.Time) time.Duration {
	if seconds, err := strconv.ParseInt(value, 10, 32); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil && date.After(now) {
		return date.Sub(now)
	}
	return time.Second << min(attempt, 5)
}
