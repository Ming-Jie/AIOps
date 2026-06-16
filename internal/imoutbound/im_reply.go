package imoutbound

import (
	"context"

	"github.com/fisk086/aiops/internal/logger"
)

// RegisteredFilesFromContext returns filenames staged by IM tools during the current turn.
func RegisteredFilesFromContext(ctx context.Context) []string {
	v := ctx.Value(writtenFilesKey{})
	if v == nil {
		return nil
	}
	t := v.(*writtenFilesTracker)
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]string, 0, len(t.names))
	for name := range t.names {
		out = append(out, name)
	}
	return out
}

// AttachmentNamesForSend returns files to upload: all tool-staged files plus markers that match staged names.
func AttachmentNamesForSend(ctx context.Context, markerNames []string) []string {
	staged := RegisteredFilesFromContext(ctx)
	stagedSet := make(map[string]struct{}, len(staged))
	for _, n := range staged {
		stagedSet[n] = struct{}{}
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(name string) {
		safe, err := SanitizeFileName(name)
		if err != nil {
			return
		}
		if _, ok := seen[safe]; ok {
			return
		}
		seen[safe] = struct{}{}
		out = append(out, safe)
	}
	for _, n := range staged {
		add(n)
	}
	for _, n := range markerNames {
		if _, ok := stagedSet[n]; ok {
			add(n)
		}
	}
	logger.Info("imoutbound: attachment names for send",
		"staged", staged,
		"markers", markerNames,
		"to_send", out,
		"has_tracker", ctx.Value(writtenFilesKey{}) != nil,
	)
	return out
}

// LogAttachmentPipeline summarizes why IM may or may not send files (call after agent reply).
func LogAttachmentPipeline(channel string, scope Scope, markerNames, fileNames []string, ctx context.Context) {
	_, hasScope := ScopeFromContext(ctx)
	staged := RegisteredFilesFromContext(ctx)
	reason := "ok"
	switch {
	case scope.SessionID == "":
		reason = "empty_session_id: WithScope skipped, tools cannot stage IM files"
	case !hasScope:
		reason = "im_scope_missing_on_context: RegisterWrittenFile will no-op"
	case len(staged) == 0 && len(markerNames) == 0:
		reason = "no_tool_staged_files: agent likely did not call builtin_im_save_file or builtin_terminal"
	case len(staged) == 0 && len(markerNames) > 0:
		reason = "markers_without_staged_files: LLM echoed markers but tool did not register file"
	case len(fileNames) == 0:
		reason = "no_files_to_send_after_merge"
	}
	logger.Info("imoutbound: attachment pipeline",
		"channel", channel,
		"agent_id", scope.AgentID,
		"session_id", scope.SessionID,
		"has_scope", hasScope,
		"has_tracker", ctx.Value(writtenFilesKey{}) != nil,
		"staged", staged,
		"markers", markerNames,
		"to_send", fileNames,
		"reason", reason,
	)
}
