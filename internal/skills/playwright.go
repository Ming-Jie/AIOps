package skills

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
)

const toolPlaywright = "builtin_playwright"

func findE2EDir() string {
	candidates := []string{"./e2e", "../e2e", "/app/e2e"}
	for _, d := range candidates {
		abs, err := filepath.Abs(d)
		if err != nil {
			continue
		}
		if fi, err := os.Stat(abs); err == nil && fi.IsDir() {
			return abs
		}
	}
	return ""
}

func execBuiltinPlaywright(ctx context.Context, in map[string]any) (string, error) {
	action := strArg(in, "action", "op", "operation")
	if action == "" {
		action = "run"
	}
	action = strings.ToLower(action)

	e2eDir := findE2EDir()
	if e2eDir == "" {
		return "e2e/ directory not found. Playwright E2E tests are not available in this environment.", nil
	}

	switch action {
	case "install":
		return runInstall(e2eDir)
	case "list":
		return runList(e2eDir)
	case "report":
		return getReport(ctx, e2eDir)
	case "run":
		return runTests(ctx, e2eDir, in)
	default:
		return "", fmt.Errorf("unknown action %q (supported: run, install, list, report)", action)
	}
}

func runInstall(e2eDir string) (string, error) {
	logger.Debug("playwright skill: installing browsers", "dir", e2eDir)

	cmd := exec.Command("npx", "playwright", "install", "--with-deps", "chromium")
	cmd.Dir = e2eDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Playwright install failed: %v\n\nOutput:\n%s", err, string(output)), nil
	}
	return fmt.Sprintf("Playwright browsers installed successfully:\n%s", string(output)), nil
}

func runList(e2eDir string) (string, error) {
	logger.Debug("playwright skill: listing tests", "dir", e2eDir)

	cmd := exec.Command("npx", "playwright", "test", "--list")
	cmd.Dir = e2eDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Failed to list tests: %v\n\nOutput:\n%s", err, string(output)), nil
	}
	return fmt.Sprintf("Available Playwright tests:\n%s", string(output)), nil
}

func getReport(ctx context.Context, e2eDir string) (string, error) {
	logger.Debug("playwright skill: checking report", "dir", e2eDir)

	reportPath := findReportDir(e2eDir)
	if reportPath == "" {
		return "No Playwright test report found. Run tests first with action=\"run\".", nil
	}

	return deliverReport(ctx, reportPath)
}

func findReportDir(e2eDir string) string {
	candidates := []string{
		filepath.Join(e2eDir, "reports", "html"),
		filepath.Join(e2eDir, "playwright-report"),
	}
	for _, d := range candidates {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			return d
		}
	}
	return ""
}

func deliverReport(ctx context.Context, reportDir string) (string, error) {
	indexHTML := filepath.Join(reportDir, "index.html")
	if _, err := os.Stat(indexHTML); os.IsNotExist(err) {
		return "Report directory found but index.html is missing.", nil
	}

	dataJS := filepath.Join(reportDir, "data.js")
	assetsDir := filepath.Join(reportDir, "assets")

	html, err := os.ReadFile(indexHTML)
	if err != nil {
		return "", fmt.Errorf("failed to read report: %w", err)
	}

	content := string(html)

	// Inline data.js if it exists
	if _, err := os.Stat(dataJS); err == nil {
		jsData, err := os.ReadFile(dataJS)
		if err == nil {
			// Remove the external data.js script tag and inline the content
			jsContent := string(jsData)
			dataScript := `<script src="data.js"></script>`
			inlineScript := fmt.Sprintf("<script>%s</script>", jsContent)
			content = strings.ReplaceAll(content, dataScript, inlineScript)

			// Also handle assets/data.js path
			dataScript2 := `<script src="assets/data.js"></script>`
			content = strings.ReplaceAll(content, dataScript2, inlineScript)
		}
	}

	// Inline asset files (CSS, JS)
	if _, err := os.Stat(assetsDir); err == nil {
		entries, _ := os.ReadDir(assetsDir)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := filepath.Ext(name)
			assetPath := filepath.Join(assetsDir, name)
			assetData, err := os.ReadFile(assetPath)
			if err != nil {
				continue
			}

			// Remove the external script/link tag and inline
			if ext == ".js" {
				// <script src="assets/styling.js"></script>
				for _, pattern := range []string{
					fmt.Sprintf(`<script src="assets/%s"></script>`, name),
					fmt.Sprintf(`<script src="assets/%s"`, name),
					fmt.Sprintf(`<script src="./assets/%s"></script>`, name),
				} {
					if strings.Contains(content, pattern) {
						inlineTag := fmt.Sprintf("<script>%s</script>", string(assetData))
						content = strings.ReplaceAll(content, pattern, inlineTag)
						break
					}
				}
			} else if ext == ".css" {
				for _, pattern := range []string{
					fmt.Sprintf(`<link rel="stylesheet" href="assets/%s">`, name),
					fmt.Sprintf(`<link rel="stylesheet" href="assets/%s"`, name),
					fmt.Sprintf(`<link rel="stylesheet" href="./assets/%s">`, name),
				} {
					if strings.Contains(content, pattern) {
						inlineTag := fmt.Sprintf("<style>%s</style>", string(assetData))
						content = strings.ReplaceAll(content, pattern, inlineTag)
						break
					}
				}
			}
		}
	}

	selfContained := []byte(content)

	// Deliver via IM or Web
	if scope, ok := imoutbound.ScopeFromContext(ctx); ok {
		filename := fmt.Sprintf("playwright-report-%d.html", time.Now().UnixMilli())
		if _, err := imoutbound.GlobalStore().WriteFileBytes(scope, filename, selfContained); err != nil {
			return "", fmt.Errorf("failed to write report to IM store: %w", err)
		}
		imoutbound.RegisterWrittenFile(ctx, filename)
		return fmt.Sprintf("[[im_file:%s]]\n\nTest report is ready as an HTML file above.", filename), nil
	}

	urlPath, _, err := SaveWebGeneratedFile("playwright-report.html", selfContained)
	if err != nil {
		return fmt.Sprintf("Report generated but failed to save for download: %v", err), nil
	}
	AddAttachment(ctx, &FileAttachment{
		Filename: "playwright-report.html",
		MimeType: "text/html",
		URL:      urlPath,
	})
	return fmt.Sprintf("Test report available: %s", urlPath), nil
}

func runTests(ctx context.Context, e2eDir string, in map[string]any) (string, error) {
	files := strArg(in, "files", "test_file", "file", "filter")
	headed := strArg(in, "headed", "gui", "show_browser")
	project := strArg(in, "project", "browser")
	workers := strArg(in, "workers", "parallel", "jobs")
	retries := strArg(in, "retries")
	reporter := strArg(in, "reporter")
	timeout := strArg(in, "timeout")

	args := []string{"playwright", "test"}

	if reporter == "" {
		reporter = "html,list"
	}
	args = append(args, "--reporter", reporter)

	if headed == "true" || headed == "yes" || headed == "1" {
		args = append(args, "--headed")
	} else {
		args = append(args, "--headless")
	}

	if project != "" {
		args = append(args, "--project", project)
	}

	if workers != "" {
		args = append(args, "--workers", workers)
	} else {
		args = append(args, "--workers", "2")
	}

	if retries != "" {
		args = append(args, "--retries", retries)
	} else {
		args = append(args, "--retries", "1")
	}

	if timeout != "" {
		args = append(args, "--timeout", timeout)
	} else {
		args = append(args, "--timeout", "60000")
	}

	if files != "" {
		for _, f := range strings.Split(files, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				args = append(args, f)
			}
		}
	}

	logger.Debug("playwright skill: running tests", "dir", e2eDir, "args", args)

	cmd := exec.Command("npx", args...)
	cmd.Dir = e2eDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		result := fmt.Sprintf("Playwright tests completed with failures:\n\n%s", string(output))
		return result, nil
	}

	result := fmt.Sprintf("Playwright tests passed:\n\n%s", string(output))

	// Try to deliver the report
	reportDir := findReportDir(e2eDir)
	if reportDir != "" {
		reportMsg, reportErr := deliverReport(ctx, reportDir)
		if reportErr == nil {
			result += "\n\n" + reportMsg
		}
	}

	return result, nil
}

func NewBuiltinPlaywrightTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolPlaywright,
			Desc: "Run Playwright end-to-end tests from the e2e/ directory. Supports running all tests, filtering by file, installing browsers, listing tests, and viewing/downloading the HTML test report.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"action":   {Type: einoschema.String, Desc: "Action: run (default), install, list, report", Required: false},
				"files":    {Type: einoschema.String, Desc: "Comma-separated test file paths to run (e.g. tests/auth.spec.ts)", Required: false},
				"headed":   {Type: einoschema.String, Desc: "Run headed (true) or headless (false, default)", Required: false},
				"project":  {Type: einoschema.String, Desc: "Playwright project: chromium (default), firefox, webkit", Required: false},
				"workers":  {Type: einoschema.String, Desc: "Parallel workers (default: 2)", Required: false},
				"retries":  {Type: einoschema.String, Desc: "Retry count on failure (default: 1)", Required: false},
				"reporter": {Type: einoschema.String, Desc: "Reporter format (default: html,list)", Required: false},
				"timeout":  {Type: einoschema.String, Desc: "Per-test timeout in ms (default: 60000)", Required: false},
			}),
		},
		execBuiltinPlaywright,
	)
}
