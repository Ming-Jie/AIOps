package imoutbound

import (
	"context"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/fisk086/aiops/internal/logger"
)

var (
	imAnyHTTPURLRe        = regexp.MustCompile(`https?://[^\s\)\]>）\]]+`)
	imInternalFileURLRe   = regexp.MustCompile(`(?i)https?://[^\s\)\]>]+/api/v1/chat/(?:files|terminal-files)/[^\s\)\]>]*`)
	imRelativeFileURLRe   = regexp.MustCompile(`/api/v1/chat/(?:files|terminal-files)/[^\s\)\]>]+`)
	imMarkdownFileLinkRe  = regexp.MustCompile(`\[[^\]]*\]\((?:https?://[^\)]*/api/v1/chat/[^\)]*|/api/v1/chat/[^\)]+)\)`)
	imChatGeneratedPathRe = regexp.MustCompile(`(?i)(?:/api/v1/chat/)?(?:files/)?chat-generated/[^\s\)\]>）]+`)
	imTerminalFilesPathRe = regexp.MustCompile(`(?i)/api/v1/chat/terminal-files/[^\s\)\]>）]+`)
	imDownloadHintLineRe  = regexp.MustCompile(`(?im)^[^\n]*(下载链接|点击下载|download link|download url|请访问|访问链接)[^\n]*$`)
	imGeneratedFilesRe    = regexp.MustCompile(`(?im)^[^\n]*📎 Generated files:\s*[^\n]*$`)
	imRandomFileLineRe    = regexp.MustCompile(`(?im)^Random file generated:\s*\S+\s*$`)
	imFileContentIntroRe  = regexp.MustCompile(`(?im)^[^\n]*文件内容如下[：:][^\n]*$`)
	imFileContentIntroEnRe = regexp.MustCompile(`(?im)^[^\n]*file content (is|as follows)[：:][^\n]*$`)
	imSavedToFileRe       = regexp.MustCompile(`(?i)(?:已保存至|保存至|保存到|写入)\s*['"` + "`" + `]?([A-Za-z0-9][A-Za-z0-9._-]{0,120}\.(?:txt|csv|json|md|log))`)
	imDataLineRe          = regexp.MustCompile(`^[\d,\s\.\-]+$`)
	imClosingProseRe      = regexp.MustCompile(`^(如果需要|如需|若需要|要是你需要)`)
)

func imStandaloneNameLineRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(name) + `\s*$`)
}

// StripIMExternalURLs removes URLs and web-only file paths from IM-visible text.
func StripIMExternalURLs(s string) string {
	s = imMarkdownFileLinkRe.ReplaceAllString(s, "")
	s = imInternalFileURLRe.ReplaceAllString(s, "")
	s = imRelativeFileURLRe.ReplaceAllString(s, "")
	s = imChatGeneratedPathRe.ReplaceAllString(s, "")
	s = imTerminalFilesPathRe.ReplaceAllString(s, "")
	s = imAnyHTTPURLRe.ReplaceAllString(s, "")
	s = imDownloadHintLineRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// SanitizeNotifyText cleans scheduled/notify text for IM channels (text-only, no attachments).
func SanitizeNotifyText(s string) string {
	s = StripFileMarkers(s)
	s = StripDataURIs(s)
	s = StripIMExternalURLs(s)
	s = imGeneratedFilesRe.ReplaceAllString(s, "")
	s = imRandomFileLineRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "File saved for IM delivery.", "")
	s = strings.ReplaceAll(s, "Include this marker in your final answer:", "")
	return strings.TrimSpace(s)
}

// ContainsPastedFileContent reports whether the assistant pasted file body into IM text.
func ContainsPastedFileContent(text string) bool {
	_, _, ok := ExtractPastedIMFile(text)
	return ok
}

// ExtractPastedIMFile pulls filename + body when the model pasted file content in the reply.
func ExtractPastedIMFile(text string) (filename, content string, ok bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", false
	}

	name := ""
	if m := imSavedToFileRe.FindStringSubmatch(text); len(m) >= 2 {
		name, _ = SanitizeFileName(strings.Trim(m[1], "`'\"，。."))
	}
	if name == "" {
		for _, line := range strings.Split(text, "\n") {
			candidate := strings.TrimSpace(strings.Trim(line, "`'\" "))
			if n, err := SanitizeFileName(candidate); err == nil && strings.Contains(candidate, ".") {
				name = n
				break
			}
		}
	}
	if name == "" {
		return "", "", false
	}

	lines := strings.Split(text, "\n")
	bodyStart := -1
	for i, line := range lines {
		trim := strings.TrimSpace(strings.Trim(line, "`'\" "))
		if trim == name {
			bodyStart = i + 1
			break
		}
		if strings.Contains(trim, name) && (imSavedToFileRe.MatchString(trim) || strings.Contains(trim, "保存")) {
			bodyStart = i + 1
			break
		}
	}
	if bodyStart < 0 {
		return "", "", false
	}
	for bodyStart < len(lines) && strings.TrimSpace(lines[bodyStart]) == "" {
		bodyStart++
	}
	if bodyStart >= len(lines) {
		return "", "", false
	}

	var bodyLines []string
	for i := bodyStart; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if trim == "" {
			if len(bodyLines) > 0 {
				break
			}
			continue
		}
		if imClosingProseRe.MatchString(trim) {
			break
		}
		if looksLikeIMDataLine(trim) {
			bodyLines = append(bodyLines, strings.TrimRight(lines[i], " \t"))
			continue
		}
		if len(bodyLines) > 0 {
			break
		}
	}

	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	if len(body) < 8 {
		return "", "", false
	}
	return name, body, true
}

func looksLikeIMDataLine(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if imDataLineRe.MatchString(s) {
		return true
	}
	digitComma := 0
	for _, r := range s {
		if unicode.IsDigit(r) || r == ',' || r == ' ' || r == '\t' {
			digitComma++
		}
	}
	return digitComma >= len(s)*3/4
}

// StripPastedFileBody removes inline file payloads from IM-visible assistant text.
func StripPastedFileBody(text string) string {
	if name, body, ok := ExtractPastedIMFile(text); ok {
		text = strings.ReplaceAll(text, body, "")
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				text = strings.ReplaceAll(text, line, "")
			}
		}
		text = imStandaloneNameLineRe(name).ReplaceAllString(text, "")
	}
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			if len(kept) > 0 && kept[len(kept)-1] != "" {
				kept = append(kept, "")
			}
			continue
		}
		if looksLikeIMDataLine(trim) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// SalvagePastedIMFile writes pasted reply content into outbound storage for separate IM upload.
func SalvagePastedIMFile(ctx context.Context, store *Store, scope Scope, replyText string) (string, bool) {
	if store == nil {
		store = GlobalStore()
	}
	name, body, ok := ExtractPastedIMFile(replyText)
	if !ok {
		return "", false
	}
	if _, err := store.WriteFile(scope, name, body); err != nil {
		logger.Warn("imoutbound: salvage pasted file write failed", "file", name, "err", err)
		return "", false
	}
	RegisterWrittenFile(ctx, name)
	logger.Warn("imoutbound: salvaged LLM-pasted file content into outbound (model should use builtin_im_save_file)",
		"file", name, "bytes", len(body), "session_id", scope.SessionID)
	return name, true
}

// ShortIMAttachmentReply is the user-visible text when a file is sent as a separate IM message.
func ShortIMAttachmentReply(filename string) string {
	safe, err := SanitizeFileName(filename)
	if err != nil {
		return "文件已生成，请查收附件。"
	}
	return "已生成 " + safe + "，请查收附件。"
}

// SanitizeIMReplyText strips markers, inline file bodies, and duplicate filename lines from IM text replies.
func SanitizeIMReplyText(text string, scope Scope, store *Store, filenames []string) string {
	text = StripFileMarkers(text)
	text = StripDataURIs(text)
	text = strings.ReplaceAll(text, "File saved for IM delivery.", "")
	text = strings.ReplaceAll(text, "Include this marker in your final answer:", "")
	text = StripIMInlinePayload(text)

	for _, name := range filenames {
		safe, err := SanitizeFileName(name)
		if err != nil {
			continue
		}
		if store != nil {
			if path, err := store.ResolveFile(scope, safe); err == nil {
				if data, err := os.ReadFile(path); err == nil {
					body := strings.TrimSpace(string(data))
					if body != "" {
						text = strings.ReplaceAll(text, body, "")
					}
				}
			}
		}
		text = imStandaloneNameLineRe(safe).ReplaceAllString(text, "")
	}

	text = StripIMExternalURLs(text)
	text = imGeneratedFilesRe.ReplaceAllString(text, "")
	text = imRandomFileLineRe.ReplaceAllString(text, "")
	text = imFileContentIntroRe.ReplaceAllString(text, "")
	text = imFileContentIntroEnRe.ReplaceAllString(text, "")
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}
