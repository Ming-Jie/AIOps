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

const toolIngress = "builtin_ingress"

var allowedIngressOps = map[string]bool{
	"list":              true,
	"describe":          true,
	"controller_status": true,
	"check_endpoints":   true,
	"check_tls":         true,
}

func execBuiltinIngress(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list"
	}

	if !allowedIngressOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedIngressOps)
	}

	name := strArg(in, "name", "ingress_name", "resource")
	if name == "" && op != "list" && op != "controller_status" {
		return "", fmt.Errorf("name is required for %s", op)
	}

	namespace := strArg(in, "namespace", "ns")
	kubeconfig := strArg(in, "kubeconfig", "kube", "kc")
	extraArgs := strArg(in, "extra_args", "extra_args")

	var cmdArgs []string

	if kubeconfig != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", kubeconfig)
	}
	if namespace != "" {
		cmdArgs = append(cmdArgs, "-n", namespace)
	}

	switch op {
	case "list":
		cmdArgs = append(cmdArgs, "get", "ingress", "--all-namespaces")
		if extraArgs != "" {
			cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
		}
	case "describe":
		cmdArgs = append(cmdArgs, "describe", "ingress", name)
		if extraArgs != "" {
			cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
		}
	case "controller_status":
		cmdArgs = append(cmdArgs, "get", "pods", "-n", "ingress-nginx", "-l", "app.kubernetes.io/name=ingress-nginx")
	case "check_endpoints":
		cmdArgs = append(cmdArgs, "get", "ingress", name, "-o", "json")
		cmdArgs = append(cmdArgs, "--all-namespaces")
	case "check_tls":
		cmdArgs = append(cmdArgs, "get", "ingress", name, "-o", "jsonpath={.spec.tls[*]}")
		cmdArgs = append(cmdArgs, "--all-namespaces")
	}

	cmdName := "kubectl"
	cmd := exec.Command(cmdName, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("kubectl %s failed: %s\n%s", op, err.Error(), string(output)), nil
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("ingress %s: (no output)", op), nil
	}
	return fmt.Sprintf("ingress %s result:\n\n%s", op, result), nil
}

func NewBuiltinIngressTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolIngress,
			Desc:  "Read-only Kubernetes Ingress diagnostics: list, describe, check controller status, check endpoints, check TLS.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":  {Type: einoschema.String, Desc: "Operation: list, describe, controller_status, check_endpoints, check_tls", Required: false},
				"name":       {Type: einoschema.String, Desc: "Ingress resource name", Required: false},
				"namespace":  {Type: einoschema.String, Desc: "Kubernetes namespace", Required: false},
				"kubeconfig": {Type: einoschema.String, Desc: "Path to kubeconfig file", Required: false},
				"extra_args": {Type: einoschema.String, Desc: "Extra kubectl arguments", Required: false},
			}),
		},
		execBuiltinIngress,
	)
}
