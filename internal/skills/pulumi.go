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

const toolPulumi = "builtin_pulumi"

var allowedPulumiOps = map[string]bool{
	"list_stacks": true,
	"preview":     true,
	"resources":   true,
	"outputs":     true,
	"config":      true,
	"history":     true,
}

func execBuiltinPulumi(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list_stacks"
	}

	if !allowedPulumiOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedPulumiOps)
	}

	stack := strArg(in, "stack", "stack_name")
	path := strArg(in, "path", "dir", "directory")
	extraArgs := strArg(in, "extra_args", "extra_args")

	var cmdArgs []string
	if path != "" {
		cmdArgs = append(cmdArgs, "--cwd", path)
	}

	switch op {
	case "list_stacks":
		cmdArgs = append(cmdArgs, "stack", "ls")
	case "preview":
		cmdArgs = append(cmdArgs, "preview")
		if stack != "" {
			cmdArgs = append(cmdArgs, "--stack", stack)
		}
	case "resources":
		cmdArgs = append(cmdArgs, "stack", "ls")
		if stack != "" {
			cmdArgs = append(cmdArgs, "--stack", stack)
		}
	case "outputs":
		cmdArgs = append(cmdArgs, "stack", "output")
		if stack != "" {
			cmdArgs = append(cmdArgs, "--stack", stack)
		}
	case "config":
		cmdArgs = append(cmdArgs, "config")
		if stack != "" {
			cmdArgs = append(cmdArgs, "--stack", stack)
		}
	case "history":
		cmdArgs = append(cmdArgs, "stack", "history")
		if stack != "" {
			cmdArgs = append(cmdArgs, "--stack", stack)
		}
	}

	if extraArgs != "" {
		cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
	}

	cmd := exec.Command("pulumi", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("pulumi %s failed: %s\n%s", op, err.Error(), string(output)), nil
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("pulumi %s: (no output)", op), nil
	}
	return fmt.Sprintf("pulumi %s result:\n\n%s", op, result), nil
}

func NewBuiltinPulumiTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolPulumi,
			Desc:  "Read-only Pulumi operations: list stacks, preview changes, list resources, get outputs, config, history.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":  {Type: einoschema.String, Desc: "Operation: list_stacks, preview, resources, outputs, config, history", Required: false},
				"stack":      {Type: einoschema.String, Desc: "Stack name (default: current)", Required: false},
				"path":       {Type: einoschema.String, Desc: "Pulumi project directory (default: current)", Required: false},
				"extra_args": {Type: einoschema.String, Desc: "Extra Pulumi CLI arguments", Required: false},
			}),
		},
		execBuiltinPulumi,
	)
}
