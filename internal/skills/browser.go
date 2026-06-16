package skills

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
)

const toolBrowser = "builtin_browser"

type agentBrowserResponse struct {
	Success bool                   `json:"success"`
	Error   string                 `json:"error,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

var allowedBrowserOps = map[string]bool{
	"navigate":   true,
	"snapshot":   true,
	"click":      true,
	"type":       true,
	"scroll":     true,
	"back":       true,
	"press":      true,
	"screenshot": true,
	"eval":       true,
	"console":    true,
	"close":      true,
}

var (
	browserSessions  sync.Map
	browserMu        sync.Mutex
	screenshotStore  sync.Map // filename → base64 PNG data
)

// PopScreenshotData retrieves and removes screenshot base64 data by filename (e.g. "screenshot_xxx.png").
// Used by SSE handlers to embed the image in tool_result/observation without passing raw base64 to the LLM.
func PopScreenshotData(filename string) (string, bool) {
	filename = normalizeScreenshotFilename(filename)
	if filename == "" {
		return "", false
	}
	v, ok := screenshotStore.LoadAndDelete(filename)
	if !ok {
		return "", false
	}
	b64, ok := v.(string)
	return b64, ok
}

var screenshotSavedRe = regexp.MustCompile(`(?i)Screenshot saved as\s+(screenshot_\d+\.png)`)

func normalizeScreenshotFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimRight(name, ".")
	return name
}

// ExtractScreenshotFilename parses builtin_browser screenshot tool output.
func ExtractScreenshotFilename(content string) string {
	m := screenshotSavedRe.FindStringSubmatch(content)
	if len(m) < 2 {
		return ""
	}
	return normalizeScreenshotFilename(m[1])
}

// EnrichScreenshotToolResult prefixes web SSE payloads with data:image when a screenshot was saved.
// ReAct observation and ADK tool_result both call this before emitting to the client.
func EnrichScreenshotToolResult(content string) string {
	fname := ExtractScreenshotFilename(content)
	if fname == "" {
		return content
	}
	b64, ok := PopScreenshotData(fname)
	if !ok || b64 == "" {
		return content
	}
	return "data:image/png;base64," + b64 + "\n\n" + content
}

// PeekScreenshotData returns screenshot base64 without removing it (e.g. team chat web expand).
func PeekScreenshotData(filename string) (string, bool) {
	v, ok := screenshotStore.Load(filename)
	if !ok {
		return "", false
	}
	b64, ok := v.(string)
	return b64, ok
}

const (
	browserCommandTimeout = 60
	browserIdleTimeout    = 5 * time.Minute
	scrollPixels          = "500"
)

type browserSession struct {
	sessionName string
	lastUsed    time.Time
}

func init() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("browser session cleanup panicked", "recover", r)
			}
		}()
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			browserSessions.Range(func(key, value any) bool {
				s := value.(*browserSession)
				if time.Since(s.lastUsed) > browserIdleTimeout {
					cleanupSessionDir(s.sessionName)
					browserSessions.Delete(key)
				}
				return true
			})
		}
	}()
}

func cleanupSessionDir(sessionName string) {
	tmpDir := filepath.Join(os.TempDir(), "agent-browser-"+sessionName)
	os.RemoveAll(tmpDir)
}

func sessionSocketDir(sessionName string) string {
	return filepath.Join(os.TempDir(), "agent-browser-"+sessionName)
}

func findAgentBrowser() (string, error) {
	if p, err := exec.LookPath("agent-browser"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("npx"); err == nil {
		return p + " agent-browser", nil
	}
	return "", fmt.Errorf(
		"agent-browser CLI not found. Install it with:\n" +
			"  npx agent-browser install --with-deps\n" +
			"Or run 'npm install -g agent-browser' for a global install.",
	)
}

func createBrowserSession(taskID string) (*browserSession, error) {
	browserMu.Lock()
	defer browserMu.Unlock()

	if v, ok := browserSessions.Load(taskID); ok {
		s := v.(*browserSession)
		s.lastUsed = time.Now()
		return s, nil
	}

	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}
	sessionName := "h_" + hex.EncodeToString(b)

	s := &browserSession{
		sessionName: sessionName,
		lastUsed:    time.Now(),
	}
	browserSessions.Store(taskID, s)

	sockDir := sessionSocketDir(sessionName)
	os.MkdirAll(sockDir, 0700)
	return s, nil
}

func execAgentBrowser(cmdPath, sessionName, command string, args []string) (*agentBrowserResponse, error) {
	var cmd *exec.Cmd

	if strings.Contains(cmdPath, " ") {
		parts := strings.Fields(cmdPath)
		cmdArgs := append(parts[1:], "--session", sessionName, "--json", command)
		cmdArgs = append(cmdArgs, args...)
		cmd = exec.Command(parts[0], cmdArgs...)
	} else {
		cmdArgs := []string{"--session", sessionName, "--json", command}
		cmdArgs = append(cmdArgs, args...)
		cmd = exec.Command(cmdPath, cmdArgs...)
	}

	sockDir := sessionSocketDir(sessionName)
	cmd.Env = append(os.Environ(),
		"AGENT_BROWSER_SOCKET_DIR="+sockDir,
	)

	if os.Geteuid() == 0 {
		cmd.Env = append(cmd.Env, "AGENT_BROWSER_ARGS=--no-sandbox,--disable-dev-shm-usage")
	}

	output, err := cmd.CombinedOutput()

	// npx/npm may emit warnings (e.g. "npm warn exec ...") to stderr before the actual JSON.
	// Strip any non-JSON prefix by searching for the first '{'.
	jsonStart := byteIndex(output, '{')
	if jsonStart > 0 {
		output = output[jsonStart:]
	}

	if err != nil {
		var resp agentBrowserResponse
		if json.Unmarshal(output, &resp) == nil {
			return &resp, nil
		}
		return nil, fmt.Errorf("agent-browser %s failed: %s\n%s", command, err.Error(), string(output))
	}

	var resp agentBrowserResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("parse agent-browser response: %w\n%s", err, string(output))
	}

	return &resp, nil
}

func execBuiltinBrowser(ctx context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "navigate"
	}

	if !allowedBrowserOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedBrowserOps)
	}

	cmdPath, err := findAgentBrowser()
	if err != nil {
		return err.Error(), nil
	}

	taskID := strArg(in, "task_id", "task", "session")
	if taskID == "" {
		taskID = "default"
	}

	url := strArg(in, "url", "target", "link")
	ref := strArg(in, "ref", "selector", "element")
	text := strArg(in, "text", "input", "value")
	js := strArg(in, "js", "script", "code")
	direction := strArg(in, "direction", "dir")
	full := strArg(in, "full", "full_page")
	clear := strArg(in, "clear", "clear_console")

	switch op {
	case "close":
		if v, ok := browserSessions.Load(taskID); ok {
			s, ok := v.(*browserSession)
			if !ok {
				browserSessions.Delete(taskID)
				break
			}
			cleanupSessionDir(s.sessionName)
			browserSessions.Delete(taskID)
		}
		return "browser session closed", nil
	}

	s, err := createBrowserSession(taskID)
	if err != nil {
		return "", fmt.Errorf("create browser session: %w", err)
	}
	s.lastUsed = time.Now()

	switch op {
	case "navigate":
		if url == "" {
			return "", fmt.Errorf("url is required for navigate")
		}

		resp, err := execAgentBrowser(cmdPath, s.sessionName, "open", []string{url})
		if err != nil {
			return "", fmt.Errorf("navigate failed: %w", err)
		}
		if !resp.Success {
			return fmt.Sprintf("navigation failed: %s", resp.Error), nil
		}

		result := "navigated to "
		if data := resp.Data; data != nil {
			if u, ok := data["url"].(string); ok && u != "" {
				result += u
			} else {
				result += url
			}
			if title, ok := data["title"].(string); ok && title != "" {
				result += "\ntitle: " + title
			}
		} else {
			result += url
		}

		// Bot detection check on title
		if title, ok := resp.Data["title"].(string); ok {
			blocked := []string{
				"access denied", "blocked", "bot detected", "verification required",
				"are you a robot", "captcha", "cloudflare", "checking your browser",
				"just a moment", "attention required",
			}
			titleLower := strings.ToLower(title)
			for _, p := range blocked {
				if strings.Contains(titleLower, p) {
					result += "\nPage may be blocked by bot detection"
					break
				}
			}
		}

		return result, nil

	case "snapshot":
		var args []string
		if full != "true" && full != "1" && full != "yes" {
			args = append(args, "-c")
		}
		resp, err := execAgentBrowser(cmdPath, s.sessionName, "snapshot", args)
		if err != nil {
			return "", fmt.Errorf("snapshot failed: %w", err)
		}
		if !resp.Success {
			return fmt.Sprintf("snapshot failed: %s", resp.Error), nil
		}

		result := "page snapshot:\n\n"
		if data := resp.Data; data != nil {
			if snapText, ok := data["snapshot"].(string); ok && snapText != "" {
				if len(snapText) > 6000 {
					snapText = snapText[:6000] + "... (truncated)"
				}
				result += snapText
			}
			if refs, ok := data["refs"]; ok {
				if refMap, ok := refs.(map[string]interface{}); ok && len(refMap) > 0 {
					result += fmt.Sprintf("\nelement refs available: %d", len(refMap))
				}
			}
		}
		if result == "page snapshot:\n\n" {
			result += "(empty)"
		}
		return result, nil

	case "click":
		if ref == "" {
			return "", fmt.Errorf("ref is required for click (e.g., @e5)")
		}
		if !strings.HasPrefix(ref, "@") {
			ref = "@" + ref
		}
		resp, err := execAgentBrowser(cmdPath, s.sessionName, "click", []string{ref})
		if err != nil {
			return "", fmt.Errorf("click %s failed: %w", ref, err)
		}
		if !resp.Success {
			return fmt.Sprintf("click %s failed: %s", ref, resp.Error), nil
		}
		return fmt.Sprintf("clicked %s", ref), nil

	case "type":
		if ref == "" || text == "" {
			return "", fmt.Errorf("ref and text are required for type")
		}
		if !strings.HasPrefix(ref, "@") {
			ref = "@" + ref
		}
		resp, err := execAgentBrowser(cmdPath, s.sessionName, "fill", []string{ref, text})
		if err != nil {
			return "", fmt.Errorf("type into %s failed: %w", ref, err)
		}
		if !resp.Success {
			return fmt.Sprintf("type into %s failed: %s", ref, resp.Error), nil
		}
		return fmt.Sprintf("typed into %s", ref), nil

	case "scroll":
		if direction == "" {
			direction = "down"
		}
		if direction != "up" && direction != "down" {
			return "", fmt.Errorf("invalid direction %q; use up or down", direction)
		}
		resp, err := execAgentBrowser(cmdPath, s.sessionName, "scroll", []string{direction, scrollPixels})
		if err != nil {
			return "", fmt.Errorf("scroll %s failed: %w", direction, err)
		}
		if !resp.Success {
			return fmt.Sprintf("scroll %s failed: %s", direction, resp.Error), nil
		}
		return fmt.Sprintf("scrolled %s", direction), nil

	case "back":
		resp, err := execAgentBrowser(cmdPath, s.sessionName, "back", []string{})
		if err != nil {
			return "", fmt.Errorf("back failed: %w", err)
		}
		if !resp.Success {
			return fmt.Sprintf("back failed: %s", resp.Error), nil
		}
		result := "navigated back"
		if data := resp.Data; data != nil {
			if u, ok := data["url"].(string); ok && u != "" {
				result += " to " + u
			}
		}
		return result, nil

	case "press":
		if text == "" {
			return "", fmt.Errorf("key is required for press (e.g., Enter, Tab)")
		}
		resp, err := execAgentBrowser(cmdPath, s.sessionName, "press", []string{text})
		if err != nil {
			return "", fmt.Errorf("press %s failed: %w", text, err)
		}
		if !resp.Success {
			return fmt.Sprintf("press %s failed: %s", text, resp.Error), nil
		}
		return fmt.Sprintf("pressed %s", text), nil

	case "screenshot":
		screenshotPath := filepath.Join(sessionSocketDir(s.sessionName), "screenshot.png")

		var args []string
		if full == "true" || full == "1" || full == "yes" {
			args = append(args, "--full")
		}
		args = append(args, screenshotPath)

		resp, err := execAgentBrowser(cmdPath, s.sessionName, "screenshot", args)
		if err != nil {
			return "", fmt.Errorf("screenshot failed: %w", err)
		}
		if !resp.Success {
			return fmt.Sprintf("screenshot failed: %s", resp.Error), nil
		}

		data, err := os.ReadFile(screenshotPath)
		if err != nil {
			return "", fmt.Errorf("read screenshot: %w", err)
		}
		b64 := base64.StdEncoding.EncodeToString(data)

		ts := time.Now().UnixMilli()
		fname := fmt.Sprintf("screenshot_%d.png", ts)

		// Store in global map so the SSE handler can embed the image in the tool_result
		// event without passing raw base64 to the LLM.
		screenshotStore.Store(fname, b64)

		// IM: outbound store + marker. Web: SSE embeds data:image via EnrichScreenshotToolResult (no duplicate attachment row).
		var imgMarker string
		if scope, ok := imoutbound.ScopeFromContext(ctx); ok {
			if _, err := imoutbound.GlobalStore().WriteFileBytes(scope, fname, data); err != nil {
				logger.Warn("builtin_browser: save screenshot to outbound store", "err", err)
			} else {
				imoutbound.RegisterWrittenFile(ctx, fname)
				imgMarker = "\n\n[[im_file:" + fname + "]]"
			}
		}

		if imgMarker != "" {
			return "Screenshot saved as " + fname + imgMarker, nil
		}
		return "Screenshot saved as " + fname, nil

	case "eval":
		if js == "" {
			return "", fmt.Errorf("js is required for eval")
		}
		resp, err := execAgentBrowser(cmdPath, s.sessionName, "eval", []string{js})
		if err != nil {
			return "", fmt.Errorf("eval failed: %w", err)
		}
		if !resp.Success {
			return fmt.Sprintf("eval failed: %s", resp.Error), nil
		}

		result := "eval result:\n\n"
		if data := resp.Data; data != nil {
			if r, ok := data["result"]; ok {
				result += fmt.Sprintf("%v", r)
			}
		}
		if result == "eval result:\n\n" {
			result += "(no output)"
		}
		return result, nil

	case "console":
		clearArgs := []string{}
		if clear == "true" || clear == "1" || clear == "yes" {
			clearArgs = append(clearArgs, "--clear")
		}

		consoleResp, err := execAgentBrowser(cmdPath, s.sessionName, "console", clearArgs)
		if err != nil {
			logger.Warn("browser: console fetch failed", "err", err)
		}
		errorsResp, err := execAgentBrowser(cmdPath, s.sessionName, "errors", clearArgs)
		if err != nil {
			logger.Warn("browser: errors fetch failed", "err", err)
		}

		var b strings.Builder
		b.WriteString("console messages:\n")
		if consoleResp != nil && consoleResp.Success && consoleResp.Data != nil {
			if msgs, ok := consoleResp.Data["messages"].([]interface{}); ok {
				for _, m := range msgs {
					if msg, ok := m.(map[string]interface{}); ok {
						t, _ := msg["type"].(string)
						txt, _ := msg["text"].(string)
						b.WriteString(fmt.Sprintf("  [%s] %s\n", t, txt))
					}
				}
			}
		}
		b.WriteString("js errors:\n")
		if errorsResp != nil && errorsResp.Success && errorsResp.Data != nil {
			if errs, ok := errorsResp.Data["errors"].([]interface{}); ok {
				for _, e := range errs {
					if em, ok := e.(map[string]interface{}); ok {
						msg, _ := em["message"].(string)
						b.WriteString(fmt.Sprintf("  %s\n", msg))
					}
				}
			}
		}
		result := b.String()
		if strings.TrimSpace(result) == "console messages:\njs errors:" {
			return "console: (no messages)", nil
		}
		return result, nil
	}

	return "", nil
}

func byteIndex(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

func NewBuiltinBrowserTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolBrowser,
			Desc: "Headless browser automation via agent-browser (Playwright/Chromium): navigate, snapshot (accessibility tree), click elements by ref (e.g. @e5), type text, scroll, go back, press keys, take screenshots, evaluate JavaScript, get console messages. Sessions isolated per task_id.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "navigate, snapshot, click, type, scroll, back, press, screenshot, eval, console, close", Required: false},
				"url":       {Type: einoschema.String, Desc: "URL to navigate to (required for navigate)", Required: false},
				"ref":       {Type: einoschema.String, Desc: "Element reference from snapshot (e.g., @e5) for click/type", Required: false},
				"text":      {Type: einoschema.String, Desc: "Text to type (for type) or key to press (for press)", Required: false},
				"js":        {Type: einoschema.String, Desc: "JavaScript expression to evaluate (required for eval)", Required: false},
				"task_id":   {Type: einoschema.String, Desc: "Session identifier (default: default); each task_id gets its own browser session", Required: false},
				"direction": {Type: einoschema.String, Desc: "Scroll direction: up or down (default: down)", Required: false},
				"full":      {Type: einoschema.String, Desc: "Full page screenshot (true/false, default: false)", Required: false},
				"clear":     {Type: einoschema.String, Desc: "Clear console/errors after reading (true/false)", Required: false},
			}),
		},
		execBuiltinBrowser,
	)
}
