package sink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/galaxy-io/filament/connectors/internal/embedding"
)

// Client handles interaction with the Milvus REST API or mock transport.
type Client struct {
	cfg        Config
	httpClient *http.Client
	mu         sync.Mutex
	store      map[string]map[string]embedding.VectorDoc
}

// NewClient initializes a Milvus client instance.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		store: make(map[string]map[string]embedding.VectorDoc),
	}
}

// Health verifies connectivity to the Milvus cluster endpoint.
func (c *Client) Health(ctx context.Context) error {
	if strings.TrimSpace(c.cfg.URL) == "" {
		return fmt.Errorf("milvus client: invalid empty URL")
	}

	if strings.HasPrefix(c.cfg.URL, "mock://") {
		return nil
	}

	reqURL := strings.TrimSuffix(c.cfg.URL, "/") + "/v2/vectordb/collections/list"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("milvus client: prepare health check request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("milvus client: health check failed for %q: %w", c.cfg.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("milvus client: health check returned error status %d", resp.StatusCode)
	}

	return nil
}

// BatchUpsert writes a batch of entity vectors into the target Milvus collection.
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

	data := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		row := map[string]interface{}{
			"id":       doc.ID,
			"vector":   doc.Vector,
			"document": doc.Document,
		}
		for k, v := range doc.Metadata {
			row[k] = v
		}
		data = append(data, row)
	}

	body := map[string]interface{}{
		"collectionName": collection,
		"data":           data,
	}
	return c.sendHTTPRequest(ctx, http.MethodPost, "/v2/vectordb/entities/upsert", body)
}

// BatchDelete deletes entity IDs from the target Milvus collection.
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

	quoted := make([]string, len(ids))
	for i, id := range ids {
		b, _ := json.Marshal(id)
		quoted[i] = string(b)
	}
	filterExpr := "id in [" + strings.Join(quoted, ",") + "]"

	body := map[string]interface{}{
		"collectionName": collection,
		"filter":         filterExpr,
	}
	return c.sendHTTPRequest(ctx, http.MethodPost, "/v2/vectordb/entities/delete", body)
}

// DeleteAllDocuments deletes all entities from a Milvus collection for WriteReplace runs.
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
		"collectionName": collection,
		"filter":         "id != ''",
	}
	return c.sendHTTPRequest(ctx, http.MethodPost, "/v2/vectordb/entities/delete", body)
}

// GetDoc returns a document from the mock store for unit testing.
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

func (c *Client) sendHTTPRequest(ctx context.Context, method, path string, payload interface{}) error {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("milvus client: marshal payload: %w", err)
	}

	baseURL := c.cfg.URL
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}

	reqURL := fmt.Sprintf("%s%s", strings.TrimSuffix(baseURL, "/"), path)
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("milvus client: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("milvus client: send request to %q: %w", reqURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("milvus client: endpoint %q returned error status %d", reqURL, resp.StatusCode)
	}

	return nil
}
