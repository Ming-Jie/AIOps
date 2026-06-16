package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolSonarQube = "builtin_sonarqube"

var allowedSonarOps = map[string]bool{
	"list_projects":   true,
	"project_status":  true,
	"metrics":         true,
	"list_issues":     true,
	"quality_gate":    true,
	"measures":        true,
}

func execBuiltinSonarQube(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list_projects"
	}

	if !allowedSonarOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedSonarOps)
	}

	sqURL := strArg(in, "sonarqube_url", "url", "endpoint")
	if sqURL == "" {
		sqURL = "http://localhost:9000"
	}
	sqURL = strings.TrimRight(sqURL, "/")

	token := strArg(in, "token", "sonar_token", "auth_token")
	projectKey := strArg(in, "project_key", "project", "key")
	severity := strArg(in, "severity", "sev")
	types := strArg(in, "types", "issue_types")
	perPage := strArg(in, "per_page", "per_page", "limit")
	if perPage == "" {
		perPage = "20"
	}

	client := &http.Client{}
	apiPath := sqURL + "/api"

	switch op {
	case "list_projects":
		apiPath += "/projects/search?ps=" + perPage
	case "project_status":
		if projectKey == "" {
			return "", fmt.Errorf("project_key is required for project_status")
		}
		apiPath += "/qualitygates/project_status?projectKey=" + projectKey
	case "metrics":
		if projectKey == "" {
			return "", fmt.Errorf("project_key is required for metrics")
		}
		apiPath += "/measures/component?component=" + projectKey + "&metricKeys=ncloc,bugs,vulnerabilities,code_smells,coverage,duplicated_lines_density,alert_status,reliability_rating,security_rating"
	case "list_issues":
		apiPath += "/issues/search?ps=" + perPage
		if projectKey != "" {
			apiPath += "&componentKeys=" + projectKey
		}
		if severity != "" {
			apiPath += "&severities=" + severity
		}
		if types != "" {
			apiPath += "&types=" + types
		}
	case "quality_gate":
		apiPath += "/qualitygates/list"
	case "measures":
		if projectKey == "" {
			return "", fmt.Errorf("project_key is required for measures")
		}
		apiPath += "/measures/search?projectKeys=" + projectKey + "&metricKeys=ncloc,bugs,vulnerabilities,code_smells,coverage"
	}

	req, err := http.NewRequest(http.MethodGet, apiPath, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	if token != "" {
		req.SetBasicAuth(token, "")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Failed to connect to SonarQube at %s: %v", sqURL, err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("SonarQube returned HTTP %d: %s", resp.StatusCode, string(body)), nil
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	return fmt.Sprintf("sonarqube %s result:\n\n%s", op, string(pretty)), nil
}

func NewBuiltinSonarQubeTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolSonarQube,
			Desc:  "Read-only SonarQube operations: list projects, project quality gate status, metrics, list issues, quality gate details, measures.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":    {Type: einoschema.String, Desc: "Operation: list_projects, project_status, metrics, list_issues, quality_gate, measures", Required: false},
				"project_key":  {Type: einoschema.String, Desc: "SonarQube project key", Required: false},
				"sonarqube_url": {Type: einoschema.String, Desc: "SonarQube URL (default: http://localhost:9000)", Required: false},
				"token":        {Type: einoschema.String, Desc: "SonarQube auth token", Required: false},
				"severity":     {Type: einoschema.String, Desc: "Filter issues by severity: INFO, MINOR, MAJOR, CRITICAL, BLOCKER", Required: false},
				"types":        {Type: einoschema.String, Desc: "Filter issues by type: BUG, VULNERABILITY, CODE_SMELL", Required: false},
				"per_page":     {Type: einoschema.String, Desc: "Results per page (default: 20)", Required: false},
			}),
		},
		execBuiltinSonarQube,
	)
}
