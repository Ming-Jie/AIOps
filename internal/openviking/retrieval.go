package openviking

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

const (
	// MaxSnippetChars caps injected / API text per hit (raw chunk from content/read).
	MaxSnippetChars = 4000
	// SearchFetchMultiplier over-fetches candidates before top-k truncation.
	SearchFetchMultiplier = 3
)

// ReadContent returns raw text for a viking URI via GET /api/v1/content/read.
func (c *Client) ReadContent(ctx context.Context, uri string) (string, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return "", nil
	}
	q := url.Values{}
	q.Set("uri", uri)
	env, _, err := c.doJSON(ctx, http.MethodGet, "/api/v1/content/read?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	if len(env.Result) == 0 {
		return "", nil
	}
	var asString string
	if err := json.Unmarshal(env.Result, &asString); err == nil && asString != "" {
		return asString, nil
	}
	var obj struct {
		Content string `json:"content"`
		Text    string `json:"text"`
	}
	if err := json.Unmarshal(env.Result, &obj); err == nil {
		if s := strings.TrimSpace(obj.Content); s != "" {
			return s, nil
		}
		if s := strings.TrimSpace(obj.Text); s != "" {
			return s, nil
		}
	}
	return strings.TrimSpace(string(env.Result)), nil
}

// ResolveHitText prefers raw chunk text from content/read over OV-generated abstract.
func ResolveHitText(ctx context.Context, c *Client, hit MatchedContext) string {
	if c != nil && hit.URI != "" {
		if raw, err := c.ReadContent(ctx, hit.URI); err == nil {
			if s := strings.TrimSpace(raw); s != "" {
				return TruncateSnippet(s, MaxSnippetChars)
			}
		}
	}
	if s := strings.TrimSpace(hit.Abstract); s != "" {
		return TruncateSnippet(s, MaxSnippetChars)
	}
	if s := strings.TrimSpace(hit.Overview); s != "" {
		return TruncateSnippet(s, MaxSnippetChars)
	}
	return ""
}

// TruncateSnippet trims text for prompt injection / API responses.
func TruncateSnippet(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// DisplayNameFromURI extracts a human-readable name from a viking URI path segment.
func DisplayNameFromURI(uri string) string {
	if uri == "" {
		return ""
	}
	uri = strings.TrimRight(uri, "/")
	if i := strings.LastIndex(uri, "/"); i >= 0 {
		uri = uri[i+1:]
	}
	if j := strings.Index(uri, "_"); j > 0 && j < len(uri)-1 {
		// Skip numeric doc-id prefix: 5_filename.pdf
		prefix := uri[:j]
		allDigits := true
		for _, r := range prefix {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return uri[j+1:]
		}
	}
	return uri
}
