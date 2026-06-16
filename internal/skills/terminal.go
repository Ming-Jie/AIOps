package skills

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
)

const (
	toolTerminal       = "builtin_terminal"
	maxTerminalOutput  = 100 << 10
	defaultTermTimeout = 30
	maxTermTimeout     = 300
	maxAttachFileSize  = 1 << 20
	defaultBinaryMIME  = "application/octet-stream"
)

type dirFileSnap struct {
	mod  time.Time
	size int64
}

var attachableExts = map[string]string{
	".txt":   "text/plain",
	".py":    "text/x-python",
	".js":    "text/javascript",
	".ts":    "text/typescript",
	".go":    "text/x-go",
	".java":  "text/x-java",
	".c":     "text/x-c",
	".cpp":   "text/x-cpp",
	".h":     "text/x-c",
	".rs":    "text/x-rust",
	".rb":    "text/x-ruby",
	".php":   "text/x-php",
	".sh":    "text/x-shellscript",
	".bash":  "text/x-shellscript",
	".zsh":   "text/x-shellscript",
	".yaml":  "text/yaml",
	".yml":   "text/yaml",
	".json":  "application/json",
	".xml":   "text/xml",
	".csv":   "text/csv",
	".md":    "text/markdown",
	".html":  "text/html",
	".css":   "text/css",
	".sql":   "text/sql",
	".toml":  "text/toml",
	".ini":   "text/plain",
	".cfg":   "text/plain",
	".conf":  "text/plain",
	".log":   "text/plain",
	".out":   "text/plain",
	".env":   "text/plain",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".svg":   "image/svg+xml",
	".webp":  "image/webp",
	".ico":   "image/x-icon",
	".pdf":   "application/pdf",
	".bin":   "application/octet-stream",
	".dat":   "application/octet-stream",
	".data":  "application/octet-stream",
	".zip":   "application/zip",
	".tar":   "application/x-tar",
	".gz":    "application/gzip",
	".bz2":   "application/x-bzip2",
	".xz":    "application/x-xz",
	".wasm":  "application/wasm",
	".exe":   "application/octet-stream",
	".dll":   "application/octet-stream",
	".so":    "application/octet-stream",
	".dylib": "application/octet-stream",
}

func mimeForFilename(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if mime, ok := attachableExts[ext]; ok {
		return mime
	}
	if ext == "" {
		return defaultBinaryMIME
	}
	return defaultBinaryMIME
}

func execBuiltinTerminal(ctx context.Context, in map[string]any) (string, error) {
	command := strArg(in, "command", "cmd", "exec")
	if command == "" {
		return "", fmt.Errorf("missing command")
	}

	timeoutSec := defaultTermTimeout
	if v := strArg(in, "timeout", "timeout_seconds"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSec = n
			if timeoutSec > maxTermTimeout {
				timeoutSec = maxTermTimeout
			}
		}
	}

	workdir := strArg(in, "workdir", "directory", "cwd")
	if workdir == "" {
		base, _ := os.Getwd()
		workdir = filepath.Join(base, "temp")
		os.MkdirAll(workdir, 0755)
	}

	snapshot := snapshotDir(workdir)

	ctx2, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx2, "sh", "-c", command)
	cmd.Dir = workdir

	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return "", fmt.Errorf("command execution error: %w", err)
		}
	}

	result := string(output)
	if len(result) > maxTerminalOutput {
		result = result[:maxTerminalOutput] + "\n... (output truncated at 100 KB)"
	}

	result = strings.TrimSpace(result)

	scope, isIM := imoutbound.ScopeFromContext(ctx)
	attachments := detectNewFiles(ctx, workdir, snapshot)
	logger.Info("terminal: detected new files", "count", len(attachments), "workdir", workdir, "is_im", isIM)
	if isIM {
		logger.Info("terminal: IM session", "agent_id", scope.AgentID, "session_id", scope.SessionID)
	} else if len(attachments) > 0 {
		logger.Info("terminal: web session — files exposed as download URLs")
	}

	var attachInfo string
	names := make([]string, 0, len(attachments))

	for _, att := range attachments {
		logger.Info("terminal: registering attachment", "file", att.Filename, "size", att.Size, "mime", att.MimeType)
		names = append(names, att.Filename)

		if isIM {
			data, err := os.ReadFile(filepath.Join(workdir, att.Filename))
			if err != nil {
				logger.Warn("terminal: failed to read file for IM outbound", "file", att.Filename, "err", err)
				continue
			}
			marker, err := imoutbound.GlobalStore().WriteFileBytes(scope, att.Filename, data)
			if err != nil {
				logger.Warn("terminal: failed to write IM outbound file", "file", att.Filename, "session_id", scope.SessionID, "err", err)
			} else {
				imoutbound.RegisterWrittenFile(ctx, att.Filename)
				if attachInfo != "" {
					attachInfo += "\n"
				}
				attachInfo += fmt.Sprintf("已为 IM 登记附件: %s", att.Filename)
				logger.Info("terminal: IM outbound file written", "file", att.Filename, "session_id", scope.SessionID)
			}
			if _, saveErr := SaveTerminalFile(att.Filename, data); saveErr != nil {
				logger.Warn("terminal: failed to save terminal temp copy", "file", att.Filename, "err", saveErr)
			} else if marker == "" {
				imoutbound.RegisterWrittenFile(ctx, att.Filename)
				attachInfo += "已为 IM 登记附件: " + att.Filename
				logger.Warn("terminal: outbound write failed; registered file name only (send may fail)", "file", att.Filename)
			}
		} else {
			AddAttachment(ctx, att)
		}
	}

	if isIM {
		// attachInfo uses Chinese registration notes only (no URLs / markers).
	} else if len(attachments) > 0 {
		attachInfo = "\n\n📎 Generated files: " + strings.Join(names, ", ")
	}

	if exitCode != 0 {
		out := fmt.Sprintf("Exit code: %d", exitCode)
		if result != "" {
			out += "\n\n" + result
		}
		if attachInfo != "" {
			out += "\n\n" + attachInfo
		}
		return scrubIMToolResult(isIM, out), nil
	}

	if result != "" && attachInfo != "" {
		return scrubIMToolResult(isIM, result+"\n\n"+attachInfo), nil
	}
	if result != "" {
		return scrubIMToolResult(isIM, result), nil
	}
	if attachInfo != "" {
		return scrubIMToolResult(isIM, attachInfo), nil
	}
	return "", nil
}

func scrubIMToolResult(isIM bool, s string) string {
	if !isIM {
		return s
	}
	return imoutbound.StripIMExternalURLs(s)
}

func snapshotDir(dir string) map[string]dirFileSnap {
	snap := make(map[string]dirFileSnap)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return snap
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		snap[e.Name()] = dirFileSnap{mod: info.ModTime(), size: info.Size()}
	}
	return snap
}

func fileChanged(before dirFileSnap, existed bool, info os.FileInfo) bool {
	if !existed {
		return info.Size() > 0
	}
	if info.Size() <= 0 {
		return false
	}
	return info.ModTime().After(before.mod) || info.Size() != before.size
}

func detectNewFiles(ctx context.Context, dir string, before map[string]dirFileSnap) []*FileAttachment {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var attachments []*FileAttachment

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}

		beforeSnap, existed := before[e.Name()]
		if !fileChanged(beforeSnap, existed, info) {
			continue
		}

		att := buildFileAttachment(ctx, filepath.Join(dir, e.Name()), info)
		if att != nil {
			attachments = append(attachments, att)
		}
	}

	return attachments
}

func buildFileAttachment(ctx context.Context, path string, info os.FileInfo) *FileAttachment {
	if info.Size() <= 0 {
		return nil
	}
	if info.Size() > maxAttachFileSize {
		logger.Warn("terminal: file too large to attach", "file", info.Name(), "size", info.Size())
		return nil
	}

	mime := mimeForFilename(info.Name())

	// IM path writes to outbound store directly; web path needs a downloadable URL first.
	if _, isIM := imoutbound.ScopeFromContext(ctx); isIM {
		return &FileAttachment{
			Filename: info.Name(),
			MimeType: mime,
			Size:     info.Size(),
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		logger.Warn("terminal: failed to read generated file", "file", info.Name(), "err", err)
		return nil
	}

	url, err := saveWebAttachmentURL(info.Name(), data)
	if err != nil {
		logger.Warn("terminal: failed to save web attachment URL", "file", info.Name(), "err", err)
		return nil
	}

	return &FileAttachment{
		Filename: info.Name(),
		MimeType: mime,
		Size:     info.Size(),
		URL:      url,
	}
}

func saveWebAttachmentURL(filename string, data []byte) (string, error) {
	url, _, err := SaveWebGeneratedFile(filename, data)
	return url, err
}

func NewBuiltinTerminalTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolTerminal,
			Desc: "Execute shell commands on the server. In IM (Lark/DingTalk), new/updated files (max 1MB) are sent as separate file messages automatically — never give the user HTTP URLs. In web chat, files appear as download buttons. Requires user approval.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"command": {Type: einoschema.String, Desc: "Shell command to execute (e.g., 'ls -la /tmp')", Required: true},
				"timeout": {Type: einoschema.String, Desc: "Execution timeout in seconds (default 30, max 300)", Required: false},
				"workdir": {Type: einoschema.String, Desc: "Working directory for the command", Required: false},
			}),
		},
		execBuiltinTerminal,
	)
}
