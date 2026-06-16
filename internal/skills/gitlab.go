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

const toolGitLab = "builtin_gitlab"

var allowedGitLabOps = map[string]bool{
	"list_projects":  true,
	"get_project":    true,
	"list_mrs":       true,
	"list_pipelines": true,
	"get_job_log":    true,
	"list_branches":  true,
	"list_tags":      true,
}

func execBuiltinGitLab(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list_projects"
	}

	if !allowedGitLabOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedGitLabOps)
	}

	gitlabURL := strArg(in, "gitlab_url", "url", "base_url")
	if gitlabURL == "" {
		gitlabURL = "https://gitlab.com"
	}
	gitlabURL = strings.TrimRight(gitlabURL, "/")

	token := strArg(in, "token", "gitlab_token", "access_token")
	projectID := strArg(in, "project_id", "project", "id")
	mrIID := strArg(in, "mr_iid", "mr", "merge_request_iid")
	pipelineID := strArg(in, "pipeline_id", "pipeline")
	perPage := strArg(in, "per_page", "per_page", "limit")
	if perPage == "" {
		perPage = "20"
	}

	client := &http.Client{}
	apiPath := gitlabURL + "/api/v4"

	switch op {
	case "list_projects":
		apiPath += "/projects?per_page=" + perPage + "&simple=true"
	case "get_project":
		if projectID == "" {
			return "", fmt.Errorf("project_id is required for get_project")
		}
		apiPath += "/projects/" + projectID
	case "list_mrs":
		if projectID == "" {
			return "", fmt.Errorf("project_id is required for list_mrs")
		}
		apiPath += "/projects/" + projectID + "/merge_requests?per_page=" + perPage
	case "list_pipelines":
		if projectID == "" {
			return "", fmt.Errorf("project_id is required for list_pipelines")
		}
		apiPath += "/projects/" + projectID + "/pipelines?per_page=" + perPage
		if mrIID != "" {
			apiPath += "&merge_request_iid=" + mrIID
		}
	case "get_job_log":
		if projectID == "" || pipelineID == "" {
			return "", fmt.Errorf("project_id and pipeline_id are required for get_job_log")
		}
		apiPath += "/projects/" + projectID + "/pipelines/" + pipelineID + "/jobs?per_page=" + perPage
	case "list_branches":
		if projectID == "" {
			return "", fmt.Errorf("project_id is required for list_branches")
		}
		apiPath += "/projects/" + projectID + "/repository/branches?per_page=" + perPage
	case "list_tags":
		if projectID == "" {
			return "", fmt.Errorf("project_id is required for list_tags")
		}
		apiPath += "/projects/" + projectID + "/repository/tags?per_page=" + perPage
	}

	req, err := http.NewRequest(http.MethodGet, apiPath, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	if token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Failed to connect to GitLab at %s: %v", gitlabURL, err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("GitLab returned HTTP %d: %s", resp.StatusCode, string(body)), nil
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	return fmt.Sprintf("gitlab %s result:\n\n%s", op, string(pretty)), nil
}

func NewBuiltinGitLabTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolGitLab,
			Desc:  "Read-only GitLab API operations: list projects, get project, list MRs, list pipelines, get job logs, list branches, list tags.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":   {Type: einoschema.String, Desc: "Operation: list_projects, get_project, list_mrs, list_pipelines, get_job_log, list_branches, list_tags", Required: false},
				"project_id":  {Type: einoschema.String, Desc: "Project ID or URL-encoded path", Required: false},
				"gitlab_url":  {Type: einoschema.String, Desc: "GitLab instance URL (default: https://gitlab.com)", Required: false},
				"token":       {Type: einoschema.String, Desc: "GitLab personal access token", Required: false},
				"mr_iid":      {Type: einoschema.String, Desc: "Merge request IID (for pipeline listing)", Required: false},
				"pipeline_id": {Type: einoschema.String, Desc: "Pipeline ID (for get_job_log)", Required: false},
				"per_page":    {Type: einoschema.String, Desc: "Results per page (default: 20)", Required: false},
			}),
		},
		execBuiltinGitLab,
	)
}
