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

const toolIstio = "builtin_istio"

var allowedIstioOps = map[string]bool{
	"list_vs":        true,
	"list_dr":        true,
	"list_gw":        true,
	"list_se":        true,
	"describe":       true,
	"sidecar_status": true,
	"analyze":        true,
}

var istioResourceMap = map[string]string{
	"vs": "virtualservice",
	"dr": "destinationrule",
	"gw": "gateway",
	"se": "serviceentry",
}

func execBuiltinIstio(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list_vs"
	}

	if !allowedIstioOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedIstioOps)
	}

	resourceType := strArg(in, "resource_type", "type", "resource")
	name := strArg(in, "name", "resource_name")
	namespace := strArg(in, "namespace", "ns")
	kubeconfig := strArg(in, "kubeconfig", "kube", "kc")
	extraArgs := strArg(in, "extra_args", "extra_args")

	var cmdArgs []string

	if kubeconfig != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", kubeconfig)
	}

	switch op {
	case "list_vs":
		cmdArgs = append(cmdArgs, "get", "virtualservices", "--all-namespaces")
	case "list_dr":
		cmdArgs = append(cmdArgs, "get", "destinationrules", "--all-namespaces")
	case "list_gw":
		cmdArgs = append(cmdArgs, "get", "gateways", "--all-namespaces")
	case "list_se":
		cmdArgs = append(cmdArgs, "get", "serviceentries", "--all-namespaces")
	case "describe":
		if resourceType == "" || name == "" {
			return "", fmt.Errorf("resource_type and name are required for describe")
		}
		cr := istioResourceMap[resourceType]
		if cr == "" {
			cr = resourceType
		}
		cmdArgs = append(cmdArgs, "describe", cr, name)
		if namespace != "" {
			cmdArgs = append(cmdArgs, "-n", namespace)
		}
	case "sidecar_status":
		cmdArgs = append(cmdArgs, "get", "pods", "--all-namespaces", "-l", "sidecar.istio.io/inject=true")
	case "analyze":
		cmdArgs = append(cmdArgs, "analyze", "--all-namespaces")
		if resourceType != "" {
			cr := istioResourceMap[resourceType]
			if cr == "" {
				cr = resourceType
			}
			cmdArgs = append(cmdArgs, "--"+cr)
		}
	}

	if extraArgs != "" {
		cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
	}

	cmd := exec.Command("kubectl", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("kubectl %s failed: %s\n%s", op, err.Error(), string(output)), nil
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("istio %s: (no output)", op), nil
	}
	return fmt.Sprintf("istio %s result:\n\n%s", op, result), nil
}

func NewBuiltinIstioTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolIstio,
			Desc:  "Read-only Istio service mesh inspection: list VirtualServices/DestinationRules/Gateways/ServiceEntries, describe, check sidecar status, analyze mesh.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":    {Type: einoschema.String, Desc: "Operation: list_vs, list_dr, list_gw, list_se, describe, sidecar_status, analyze", Required: false},
				"resource_type": {Type: einoschema.String, Desc: "Resource type: vs, dr, gw, se (for describe/analyze)", Required: false},
				"name":         {Type: einoschema.String, Desc: "Resource name", Required: false},
				"namespace":    {Type: einoschema.String, Desc: "Kubernetes namespace", Required: false},
				"kubeconfig":   {Type: einoschema.String, Desc: "Path to kubeconfig file", Required: false},
				"extra_args":   {Type: einoschema.String, Desc: "Extra kubectl arguments", Required: false},
			}),
		},
		execBuiltinIstio,
	)
}
