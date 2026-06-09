// Package kbimport fetches remote documents over HTTPS for knowledge-base ingestion.
package kbimport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

const (
	MaxBytes       = 50 * 1024 * 1024 // align with KB upload limit
	MaxRedirects   = 5
	RequestTimeout = 60 * time.Second
)

var (
	ErrInvalidURL      = errors.New("invalid URL")
	ErrURLNotAllowed   = errors.New("URL not allowed")
	ErrFileTooLarge    = errors.New("file too large")
	ErrUnsupportedType = errors.New("unsupported file type")
	ErrEmptyBody       = errors.New("empty response body")
)

// AllowedExtensions matches knowledge-base upload types.
var AllowedExtensions = map[string]bool{
	".pdf": true, ".md": true, ".markdown": true, ".txt": true, ".text": true,
	".html": true, ".htm": true, ".docx": true, ".epub": true,
	".xlsx": true, ".xls": true, ".pptx": true, ".csv": true, ".json": true,
}

var safeNameRe = regexp.MustCompile(`[^\w\s\-.]`)

// Result holds a fetched remote document.
type Result struct {
	Body        []byte
	Filename    string
	ContentType string
	SourceURL   string
}

// Fetch downloads a HTTPS resource with SSRF guards and size limits.
func Fetch(ctx context.Context, rawURL string, preferredName string) (*Result, error) {
	parsed, err := ValidateURL(rawURL)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: RequestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= MaxRedirects {
				return fmt.Errorf("too many redirects")
			}
			if err := validateRequestURL(req.URL); err != nil {
				return err
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, ErrInvalidURL
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "AIOps-KB-Importer/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch failed: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(body) == 0 {
		return nil, ErrEmptyBody
	}
	if len(body) > MaxBytes {
		return nil, ErrFileTooLarge
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if i := strings.Index(contentType, ";"); i >= 0 {
		contentType = strings.TrimSpace(contentType[:i])
	}

	filename := InferFilename(parsed, preferredName, contentType, resp.Header.Get("Content-Disposition"))
	ext := strings.ToLower(path.Ext(filename))
	if !AllowedExtensions[ext] {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedType, ext)
	}

	return &Result{
		Body:        body,
		Filename:    filename,
		ContentType: contentType,
		SourceURL:   parsed.String(),
	}, nil
}

// ValidateURL parses and validates an import URL (HTTPS only, no credentials).
func ValidateURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrInvalidURL
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, ErrInvalidURL
	}
	return parsed, validateRequestURL(parsed)
}

func validateRequestURL(parsed *url.URL) error {
	if parsed == nil {
		return ErrInvalidURL
	}
	if parsed.Scheme != "https" {
		return ErrURLNotAllowed
	}
	if parsed.User != nil {
		return ErrURLNotAllowed
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return ErrInvalidURL
	}
	if err := assertHostAllowed(host); err != nil {
		return err
	}
	return nil
}

func assertHostAllowed(host string) error {
	if strings.EqualFold(host, "localhost") {
		return ErrURLNotAllowed
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return ErrURLNotAllowed
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolve host: %w", err)
	}
	if len(ips) == 0 {
		return ErrURLNotAllowed
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return ErrURLNotAllowed
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

// InferFilename picks a safe display filename for the KB document row.
func InferFilename(parsed *url.URL, preferredName, contentType, contentDisposition string) string {
	if name := sanitizeFilename(preferredName); name != "" {
		return ensureExtension(name, contentType, parsed)
	}
	if name := parseContentDispositionFilename(contentDisposition); name != "" {
		return ensureExtension(sanitizeFilename(name), contentType, parsed)
	}
	if parsed != nil {
		base := path.Base(parsed.Path)
		base = strings.Split(base, "?")[0]
		if name := sanitizeFilename(base); name != "" && name != "." && name != "/" {
			return ensureExtension(name, contentType, parsed)
		}
	}
	if ext := extFromContentType(contentType); ext != "" {
		return "import" + ext
	}
	return "import.bin"
}

func ensureExtension(name, contentType string, parsed *url.URL) string {
	ext := strings.ToLower(path.Ext(name))
	if AllowedExtensions[ext] {
		return name
	}
	if alt := extFromContentType(contentType); alt != "" {
		return strings.TrimSuffix(name, ext) + alt
	}
	if parsed != nil {
		if urlExt := strings.ToLower(path.Ext(parsed.Path)); AllowedExtensions[urlExt] {
			return name + urlExt
		}
	}
	return name
}

func sanitizeFilename(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = path.Base(strings.ReplaceAll(raw, "\\", "/"))
	raw = safeNameRe.ReplaceAllString(raw, "_")
	raw = strings.Trim(raw, "._ ")
	if raw == "" || raw == "." {
		return ""
	}
	if len(raw) > 200 {
		ext := path.Ext(raw)
	 stem := strings.TrimSuffix(raw, ext)
		if len(stem) > 200-len(ext) {
			stem = stem[:200-len(ext)]
		}
		raw = stem + ext
	}
	return raw
}

func parseContentDispositionFilename(cd string) string {
	cd = strings.TrimSpace(cd)
	if cd == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(cd)
	if err != nil {
		return ""
	}
	if fn := strings.TrimSpace(params["filename"]); fn != "" {
		return fn
	}
	return ""
}

func extFromContentType(contentType string) string {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	switch contentType {
	case "application/pdf":
		return ".pdf"
	case "text/markdown":
		return ".md"
	case "text/plain":
		return ".txt"
	case "text/html":
		return ".html"
	case "application/json":
		return ".json"
	case "text/csv":
		return ".csv"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx"
	case "application/epub+zip":
		return ".epub"
	}
	if exts, _ := mime.ExtensionsByType(contentType); len(exts) > 0 {
		for _, e := range exts {
			if AllowedExtensions[e] {
				return e
			}
		}
	}
	return ""
}
