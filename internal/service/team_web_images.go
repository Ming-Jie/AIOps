package service

import (
	"encoding/base64"
	"os"
	"regexp"
	"strings"

	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/model"
	"github.com/fisk086/aiops/internal/skills"
)

var (
	teamMdImageRefRe     = regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	teamIMFileMarkerRe   = regexp.MustCompile(`\[\[(?:lark_file|dingtalk_file|im_file):([^\]]+)\]\]`)
)

func isAbsoluteImageRef(ref string) bool {
	lower := strings.ToLower(ref)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(ref, "/")
}

// expandTeamImagesForWeb expands team chat attachments for web UI (data:image inline).
// seenFilenames ensures each attachment is rendered at most once per conversation load.
// DB / IM keep [[im_file:...]] markers and filename-only markdown.
func expandTeamImagesForWeb(content string, convID int64, agentIDs []int64, seenFilenames map[string]struct{}) string {
	if strings.TrimSpace(content) == "" || convID < 1 || len(agentIDs) == 0 {
		return content
	}
	content = expandTeamIMFileMarkersForWeb(content, convID, agentIDs, seenFilenames)
	content = expandTeamMarkdownImageRefsForWeb(content, convID, agentIDs, seenFilenames)
	return strings.TrimSpace(content)
}

// expandTeamIMFileMarkersForWeb turns [[im_file:screenshot.png]] into markdown images for web.
func expandTeamIMFileMarkersForWeb(content string, convID int64, agentIDs []int64, seenFilenames map[string]struct{}) string {
	sessionID := imoutbound.TeamConvSessionID(convID)
	return teamIMFileMarkerRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := teamIMFileMarkerRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return ""
		}
		name, err := imoutbound.SanitizeFileName(strings.TrimSpace(sub[1]))
		if err != nil {
			return ""
		}
		if teamAttachmentAlreadyShown(name, seenFilenames) {
			return ""
		}
		dataURI, ok := teamImageDataURI(name, sessionID, agentIDs)
		if !ok {
			return ""
		}
		teamMarkAttachmentShown(name, seenFilenames)
		return "\n\n![截图](" + dataURI + ")\n"
	})
}

// expandTeamMarkdownImageRefsForWeb replaces bare screenshot filenames in markdown image refs
// with data:image URIs for web clients.
func expandTeamMarkdownImageRefsForWeb(content string, convID int64, agentIDs []int64, seenFilenames map[string]struct{}) string {
	if strings.TrimSpace(content) == "" || convID < 1 || len(agentIDs) == 0 {
		return content
	}
	sessionID := imoutbound.TeamConvSessionID(convID)
	return teamMdImageRefRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := teamMdImageRefRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		ref := strings.TrimSpace(sub[1])
		if isAbsoluteImageRef(ref) {
			if strings.HasPrefix(strings.ToLower(ref), "data:image/") {
				return match
			}
			name, err := imoutbound.SanitizeFileName(ref)
			if err != nil {
				return match
			}
			if teamAttachmentAlreadyShown(name, seenFilenames) {
				return ""
			}
			teamMarkAttachmentShown(name, seenFilenames)
			return match
		}
		name, err := imoutbound.SanitizeFileName(ref)
		if err != nil {
			return match
		}
		if teamAttachmentAlreadyShown(name, seenFilenames) {
			return ""
		}
		dataURI, ok := teamImageDataURI(name, sessionID, agentIDs)
		if !ok {
			return match
		}
		teamMarkAttachmentShown(name, seenFilenames)
		return strings.Replace(match, "("+ref+")", "("+dataURI+")", 1)
	})
}

func teamAttachmentAlreadyShown(name string, seen map[string]struct{}) bool {
	if seen == nil {
		return false
	}
	_, ok := seen[name]
	return ok
}

func teamMarkAttachmentShown(name string, seen map[string]struct{}) {
	if seen == nil {
		return
	}
	seen[name] = struct{}{}
}

func teamImageDataURI(name, sessionID string, agentIDs []int64) (string, bool) {
	data, ok := loadTeamOutboundOrScreenshot(name, sessionID, agentIDs)
	if !ok {
		return "", false
	}
	mime := imageMimeFromFilename(name)
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), true
}

func imageMimeFromFilename(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	default:
		return "image/png"
	}
}

func loadTeamOutboundOrScreenshot(name, sessionID string, agentIDs []int64) ([]byte, bool) {
	if b64, ok := skills.PeekScreenshotData(name); ok && b64 != "" {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err == nil && len(data) > 0 {
			return data, true
		}
	}
	store := imoutbound.GlobalStore()
	for _, aid := range agentIDs {
		if aid < 1 {
			continue
		}
		path, err := store.ResolveFile(imoutbound.Scope{AgentID: aid, SessionID: sessionID}, name)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			return data, true
		}
	}
	return nil, false
}

func teamMemberAgentIDs(members []*model.TeamMember) []int64 {
	ids := make([]int64, 0, len(members))
	for _, m := range members {
		if m != nil && m.AgentID > 0 {
			ids = append(ids, m.AgentID)
		}
	}
	return ids
}
