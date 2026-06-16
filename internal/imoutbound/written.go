package imoutbound

import (
	"context"
	"sync"

	"github.com/fisk086/aiops/internal/logger"
)

type writtenFilesKey struct{}

type writtenFilesTracker struct {
	mu    sync.Mutex
	names map[string]struct{}
}

func withWrittenFilesTracker(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Value(writtenFilesKey{}) != nil {
		return ctx
	}
	return context.WithValue(ctx, writtenFilesKey{}, &writtenFilesTracker{names: make(map[string]struct{})})
}

// RegisterWrittenFile records a filename staged for the current IM turn.
func RegisterWrittenFile(ctx context.Context, filename string) {
	name, err := SanitizeFileName(filename)
	if err != nil {
		logger.Warn("imoutbound: RegisterWrittenFile invalid filename", "file", filename, "err", err)
		return
	}
	v := ctx.Value(writtenFilesKey{})
	if v == nil {
		_, hasScope := ScopeFromContext(ctx)
		logger.Warn("imoutbound: RegisterWrittenFile skipped — no file tracker on context",
			"file", name,
			"has_im_scope", hasScope,
			"hint", "call imoutbound.WithScope before agent invoke (requires non-empty session_id)",
		)
		return
	}
	t := v.(*writtenFilesTracker)
	t.mu.Lock()
	t.names[name] = struct{}{}
	t.mu.Unlock()
	logger.Info("imoutbound: file staged for IM send", "file", name)
}

// PartitionRegisteredFiles splits parsed markers into tool-staged files vs unregistered names.
// When no tracker is present, all names are treated as registered.
func PartitionRegisteredFiles(ctx context.Context, names []string) (registered, unregistered []string) {
	v := ctx.Value(writtenFilesKey{})
	if v == nil {
		return names, nil
	}
	t := v.(*writtenFilesTracker)
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, name := range names {
		if _, ok := t.names[name]; ok {
			registered = append(registered, name)
		} else {
			unregistered = append(unregistered, name)
		}
	}
	return registered, unregistered
}
