package skills

import (
	"fmt"

	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
)

// ResolveIMAttachmentPath resolves an IM attachment from outbound storage, falling back to
// recent terminal temp files when the agent referenced a generated file without outbound write.
func ResolveIMAttachmentPath(store *imoutbound.Store, scope imoutbound.Scope, filename string) (string, error) {
	if store == nil {
		store = imoutbound.GlobalStore()
	}
	if path, err := store.ResolveFile(scope, filename); err == nil {
		logger.Info("im attachment resolved from outbound scope", "file", filename, "path", path, "session_id", scope.SessionID)
		return path, nil
	}
	if scope.AgentID > 0 {
		if path, err := store.FindRecentFileForAgent(scope.AgentID, filename); err == nil {
			logger.Warn("im attachment resolved from agent-wide outbound (not current session)",
				"file", filename, "path", path, "session_id", scope.SessionID, "agent_id", scope.AgentID)
			return path, nil
		}
	}
	if path, err := FindRecentTerminalFilePath(filename); err == nil {
		logger.Warn("im attachment resolved from terminal temp (not outbound store)",
			"file", filename, "path", path, "session_id", scope.SessionID)
		return path, nil
	}
	logger.Warn("im attachment not found",
		"file", filename,
		"agent_id", scope.AgentID,
		"session_id", scope.SessionID,
		"outbound_base", store.Base(),
	)
	return "", fmt.Errorf("attachment not found: %s", filename)
}
