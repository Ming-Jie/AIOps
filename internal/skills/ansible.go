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

const toolAnsible = "builtin_ansible"

var allowedAnsibleOps = map[string]bool{
	"ad_hoc":         true,
	"playbook_check": true,
	"playbook_run":   true,
	"inventory":      true,
	"list_hosts":     true,
}

var writeAnsibleOps = map[string]bool{
	"playbook_run": true,
}

func execBuiltinAnsible(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "inventory"
	}

	if !allowedAnsibleOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedAnsibleOps)
	}

	module := strArg(in, "module", "mod")
	args := strArg(in, "args", "module_args")
	playbook := strArg(in, "playbook", "playbook_path")
	inventory := strArg(in, "inventory", "inv", "inventory_file")
	hosts := strArg(in, "hosts", "pattern", "target")
	extraArgs := strArg(in, "extra_args", "extra_args")

	if inventory == "" {
		inventory = "/etc/ansible/hosts"
	}
	if hosts == "" {
		hosts = "all"
	}

	var cmdName string
	var cmdArgs []string

	switch op {
	case "ad_hoc":
		if module == "" {
			return "", fmt.Errorf("module is required for ad_hoc operation")
		}
		cmdName = "ansible"
		cmdArgs = []string{hosts, "-i", inventory, "-m", module}
		if args != "" {
			cmdArgs = append(cmdArgs, "-a", args)
		}
	case "playbook_check":
		if playbook == "" {
			return "", fmt.Errorf("playbook is required for playbook_check")
		}
		cmdName = "ansible-playbook"
		cmdArgs = []string{playbook, "-i", inventory, "--check", "--diff"}
	case "playbook_run":
		if playbook == "" {
			return "", fmt.Errorf("playbook is required for playbook_run")
		}
		cmdName = "ansible-playbook"
		cmdArgs = []string{playbook, "-i", inventory}
	case "inventory":
		cmdName = "ansible-inventory"
		cmdArgs = []string{"-i", inventory, "--list"}
	case "list_hosts":
		cmdName = "ansible"
		cmdArgs = []string{hosts, "-i", inventory, "--list-hosts"}
	}

	if extraArgs != "" {
		cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
	}

	cmd := exec.Command(cmdName, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("ansible %s failed: %s\n%s", op, err.Error(), string(output)), nil
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("ansible %s: (completed)", op), nil
	}
	return fmt.Sprintf("ansible %s result:\n\n%s", op, result), nil
}

func NewBuiltinAnsibleTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolAnsible,
			Desc:  "Run Ansible ad-hoc commands, playbook check/run, and inventory listing. Write operations require confirmation.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":  {Type: einoschema.String, Desc: "Operation: ad_hoc, playbook_check, playbook_run, inventory, list_hosts", Required: false},
				"module":     {Type: einoschema.String, Desc: "Ansible module name (for ad_hoc: ping, command, shell, setup, etc.)", Required: false},
				"args":       {Type: einoschema.String, Desc: "Module arguments (for ad_hoc)", Required: false},
				"playbook":   {Type: einoschema.String, Desc: "Path to playbook file", Required: false},
				"inventory":  {Type: einoschema.String, Desc: "Inventory path (default: /etc/ansible/hosts)", Required: false},
				"hosts":      {Type: einoschema.String, Desc: "Host pattern (default: all)", Required: false},
				"extra_args": {Type: einoschema.String, Desc: "Extra ansible/ansible-playbook arguments", Required: false},
			}),
		},
		execBuiltinAnsible,
	)
}
