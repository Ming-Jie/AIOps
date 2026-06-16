package skills

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/aiops/internal/imoutbound"
)

const toolWebSaveFile = "builtin_web_save_file"

// MaxWebBubbleRunes caps assistant text shown in the web chat bubble (prevents multi-MB DOM blowups).
const MaxWebBubbleRunes = 12_000

// WebContentOmittedFallback is persisted when sanitization removed inline file/data payloads but attachments exist.
const WebContentOmittedFallback = "文件已生成，请使用下方下载按钮获取完整内容。"

var (
	webBase64LineRe       = regexp.MustCompile(`(?m)^[A-Za-z0-9+/]{40,}={0,2}\s*$`)
	webRandomFileLineRe   = regexp.MustCompile(`(?im)^Random file generated:\s*\S+\s*$`)
	webGeneratedFilesRe   = regexp.MustCompile(`(?im)^[^\n]*📎 Generated files:\s*[^\n]*$`)
	webFileContentIntroRe = regexp.MustCompile(`(?im)^[^\n]*文件内容如下[：:][^\n]*$`)
	webIMFileMarkerRe     = regexp.MustCompile(`\[\[(?:lark_file|dingtalk_file|im_file):[^\]]+\]\]`)
	webSavedFileHintRe    = regexp.MustCompile(`(?i)(?:已保存至|已生成|保存为|写入)\s*['"` + "`" + `]?([A-Za-z0-9][A-Za-z0-9._-]{0,120}\.(?:txt|csv|json|md|log))`)
)

// WebAgentInstructionSuffix is appended to the agent system instruction during web chat turns.
func WebAgentInstructionSuffix() string {
	return `

## Web 网页聊天
- 用户需要可下载文件时：调用 builtin_web_save_file（content 放文件正文）或 builtin_terminal 生成文件；界面会显示下载按钮。
- 最终回复**只能**写简短说明（如「已生成 xxx.txt，请点击下载」）；**禁止**粘贴文件全文、大数据、base64 或超长文本到回复里。`
}

// SanitizeWebAssistantText cleans web chat assistant text only (SSE persistence / ReAct web replies).
// Web UI renders images (data:image in reactSteps) and file downloads (/api/v1/chat/files/...) in the browser.
// IM bots use imoutbound.SanitizeIMReplyText and upload native file/image messages instead.
func SanitizeWebAssistantText(s string) string {
	original := strings.TrimSpace(s)
	if original == "" {
		return ""
	}
	s = original
	s = webIMFileMarkerRe.ReplaceAllString(s, "")
	s = webRandomFileLineRe.ReplaceAllString(s, "")
	s = webGeneratedFilesRe.ReplaceAllString(s, "")
	s = webFileContentIntroRe.ReplaceAllString(s, "")
	s = webBase64LineRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "File saved for IM delivery.", "")
	s = strings.ReplaceAll(s, "Include this marker in your final answer:", "")
	s = imoutbound.StripPastedFileBody(s)
	s = stripWebDeclaredFileBody(s)
	s = stripLargeFencedCodeBlocks(s)
	s = truncateWebBubbleRunes(s, MaxWebBubbleRunes)
	lines := strings.Split(s, "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		trimmed = append(trimmed, line)
	}
	result := strings.Join(trimmed, "\n")
	if strings.TrimSpace(result) != "" {
		return result
	}
	return webSanitizeFallback(original)
}

func webSanitizeFallback(original string) string {
	for _, line := range strings.Split(original, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isWebReplyTailLine(line) || webSavedFileHintRe.MatchString(line) {
			return truncateWebBubbleRunes(line, MaxWebBubbleRunes)
		}
	}
	return WebContentOmittedFallback
}

func stripLargeFencedCodeBlocks(s string) string {
	const minBodyRunes = 2000
	var out strings.Builder
	i := 0
	for i < len(s) {
		start := strings.Index(s[i:], "```")
		if start < 0 {
			out.WriteString(s[i:])
			break
		}
		start += i
		out.WriteString(s[i:start])

		lineEnd := strings.Index(s[start:], "\n")
		if lineEnd < 0 {
			out.WriteString(s[start:])
			break
		}
		bodyStart := start + lineEnd + 1
		closeRel := strings.Index(s[bodyStart:], "```")
		if closeRel < 0 {
			out.WriteString(s[start:])
			break
		}
		body := s[bodyStart : bodyStart+closeRel]
		blockEnd := bodyStart + closeRel + 3
		if utf8.RuneCountInString(body) > minBodyRunes {
			out.WriteString("\n…（代码块过长，请使用附件下载完整内容）\n")
		} else {
			out.WriteString(s[start:blockEnd])
		}
		i = blockEnd
	}
	return out.String()
}

func truncateWebBubbleRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "\n\n…（内容过长，请使用下方附件下载完整文件）"
}

// stripWebDeclaredFileBody removes inline file payloads after a save/generate hint (poetry, CSV, etc.).
func stripWebDeclaredFileBody(s string) string {
	m := webSavedFileHintRe.FindStringSubmatch(s)
	if len(m) < 2 {
		return s
	}
	name := m[1]
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	skipping := false
	for _, line := range lines {
		trim := strings.TrimSpace(strings.Trim(line, "`'\" "))
		if !skipping && (trim == name || strings.Contains(trim, name) && (strings.Contains(trim, "保存") || strings.Contains(trim, "生成") || strings.Contains(trim, "写入"))) {
			out = append(out, line)
			skipping = true
			continue
		}
		if skipping {
			if trim == "" {
				continue
			}
			if isWebReplyTailLine(trim) {
				skipping = false
				out = append(out, line)
			}
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func isWebReplyTailLine(s string) bool {
	for _, kw := range []string{"请", "下载", "查收", "附件", "按钮", "点击", "需要", "如果"} {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// NewWebSaveFileTool saves a text file for web chat download (not available in IM sessions).
func NewWebSaveFileTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolWebSaveFile,
			Desc: "Save a text file for the user to download in web chat. The UI shows a download button automatically. You MUST call this tool to create files — pasting file content in your reply will not work for large data. Never paste file content in the reply text.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"filename": {
					Type:     einoschema.String,
					Desc:     "File name with extension, e.g. chuntian.txt or report.csv",
					Required: true,
				},
				"content": {
					Type:     einoschema.String,
					Desc:     "Full file body (UTF-8 text, max 1MB)",
					Required: true,
				},
			}),
		},
		func(ctx context.Context, in map[string]any) (string, error) {
			if _, ok := imoutbound.ScopeFromContext(ctx); ok {
				return "", fmt.Errorf("%s is only available in web chat; use builtin_im_save_file in IM sessions", toolWebSaveFile)
			}
			filename := strArg(in, "filename", "file_name", "name")
			content := strArg(in, "content", "data", "body", "text")
			if filename == "" {
				return "", fmt.Errorf("missing filename")
			}
			if content == "" {
				return "", fmt.Errorf("missing content")
			}
			if len(content) > maxAttachFileSize {
				return "", fmt.Errorf("content too large (max %d bytes)", maxAttachFileSize)
			}
			url, err := saveWebAttachmentURL(filename, []byte(content))
			if err != nil {
				return "", err
			}
			AddAttachment(ctx, &FileAttachment{
				Filename: filename,
				MimeType: "text/plain; charset=utf-8",
				Size:     int64(len(content)),
				URL:      url,
			})
			return fmt.Sprintf("File %s saved for web download at %s. Reply with one short sentence only—do not paste file contents.", filename, url), nil
		},
	)
}
