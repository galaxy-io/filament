package webhooks

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/galaxy-io/filament"
)

const (
	defaultCacheTTL    = 15 * time.Second
	defaultNegativeTTL = 5 * time.Second
	defaultLoadTimeout = 5 * time.Second
	maxCacheEntries    = 10_000
)

// Resolved is an endpoint with its signing keys already read.
type Resolved struct {
	Endpoint Endpoint
	Keys     [][]byte
}

// EndpointCache keeps recently used endpoints and their keys in memory, so a
// delivery does not cost a database query and a secret read. Concurrent misses
// for one endpoint share a single load, and unknown IDs are remembered briefly.
//
// A cached entry can trail a configuration change by at most TTL. That is
// safe: a replaced key keeps verifying for SecondaryKeyGrace, and deliveries
// carry no routing, so a stale entry never misroutes one.
//
// A shared load is detached from the request that started it and bounded by
// LoadTimeout. Each caller still stops waiting when its own request ends, so
// a slow database or secret store cannot pin receiver slots.
type EndpointCache struct {
	Loader      EndpointLoader
	Secrets     filament.Secrets
	TTL         time.Duration
	NegativeTTL time.Duration
	LoadTimeout time.Duration
	Now         func() time.Time

	group   singleflight.Group
	mu      sync.Mutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	resolved Resolved
	missing  bool
	expires  time.Time
}

// Resolve returns the endpoint and its active keys, or filament.ErrNotFound.
func (c *EndpointCache) Resolve(ctx context.Context, sourceID string) (Resolved, error) {
	now := c.now()
	c.mu.Lock()
	entry, ok := c.entries[sourceID]
	c.mu.Unlock()
	if ok && now.Before(entry.expires) {
		if entry.missing {
			return Resolved{}, filament.ErrNotFound
		}
		return entry.resolved, nil
	}
	result := c.group.DoChan(sourceID, func() (any, error) {
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.ttl(c.LoadTimeout, defaultLoadTimeout))
		defer cancel()
		resolved, err := c.load(loadCtx, sourceID)
		switch {
		case err == nil:
			c.store(sourceID, cacheEntry{resolved: resolved, expires: c.now().Add(c.ttl(c.TTL, defaultCacheTTL))})
		case errors.Is(err, filament.ErrNotFound):
			c.store(sourceID, cacheEntry{missing: true, expires: c.now().Add(c.ttl(c.NegativeTTL, defaultNegativeTTL))})
		}
		return resolved, err
	})
	select {
	case <-ctx.Done():
		return Resolved{}, context.Cause(ctx)
	case r := <-result:
		if r.Err != nil {
			return Resolved{}, r.Err
		}
		return r.Val.(Resolved), nil
	}
}

// Invalidate drops one endpoint so the next delivery reloads it.
func (c *EndpointCache) Invalidate(sourceID string) {
	c.mu.Lock()
	delete(c.entries, sourceID)
	c.mu.Unlock()
	c.group.Forget(sourceID)
}

func (c *EndpointCache) load(ctx context.Context, sourceID string) (Resolved, error) {
	endpoint, err := c.Loader.LoadWebhookEndpoint(ctx, sourceID)
	if err != nil {
		return Resolved{}, err
	}
	if endpoint.SourceID != sourceID || endpoint.Tenant == "" {
		return Resolved{}, filament.ErrNotFound
	}
	refs := endpoint.ActiveKeyRefs(c.now())
	if len(refs) == 0 {
		return Resolved{}, errors.New("webhook endpoint has no signing key")
	}
	keys := make([][]byte, 0, len(refs))
	for _, ref := range refs {
		secret, err := c.Secrets.Read(ctx, ref)
		if err != nil {
			return Resolved{}, err
		}
		if len(secret.Value) == 0 || (secret.Tenant != "" && secret.Tenant != endpoint.Tenant) {
			return Resolved{}, errors.New("webhook signing key does not belong to the endpoint tenant")
		}
		keys = append(keys, secret.Value)
	}
	return Resolved{Endpoint: endpoint, Keys: keys}, nil
}

func (c *EndpointCache) store(sourceID string, entry cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]cacheEntry{}
	}
	if len(c.entries) >= maxCacheEntries {
		// Unknown IDs go first, so a flood of them cannot evict real endpoints.
		now := c.now()
		for id, e := range c.entries {
			if e.missing || !now.Before(e.expires) {
				delete(c.entries, id)
			}
		}
	}
	c.entries[sourceID] = entry
}

func (c *EndpointCache) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (*EndpointCache) ttl(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}
