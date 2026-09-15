package sink

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	// ErrIndexNotFound is returned when an index does not exist in Meilisearch.
	ErrIndexNotFound = errors.New("meilisearch: index not found")
)

// IndexResponse represents the metadata of a Meilisearch index.
type IndexResponse struct {
	UID        string `json:"uid"`
	PrimaryKey string `json:"primaryKey"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

// TaskResponse represents the response when an asynchronous task is enqueued.
type TaskResponse struct {
	TaskUID  int64  `json:"taskUid"`
	IndexUID string `json:"indexUid"`
	Status   string `json:"status"`
	Type     string `json:"type"`
}

// TaskResult describes the status and details of an executed task.
type TaskResult struct {
	UID        int64      `json:"uid"`
	IndexUID   string     `json:"indexUid"`
	Status     string     `json:"status"`
	Type       string     `json:"type"`
	Error      *TaskError `json:"error,omitempty"`
	Duration   string     `json:"duration"`
	EnqueuedAt string     `json:"enqueuedAt"`
	StartedAt  string     `json:"startedAt"`
	FinishedAt string     `json:"finishedAt"`
}

// TaskError holds error details from a failed task.
type TaskError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
	Type    string `json:"type"`
	Link    string `json:"link"`
}

// Client manages communication with the Meilisearch REST API.
type Client struct {
	baseURL     string
	apiKey      string
	gzipEnabled bool
	http        *http.Client
	gzipPool    sync.Pool
	bufPool     sync.Pool
}

// NewClient returns a new Meilisearch HTTP client.
func NewClient(baseURL, apiKey string, gzipEnabled bool) *Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		apiKey:      apiKey,
		gzipEnabled: gzipEnabled,
		http: &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		},
		gzipPool: sync.Pool{
			New: func() any {
				return gzip.NewWriter(io.Discard)
			},
		},
		bufPool: sync.Pool{
			New: func() any {
				return new(bytes.Buffer)
			},
		},
	}
}

// Health checks if the Meilisearch server is running and healthy.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("meilisearch: health request: %w", err)
	}
	c.applyHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("meilisearch: connect to %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("meilisearch: health check failed (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetIndex retrieves metadata for an index by its UID.
func (c *Client) GetIndex(ctx context.Context, uid string) (*IndexResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/indexes/%s", c.baseURL, url.PathEscape(uid)), nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meilisearch: get index %q: %w", uid, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrIndexNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("meilisearch: get index %q failed (%d): %s", uid, resp.StatusCode, string(body))
	}

	var idx IndexResponse
	if err := json.NewDecoder(resp.Body).Decode(&idx); err != nil {
		return nil, fmt.Errorf("meilisearch: decode index %q: %w", uid, err)
	}
	return &idx, nil
}

// CreateIndex creates a new index with the specified primary key.
func (c *Client) CreateIndex(ctx context.Context, uid, primaryKey string) (*TaskResponse, error) {
	payload := map[string]string{"uid": uid}
	if primaryKey != "" {
		payload["primaryKey"] = primaryKey
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/indexes", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meilisearch: create index %q: %w", uid, err)
	}
	defer resp.Body.Close()

	// If index already exists (409 Conflict), ignore.
	if resp.StatusCode == http.StatusConflict {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("meilisearch: create index %q (%d): %s", uid, resp.StatusCode, string(body))
	}

	var task TaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, fmt.Errorf("meilisearch: decode create index %q task: %w", uid, err)
	}
	return &task, nil
}

// DeleteAllDocuments deletes all documents from an index while keeping the index and settings.
func (c *Client) DeleteAllDocuments(ctx context.Context, uid string) (*TaskResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/indexes/%s/documents", c.baseURL, url.PathEscape(uid)), nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meilisearch: delete all documents %q: %w", uid, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("meilisearch: delete all documents %q (%d): %s", uid, resp.StatusCode, string(body))
	}

	var task TaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, fmt.Errorf("meilisearch: decode delete documents task: %w", err)
	}
	return &task, nil
}

// AddDocumentsNDJSON streams an NDJSON payload to the index.
// update=true sends PUT (partial document updates), update=false sends POST (add or replace).
func (c *Client) AddDocumentsNDJSON(ctx context.Context, uid, primaryKey string, ndjsonPayload []byte, update bool) (*TaskResponse, error) {
	if len(ndjsonPayload) == 0 {
		return nil, nil
	}

	method := http.MethodPost
	if update {
		method = http.MethodPut
	}

	endpoint := fmt.Sprintf("%s/indexes/%s/documents", c.baseURL, url.PathEscape(uid))
	if primaryKey != "" {
		endpoint += "?primaryKey=" + url.QueryEscape(primaryKey)
	}

	var bodyReader io.Reader
	var isGzip bool

	if c.gzipEnabled {
		buf := c.bufPool.Get().(*bytes.Buffer)
		buf.Reset()
		defer c.bufPool.Put(buf)

		gz := c.gzipPool.Get().(*gzip.Writer)
		gz.Reset(buf)
		if _, err := gz.Write(ndjsonPayload); err != nil {
			gz.Close()
			c.gzipPool.Put(gz)
			return nil, fmt.Errorf("meilisearch: gzip compress: %w", err)
		}
		if err := gz.Close(); err != nil {
			c.gzipPool.Put(gz)
			return nil, fmt.Errorf("meilisearch: gzip finish: %w", err)
		}
		c.gzipPool.Put(gz)

		bodyReader = bytes.NewReader(buf.Bytes())
		isGzip = true
	} else {
		bodyReader = bytes.NewReader(ndjsonPayload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/x-ndjson")
	if isGzip {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meilisearch: send documents to %q: %w", uid, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("meilisearch: add documents to %q failed (%d): %s", uid, resp.StatusCode, string(body))
	}

	var task TaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, fmt.Errorf("meilisearch: decode task response: %w", err)
	}
	return &task, nil
}

// DeleteDocumentsBatch sends a batch of document IDs to be deleted.
func (c *Client) DeleteDocumentsBatch(ctx context.Context, uid string, docIDs []string) (*TaskResponse, error) {
	if len(docIDs) == 0 {
		return nil, nil
	}

	data, err := json.Marshal(docIDs)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/indexes/%s/documents/delete-batch", c.baseURL, url.PathEscape(uid))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meilisearch: delete documents batch %q: %w", uid, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("meilisearch: delete documents batch %q failed (%d): %s", uid, resp.StatusCode, string(body))
	}

	var task TaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, fmt.Errorf("meilisearch: decode delete-batch task response: %w", err)
	}
	return &task, nil
}

// GetTask retrieves the current status of an asynchronous task.
func (c *Client) GetTask(ctx context.Context, taskUID int64) (*TaskResult, error) {
	endpoint := fmt.Sprintf("%s/tasks/%d", c.baseURL, taskUID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meilisearch: get task %d: %w", taskUID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("meilisearch: get task %d failed (%d): %s", taskUID, resp.StatusCode, string(body))
	}

	var result TaskResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("meilisearch: decode task %d: %w", taskUID, err)
	}
	return &result, nil
}

// WaitForTask blocks until the task reaches a terminal status (succeeded, failed, canceled).
func (c *Client) WaitForTask(ctx context.Context, taskUID int64, timeout time.Duration) (*TaskResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	delay := 50 * time.Millisecond
	const maxDelay = 250 * time.Millisecond

	for {
		task, err := c.GetTask(ctx, taskUID)
		if err != nil {
			return nil, err
		}

		switch task.Status {
		case "succeeded":
			return task, nil
		case "failed":
			if task.Error != nil {
				return nil, fmt.Errorf("meilisearch task %d failed [%s]: %s", taskUID, task.Error.Code, task.Error.Message)
			}
			return nil, fmt.Errorf("meilisearch task %d failed", taskUID)
		case "canceled":
			return nil, fmt.Errorf("meilisearch task %d was canceled", taskUID)
		case "enqueued", "processing":
			// Still running, sleep and retry
		default:
			// Unknown status, assume running
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("meilisearch: timeout waiting for task %d: %w", taskUID, ctx.Err())
		case <-time.After(delay):
			delay = min(delay*2, maxDelay)
		}
	}
}

// WaitForTasks waits for all specified task UIDs to finish successfully.
func (c *Client) WaitForTasks(ctx context.Context, taskUIDs []int64, timeout time.Duration) error {
	if len(taskUIDs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Since Meilisearch executes tasks sequentially in queue order, waiting for the
	// latest task guarantees earlier tasks have settled. We still verify earlier tasks.
	for _, uid := range taskUIDs {
		if _, err := c.WaitForTask(ctx, uid, timeout); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) applyHeaders(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("User-Agent", "Filament/1.0 (Meilisearch Sink)")
}

// FormatDocumentID formats an Arrow cell value as a valid Meilisearch document ID string.
func FormatDocumentID(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}
