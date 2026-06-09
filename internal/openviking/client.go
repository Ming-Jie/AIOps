// Package openviking is a thin HTTP client for the OpenViking knowledge-base
// service (vector store + retrieval).
//
// Authentication is by API key only — the key itself identifies the caller on
// the OpenViking side. Knowledge bases are isolated from each other by URI path
// (viking://resources/kb/{kb_id}/); per-user isolation is enforced in our own
// DB layer (kb_documents.owner_id), not via OpenViking tenant headers.
package openviking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/fisk086/aiops/internal/logger"
)

// Client wraps the OpenViking HTTP REST API.
type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewClient builds a client. Only the API key is needed for auth.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: trimSlash(baseURL),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 130 * time.Second},
	}
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// envelope is the unified OpenViking response wrapper: {status, result, error{code,message}}.
type envelope struct {
	Status string          `json:"status"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Time float64 `json:"time"`
}

// authHeaders returns the auth headers sent on every request. Both X-API-Key and
// Authorization: Bearer are set so either OpenViking-native or gateway-fronted
// deployments accept the key.
func (c *Client) authHeaders() map[string]string {
	return map[string]string{
		"Content-Type":  "application/json",
		"X-API-Key":     c.apiKey,
		"Authorization": "Bearer " + c.apiKey,
	}
}

// doJSON performs a JSON request and decodes the result envelope.
func (c *Client) doJSON(ctx context.Context, method, path string, body any, okCodes ...int) (*envelope, int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	for k, v := range c.authHeaders() {
		req.Header.Set(k, v)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("openviking request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var env envelope
	_ = json.Unmarshal(raw, &env) // best-effort; some endpoints (health) may not wrap

	if !statusOK(resp.StatusCode, okCodes) {
		msg := string(raw)
		if env.Error != nil {
			msg = env.Error.Code + ": " + env.Error.Message
		}
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return &env, resp.StatusCode, fmt.Errorf("openviking %s %s -> %d: %s", method, path, resp.StatusCode, msg)
	}
	return &env, resp.StatusCode, nil
}

func statusOK(code int, okCodes []int) bool {
	if len(okCodes) == 0 {
		return code >= 200 && code < 300
	}
	for _, c := range okCodes {
		if c == code {
			return true
		}
	}
	return code >= 200 && code < 300
}

// Mkdir creates a directory in OV AGFS (idempotent — already-exists is tolerated).
func (c *Client) Mkdir(ctx context.Context, uri string) error {
	_, _, err := c.doJSON(ctx, http.MethodPost, "/api/v1/fs/mkdir", map[string]string{"uri": uri}, 200, 201, 409)
	return err
}

// Rm deletes a file or directory (recursive). 404 is treated as success.
func (c *Client) Rm(ctx context.Context, uri string) error {
	q := url.Values{}
	q.Set("uri", uri)
	q.Set("recursive", "true")
	_, _, err := c.doJSON(ctx, http.MethodDelete, "/api/v1/fs?"+q.Encode(), nil, 200, 204, 404)
	return err
}

// FsEntry is a single directory entry returned by Ls.
type FsEntry struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
	URI   string `json:"uri"`
}

// Ls lists directory contents. Returns nil on a missing directory.
func (c *Client) Ls(ctx context.Context, uri string) ([]FsEntry, error) {
	q := url.Values{}
	q.Set("uri", uri)
	env, code, err := c.doJSON(ctx, http.MethodGet, "/api/v1/fs/ls?"+q.Encode(), nil, 200, 404)
	if err != nil {
		return nil, err
	}
	if code == 404 || len(env.Result) == 0 {
		return nil, nil
	}
	var entries []FsEntry
	if err := json.Unmarshal(env.Result, &entries); err != nil {
		return nil, fmt.Errorf("decode ls result: %w", err)
	}
	return entries, nil
}

// TempUpload uploads a local file and returns its temp_file_id for a later AddResource call.
func (c *Client) TempUpload(ctx context.Context, localPath string) (string, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open local file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", fmt.Errorf("copy file into form: %w", err)
	}
	if err := mw.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/resources/temp_upload", &buf)
	if err != nil {
		return "", fmt.Errorf("create temp_upload request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("temp_upload request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("temp_upload -> %d: %s", resp.StatusCode, truncate(string(raw), 500))
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", fmt.Errorf("decode temp_upload: %w", err)
	}
	var result struct {
		TempFileID string `json:"temp_file_id"`
	}
	if err := json.Unmarshal(env.Result, &result); err != nil {
		return "", fmt.Errorf("decode temp_upload result: %w", err)
	}
	if result.TempFileID == "" {
		return "", fmt.Errorf("temp_upload returned empty temp_file_id")
	}
	return result.TempFileID, nil
}

// AddResourceResult is the outcome of AddResource.
type AddResourceResult struct {
	RootURI string `json:"root_uri"`
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
}

// AddResource indexes a previously uploaded temp file at the target URI.
// wait=true blocks until semantic/vector processing finishes.
func (c *Client) AddResource(ctx context.Context, tempFileID, toURI, reason string, wait bool) (*AddResourceResult, error) {
	// Note: the HTTP API forbids unknown fields (extra=forbid). Indexing and
	// summarization happen automatically — do not send build_index/summarize.
	body := map[string]any{
		"temp_file_id": tempFileID,
		"to":           toURI,
		"reason":       reason,
		"wait":         wait,
	}
	env, _, err := c.doJSON(ctx, http.MethodPost, "/api/v1/resources", body)
	if err != nil {
		return nil, err
	}
	var result AddResourceResult
	if len(env.Result) > 0 {
		if err := json.Unmarshal(env.Result, &result); err != nil {
			return nil, fmt.Errorf("decode add_resource result: %w", err)
		}
	}
	return &result, nil
}

// MatchedContext is a single retrieval hit.
type MatchedContext struct {
	URI         string  `json:"uri"`
	Abstract    string  `json:"abstract"`
	Overview    string  `json:"overview"`
	Content     string  `json:"content,omitempty"` // hydrated raw text (content/read), for RAG + UI
	Score       float64 `json:"score"`
	ContextType string  `json:"context_type"`
	Level       int     `json:"level"`
}

type findResult struct {
	Memories  []MatchedContext `json:"memories"`
	Resources []MatchedContext `json:"resources"`
	Skills    []MatchedContext `json:"skills"`
	Total     int              `json:"total"`
}

// Find performs a plain vector similarity search scoped to targetURI (no intent
// analysis or query expansion). Prefer SmartSearch for natural-language questions.
func (c *Client) Find(ctx context.Context, query, targetURI string, limit int, scoreThreshold float64) ([]MatchedContext, error) {
	return c.searchEndpoint(ctx, "/api/v1/search/find", query, targetURI, limit, scoreThreshold)
}

// SmartSearch performs a context-aware search (intent analysis + query expansion
// + rerank). Higher quality than Find for natural-language questions; session is
// omitted, but intent analysis still runs.
func (c *Client) SmartSearch(ctx context.Context, query, targetURI string, limit int, scoreThreshold float64) ([]MatchedContext, error) {
	return c.searchEndpoint(ctx, "/api/v1/search/search", query, targetURI, limit, scoreThreshold)
}

func (c *Client) searchEndpoint(ctx context.Context, path, query, targetURI string, limit int, scoreThreshold float64) ([]MatchedContext, error) {
	if limit <= 0 {
		limit = 10
	}
	body := map[string]any{
		"query":      query,
		"target_uri": targetURI,
		"limit":      limit,
	}
	if scoreThreshold > 0 {
		body["score_threshold"] = scoreThreshold
	}
	env, _, err := c.doJSON(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	var res findResult
	if len(env.Result) > 0 {
		if err := json.Unmarshal(env.Result, &res); err != nil {
			return nil, fmt.Errorf("decode search result: %w", err)
		}
	}
	hits := make([]MatchedContext, 0, len(res.Resources)+len(res.Memories))
	hits = append(hits, res.Resources...)
	hits = append(hits, res.Memories...)
	return hits, nil
}

// TaskStatus describes a background task (e.g. async indexing).
type TaskStatus struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"` // queued | running | done | failed
}

// GetTask returns the status of a single background task.
func (c *Client) GetTask(ctx context.Context, taskID string) (*TaskStatus, error) {
	path := "/api/v1/tasks/" + url.PathEscape(taskID)
	env, _, err := c.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var t TaskStatus
	if len(env.Result) > 0 {
		if err := json.Unmarshal(env.Result, &t); err != nil {
			return nil, fmt.Errorf("decode task result: %w", err)
		}
	}
	return &t, nil
}

// Health reports whether the OpenViking server is reachable.
func (c *Client) Health(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := c.client.Do(req)
	if err != nil {
		logger.Warn("openviking health check failed", "err", err)
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// KBPath returns the AGFS directory URI for a knowledge base.
func KBPath(kbID int64) string {
	return "viking://resources/kb/" + strconv.FormatInt(kbID, 10) + "/"
}

// DocURI returns a UNIQUE AGFS target URI for a document inside a knowledge base.
// The document id is prefixed so re-uploading the same filename produces a
// distinct resource (no collision in OpenViking, which dedupes by URI). This
// keeps a strict 1:1 mapping between a DB row and an OpenViking resource, so
// deleting one document never affects another.
func DocURI(kbID, docID int64, filename string) string {
	return KBPath(kbID) + strconv.FormatInt(docID, 10) + "_" + filename
}
