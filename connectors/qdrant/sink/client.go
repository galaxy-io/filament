package sink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/galaxy-io/filament/connectors/internal/embedding"
)

// Client handles interaction with the Qdrant REST API or mock provider.
type Client struct {
	cfg        Config
	httpClient *http.Client
	mu         sync.Mutex
	store      map[string]map[string]embedding.VectorDoc
}

// NewClient initializes a Qdrant client instance.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		store: make(map[string]map[string]embedding.VectorDoc),
	}
}

// Health verifies reachability to the Qdrant cluster endpoint.
func (c *Client) Health(ctx context.Context) error {
	if strings.TrimSpace(c.cfg.URL) == "" {
		return fmt.Errorf("qdrant client: invalid empty URL")
	}

	if strings.HasPrefix(c.cfg.URL, "mock://") {
		return nil
	}

	reqURL := strings.TrimSuffix(c.cfg.URL, "/") + "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("qdrant client: prepare health check request: %w", err)
	}

	if c.cfg.APIKey != "" {
		req.Header.Set("api-key", c.cfg.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant client: health check failed for %q: %w", c.cfg.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("qdrant client: health check returned error status %d", resp.StatusCode)
	}

	return nil
}

// BatchUpsert writes a batch of vector points into the target Qdrant collection.
func (c *Client) BatchUpsert(ctx context.Context, collection string, docs []embedding.VectorDoc) error {
	if len(docs) == 0 {
		return nil
	}

	if strings.HasPrefix(c.cfg.URL, "mock://") {
		c.mu.Lock()
		defer c.mu.Unlock()

		if collection == "" {
			collection = "default"
		}
		if _, ok := c.store[collection]; !ok {
			c.store[collection] = make(map[string]embedding.VectorDoc)
		}
		for _, doc := range docs {
			c.store[collection][doc.ID] = doc
		}
		return nil
	}

	points := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		payload := map[string]interface{}{
			"document": doc.Document,
		}
		for k, v := range doc.Metadata {
			payload[k] = v
		}

		pointID := formatQdrantPointID(doc.ID)

		point := map[string]interface{}{
			"id":      pointID,
			"vector":  doc.Vector,
			"payload": payload,
		}
		points = append(points, point)
	}

	body := map[string]interface{}{"points": points}
	return c.sendHTTPRequest(ctx, http.MethodPut, collection, "/points", body)
}

// BatchDelete deletes point IDs from the target Qdrant collection.
func (c *Client) BatchDelete(ctx context.Context, collection string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	if strings.HasPrefix(c.cfg.URL, "mock://") {
		c.mu.Lock()
		defer c.mu.Unlock()

		if collection == "" {
			collection = "default"
		}
		if coll, ok := c.store[collection]; ok {
			for _, id := range ids {
				delete(coll, id)
			}
		}
		return nil
	}

	qdrantIDs := make([]interface{}, len(ids))
	for i, id := range ids {
		qdrantIDs[i] = formatQdrantPointID(id)
	}

	body := map[string]interface{}{"points": qdrantIDs}
	return c.sendHTTPRequest(ctx, http.MethodPost, collection, "/points/delete", body)
}

// DeleteAllDocuments deletes all points from a Qdrant collection for WriteReplace runs.
func (c *Client) DeleteAllDocuments(ctx context.Context, collection string) error {
	if strings.HasPrefix(c.cfg.URL, "mock://") {
		c.mu.Lock()
		defer c.mu.Unlock()

		if collection == "" {
			collection = "default"
		}
		delete(c.store, collection)
		return nil
	}

	body := map[string]interface{}{
		"filter": map[string]interface{}{},
	}
	return c.sendHTTPRequest(ctx, http.MethodPost, collection, "/points/delete", body)
}

// GetDoc returns a point from the mock store for unit testing.
func (c *Client) GetDoc(collection, id string) (embedding.VectorDoc, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if collection == "" {
		collection = "default"
	}

	if coll, ok := c.store[collection]; ok {
		doc, found := coll[id]
		return doc, found
	}
	return embedding.VectorDoc{}, false
}

// formatQdrantPointID formats string IDs as uint64 if numeric, or string UUID.
func formatQdrantPointID(id string) interface{} {
	if val, err := strconv.ParseUint(id, 10, 64); err == nil {
		return val
	}
	return id
}

func (c *Client) sendHTTPRequest(ctx context.Context, method, collection, path string, payload interface{}) error {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("qdrant client: marshal payload: %w", err)
	}

	baseURL := c.cfg.URL
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}

	escapedCollection := url.PathEscape(collection)
	reqURL := fmt.Sprintf("%s/collections/%s%s", strings.TrimSuffix(baseURL, "/"), escapedCollection, path)

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("qdrant client: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("api-key", c.cfg.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant client: send request to %q: %w", reqURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("qdrant client: endpoint %q returned error status %d", reqURL, resp.StatusCode)
	}

	return nil
}
