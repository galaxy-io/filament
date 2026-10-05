package export

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/template"
)

func validateExportURL(location string, hosts []string) (*url.URL, error) {
	u, err := url.Parse(location)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("export download requires an HTTPS URL without userinfo or fragment")
	}
	if !slices.ContainsFunc(hosts, func(h string) bool { return strings.EqualFold(h, u.Hostname()) }) {
		return nil, fmt.Errorf("export download host is not allowed")
	}
	return u, nil
}

func (c *Runtime) downloadExport(ctx context.Context, res manifest.Resource, location string, scope template.Scope) (*http.Response, error) {
	spec := res.Export
	// Use the stream transport only: no API auth, connection headers, cookie jar,
	// or whole-response timeout. The export deadline bounds the transfer.
	client := http.Client{Transport: c.Transport}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many export download redirects")
		}
		_, err := c.validateDownloadURL(req.URL.String(), spec.Result)
		return err
	}
	failures := 0
	refreshed := false
	for {
		u, err := c.validateDownloadURL(location, spec.Result)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
		if err != nil {
			return nil, fmt.Errorf("invalid export download request")
		}
		if spec.Result.Auth == "connection" {
			if c.WaitDownload != nil {
				if err := c.WaitDownload(ctx); err != nil {
					return nil, err
				}
			}
			if c.Authenticate == nil {
				return nil, fmt.Errorf("export download requires connection authentication")
			}
			if err := c.Authenticate(ctx, req, scope); err != nil {
				return nil, fmt.Errorf("export download authentication failed")
			}
		}
		// File compression is manifest-controlled. Avoid implicit HTTP decompression.
		req.Header.Set("Accept-Encoding", "identity")
		resp, err := client.Do(req)
		if err != nil {
			return nil, TransportError(ctx, err)
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		_ = resp.Body.Close()
		if spec.Wait.Type == "download" && slices.Contains(spec.Wait.PendingStatuses, resp.StatusCode) {
			delay := time.Duration(spec.Wait.IntervalSeconds) * time.Second
			if resp.Header.Get("Retry-After") != "" {
				delay = max(delay, c.RetryAfter(resp, 0))
			}
			if err := sleepCtx(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}
		// A job endpoint can re-issue an expired URL. Refresh once rather than
		// treating all forbidden responses as pending forever.
		if spec.Wait.Type == "job" && !refreshed && (resp.StatusCode == 403 || resp.StatusCode == 410) {
			if err := c.waitExport(ctx, res, scope); err != nil {
				return nil, err
			}
			location, err = template.Render(spec.Result.URL, scope)
			if err != nil {
				return nil, err
			}
			refreshed = true
			continue
		}
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			failures++
			if failures < c.MaxRetries {
				if err := sleepCtx(ctx, c.RetryAfter(resp, failures-1)); err != nil {
					return nil, err
				}
				continue
			}
		}
		return nil, fmt.Errorf("export %s download returned HTTP %d", res.Name, resp.StatusCode)
	}
}

// TransportError preserves cancellation and timeouts without exposing signed URLs.
func TransportError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// net/http URL errors contain the complete signed URL. Preserve a useful
	// error category without exposing that URL or any response body.
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("export request timed out: %w", context.DeadlineExceeded)
	}
	return fmt.Errorf("export HTTP transport failed")
}

// Credentialed artifacts may never leave the configured API origin, even when
// a redirect target appears in allowed_hosts. Signed URLs retain the old policy.
func (c *Runtime) validateDownloadURL(location string, result manifest.ExportResult) (*url.URL, error) {
	if result.Auth != "connection" {
		return validateExportURL(location, result.AllowedHosts)
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return nil, fmt.Errorf("authenticated export requires an HTTPS API origin")
	}
	u, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("invalid authenticated export URL")
	}
	u = base.ResolveReference(u)
	if u.Scheme != base.Scheme || !strings.EqualFold(u.Host, base.Host) || u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("authenticated export download must use the API origin")
	}
	return u, nil
}
