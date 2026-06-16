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

const toolHarbor = "builtin_harbor"

var allowedHarborOps = map[string]bool{
	"list_projects":      true,
	"list_repos":         true,
	"list_artifacts":     true,
	"get_artifact":       true,
	"system_info":        true,
	"list_robot_accounts": true,
}

func execBuiltinHarbor(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list_projects"
	}

	if !allowedHarborOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedHarborOps)
	}

	harborURL := strArg(in, "harbor_url", "url", "endpoint")
	if harborURL == "" {
		harborURL = "http://localhost:80"
	}
	harborURL = strings.TrimRight(harborURL, "/")

	username := strArg(in, "username", "user", "harbor_user")
	password := strArg(in, "password", "pass", "harbor_password", "api_token")
	project := strArg(in, "project", "project_name")
	repository := strArg(in, "repository", "repo", "repo_name")
	artifact := strArg(in, "artifact", "reference", "tag")
	perPage := strArg(in, "per_page", "per_page", "limit")
	if perPage == "" {
		perPage = "20"
	}

	client := &http.Client{}
	apiPath := harborURL + "/api/v2.0"

	switch op {
	case "list_projects":
		apiPath += "/projects?page_size=" + perPage
	case "list_repos":
		if project == "" {
			return "", fmt.Errorf("project is required for list_repos")
		}
		apiPath += "/projects/" + project + "/repositories?page_size=" + perPage
	case "list_artifacts":
		if project == "" || repository == "" {
			return "", fmt.Errorf("project and repository are required for list_artifacts")
		}
		apiPath += "/projects/" + project + "/repositories/" + repository + "/artifacts?page_size=" + perPage
	case "get_artifact":
		if project == "" || repository == "" || artifact == "" {
			return "", fmt.Errorf("project, repository, and artifact are required for get_artifact")
		}
		apiPath += "/projects/" + project + "/repositories/" + repository + "/artifacts/" + artifact
	case "system_info":
		apiPath += "/systeminfo"
	case "list_robot_accounts":
		if project == "" {
			return "", fmt.Errorf("project is required for list_robot_accounts")
		}
		apiPath += "/projects/" + project + "/robots?page_size=" + perPage
	}

	req, err := http.NewRequest(http.MethodGet, apiPath, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Failed to connect to Harbor at %s: %v", harborURL, err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Harbor returned HTTP %d: %s", resp.StatusCode, string(body)), nil
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	return fmt.Sprintf("harbor %s result:\n\n%s", op, string(pretty)), nil
}

func NewBuiltinHarborTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolHarbor,
			Desc:  "Read-only Harbor registry operations: list projects, list repos, list/get artifacts, system info, list robot accounts.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: list_projects, list_repos, list_artifacts, get_artifact, system_info, list_robot_accounts", Required: false},
				"project":   {Type: einoschema.String, Desc: "Project name", Required: false},
				"repository": {Type: einoschema.String, Desc: "Repository name", Required: false},
				"artifact":  {Type: einoschema.String, Desc: "Artifact reference (tag or digest)", Required: false},
				"harbor_url": {Type: einoschema.String, Desc: "Harbor URL (default: http://localhost:80)", Required: false},
				"username":  {Type: einoschema.String, Desc: "Harbor username", Required: false},
				"password":  {Type: einoschema.String, Desc: "Harbor password or API token", Required: false},
				"per_page":  {Type: einoschema.String, Desc: "Results per page (default: 20)", Required: false},
			}),
		},
		execBuiltinHarbor,
	)
}
