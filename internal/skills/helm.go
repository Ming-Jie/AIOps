package skills

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolHelm = "builtin_helm"

var allowedHelmOps = map[string]bool{
	"list_repos":    true,
	"search":        true,
	"list_releases": true,
	"status":        true,
	"history":       true,
	"get_values":    true,
	"install":       true,
	"upgrade":       true,
	"rollback":      true,
	"uninstall":     true,
}

var writeHelmOps = map[string]bool{
	"install":   true,
	"upgrade":   true,
	"rollback":  true,
	"uninstall": true,
}

func execBuiltinHelm(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list_releases"
	}

	if !allowedHelmOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedHelmOps)
	}

	release := strArg(in, "release", "name", "release_name")
	chart := strArg(in, "chart", "chart_ref")
	namespace := strArg(in, "namespace", "ns")
	if namespace == "" {
		namespace = "default"
	}
	repo := strArg(in, "repo", "repo_name")
	revision := strArg(in, "revision", "rev")
	values := strArg(in, "values", "values_yaml")
	extraArgs := strArg(in, "extra_args", "extra_args")

	if writeHelmOps[op] {
		if op == "rollback" && revision == "" {
			return "", fmt.Errorf("revision is required for rollback")
		}
	}

	var cmdArgs []string

	switch op {
	case "list_repos":
		cmdArgs = []string{"repo", "list"}
	case "search":
		if chart == "" {
			return "", fmt.Errorf("chart is required for search")
		}
		cmdArgs = []string{"search", "repo", chart}
		if repo != "" {
			cmdArgs = append(cmdArgs, "--repo", repo)
		}
	case "list_releases":
		cmdArgs = []string{"list", "--namespace", namespace, "--all"}
	case "status":
		if release == "" {
			return "", fmt.Errorf("release is required for status")
		}
		cmdArgs = []string{"status", release, "--namespace", namespace}
	case "history":
		if release == "" {
			return "", fmt.Errorf("release is required for history")
		}
		cmdArgs = []string{"history", release, "--namespace", namespace}
	case "get_values":
		if release == "" {
			return "", fmt.Errorf("release is required for get_values")
		}
		cmdArgs = []string{"get", "values", release, "--namespace", namespace, "--all"}
	case "install":
		if release == "" || chart == "" {
			return "", fmt.Errorf("release and chart are required for install")
		}
		cmdArgs = []string{"install", release, chart, "--namespace", namespace}
	case "upgrade":
		if release == "" || chart == "" {
			return "", fmt.Errorf("release and chart are required for upgrade")
		}
		cmdArgs = []string{"upgrade", release, chart, "--namespace", namespace}
	case "rollback":
		if release == "" || revision == "" {
			return "", fmt.Errorf("release and revision are required for rollback")
		}
		cmdArgs = []string{"rollback", release, revision, "--namespace", namespace}
	case "uninstall":
		if release == "" {
			return "", fmt.Errorf("release is required for uninstall")
		}
		cmdArgs = []string{"uninstall", release, "--namespace", namespace}
	}

	if values != "" && (op == "install" || op == "upgrade") {
		cmdArgs = append(cmdArgs, "--values", "-")
	}

	if extraArgs != "" {
		cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
	}

	cmd := exec.Command("helm", cmdArgs...)
	if values != "" && (op == "install" || op == "upgrade") {
		cmd.Stdin = strings.NewReader(values)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("helm %s failed: %s\n%s", op, err.Error(), string(output)), nil
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("helm %s: (completed)", op), nil
	}
	return fmt.Sprintf("helm %s result:\n\n%s", op, result), nil
}

func NewBuiltinHelmTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolHelm,
			Desc:  "Manage Helm charts and releases: list repos, search charts, list releases, status, history, get values, install, upgrade, rollback, uninstall.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":  {Type: einoschema.String, Desc: "Operation: list_repos, search, list_releases, status, history, get_values, install, upgrade, rollback, uninstall", Required: false},
				"release":    {Type: einoschema.String, Desc: "Release name", Required: false},
				"chart":      {Type: einoschema.String, Desc: "Chart reference", Required: false},
				"namespace":  {Type: einoschema.String, Desc: "Kubernetes namespace (default: default)", Required: false},
				"repo":       {Type: einoschema.String, Desc: "Helm repo name", Required: false},
				"revision":   {Type: einoschema.String, Desc: "Revision number for rollback", Required: false},
				"values":     {Type: einoschema.String, Desc: "YAML values content for install/upgrade", Required: false},
				"extra_args": {Type: einoschema.String, Desc: "Extra Helm CLI arguments", Required: false},
			}),
		},
		execBuiltinHelm,
	)
}
