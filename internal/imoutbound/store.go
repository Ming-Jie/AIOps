// Package imoutbound stages files for IM bots (e.g. Lark) to send after agent replies.
package imoutbound

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	FileMarkerPrefix = "[[lark_file:"
	FileMarkerSuffix = "]]"
)

type scopeKey struct{}

// Scope identifies one IM conversation turn's outbound file directory.
type Scope struct {
	AgentID   int64
	SessionID string
}

func WithScope(ctx context.Context, agentID int64, sessionID string) context.Context {
	sessionID = strings.TrimSpace(sessionID)
	if agentID < 1 || sessionID == "" {
		return ctx
	}
	return context.WithValue(ctx, scopeKey{}, Scope{AgentID: agentID, SessionID: sessionID})
}

func ScopeFromContext(ctx context.Context) (Scope, bool) {
	if ctx == nil {
		return Scope{}, false
	}
	v := ctx.Value(scopeKey{})
	if v == nil {
		return Scope{}, false
	}
	s, ok := v.(Scope)
	if !ok || s.AgentID < 1 || strings.TrimSpace(s.SessionID) == "" {
		return Scope{}, false
	}
	return s, true
}

// Store writes outbound files under {base}/lark-outbound/{agentID}/{sessionID}/.
type Store struct {
	mu   sync.RWMutex
	base string
}

var globalStore = &Store{}

func GlobalStore() *Store {
	return globalStore
}

func (s *Store) SetBase(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.base = strings.TrimSpace(dir)
}

func (s *Store) Base() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.base
}

func (s *Store) DirForScope(scope Scope) (string, error) {
	base := strings.TrimSpace(s.Base())
	if base == "" {
		return "", fmt.Errorf("outbound store not configured")
	}
	safeSession := sanitizePathSegment(scope.SessionID)
	if safeSession == "" {
		return "", fmt.Errorf("invalid session id")
	}
	dir := filepath.Join(base, "lark-outbound", fmt.Sprintf("%d", scope.AgentID), safeSession)
	dir = filepath.Clean(dir)
	root := filepath.Clean(filepath.Join(base, "lark-outbound"))
	if !strings.HasPrefix(dir, root+string(os.PathSeparator)) && dir != root {
		return "", fmt.Errorf("invalid outbound path")
	}
	return dir, nil
}

func sanitizePathSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, string(os.PathSeparator), "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "..", "_")
	return s
}

func SanitizeFileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("filename required")
	}
	name = filepath.Base(name)
	if name == "." || name == ".." || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid filename")
	}
	if len(name) > 200 {
		return "", fmt.Errorf("filename too long")
	}
	return name, nil
}

// WriteFile saves content for the scoped IM session and returns the attachment marker.
func (s *Store) WriteFile(scope Scope, filename, content string) (string, error) {
	name, err := SanitizeFileName(filename)
	if err != nil {
		return "", err
	}
	if len(content) > maxOutboundFileBytes {
		return "", fmt.Errorf("file too large (max %d bytes)", maxOutboundFileBytes)
	}
	dir, err := s.DirForScope(scope)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("mkdir outbound: %w", err)
	}
	full := filepath.Join(dir, name)
	full = filepath.Clean(full)
	if !strings.HasPrefix(full, dir+string(os.PathSeparator)) && full != dir {
		return "", fmt.Errorf("invalid file path")
	}
	if err := os.WriteFile(full, []byte(content), 0o640); err != nil {
		return "", fmt.Errorf("write outbound file: %w", err)
	}
	return FileMarkerPrefix + name + FileMarkerSuffix, nil
}

// ResolveFile returns the absolute path for a marker filename within scope.
func (s *Store) ResolveFile(scope Scope, filename string) (string, error) {
	name, err := SanitizeFileName(filename)
	if err != nil {
		return "", err
	}
	dir, err := s.DirForScope(scope)
	if err != nil {
		return "", err
	}
	full := filepath.Join(dir, name)
	full = filepath.Clean(full)
	if !strings.HasPrefix(full, dir+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid file path")
	}
	if _, err := os.Stat(full); err != nil {
		return "", fmt.Errorf("outbound file not found: %w", err)
	}
	return full, nil
}

const maxOutboundFileBytes = 20 << 20 // 20 MiB
