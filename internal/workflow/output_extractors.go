package workflow

import (
	"strings"

	"github.com/fisk086/aiops/internal/model"
)

// OutputExtractor returns state/output keys a node type writes from its config.
// Keep in lock-step with ui/src/lib/upstreamOutputs.ts declaredOutputs().
type OutputExtractor func(nodeType string, config map[string]any) []string

var outputExtractors = map[string]OutputExtractor{}

func RegisterOutputExtractor(nodeType string, extractor OutputExtractor) {
	outputExtractors[nodeType] = extractor
}

func DeclaredOutputs(nodeType string, config map[string]any) []string {
	if config == nil {
		config = map[string]any{}
	}
	if extractor, ok := outputExtractors[nodeType]; ok {
		return extractor(nodeType, config)
	}
	return nil
}

func identifierKeys(value any) []string {
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		if k != "" && isIdentifier(k) {
			out = append(out, k)
		}
	}
	return out
}

func stringValue(value any) string {
	s, ok := value.(string)
	if !ok || s == "" || !isIdentifier(s) {
		return ""
	}
	return s
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && r != '_' {
				return false
			}
			continue
		}
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func extractStartOutputs(_ string, config map[string]any) []string {
	out := []string{"message", "type"}
	for _, name := range inputFieldNames(config) {
		if !containsString(out, name) {
			out = append(out, name)
		}
	}
	if schema, ok := config["input_schema"].(map[string]any); ok {
		if props, ok := schema["properties"].(map[string]any); ok {
			for name := range props {
				if isIdentifier(name) && !containsString(out, name) {
					out = append(out, name)
				}
			}
		}
	}
	return out
}

func inputFieldNames(config map[string]any) []string {
	raw, ok := config["input_fields"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		name = strings.TrimSpace(name)
		if isIdentifier(name) {
			out = append(out, name)
		}
	}
	return out
}

func extractLlmOutputs(_ string, config map[string]any) []string {
	outVar := stringValue(config["output_var"])
	if outVar == "" {
		outVar = "content"
	}
	return []string{outVar, "type"}
}

func extractCodeOutputs(_ string, config map[string]any) []string {
	out := []string{"output", "error"}
	for _, name := range identifierKeys(config["outputs"]) {
		if !containsString(out, name) {
			out = append(out, name)
		}
	}
	return out
}

func extractVariableOutputs(_ string, config map[string]any) []string {
	out := []string{"assigned", "type"}
	for _, name := range identifierKeys(config["assignments"]) {
		out = append(out, "assigned."+name)
	}
	return out
}

func extractMcpOutputs(_ string, config map[string]any) []string {
	toolName := stringValue(config["tool_name"])
	if toolName == "" {
		toolName = stringValue(config["toolName"])
	}
	if toolName == "" {
		return []string{"tools", "tool_name", "mcp_config_id", "type"}
	}
	return []string{"tool_name", "tool_meta", "result", "type"}
}

func containsString(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

func initOutputExtractors() {
	RegisterOutputExtractor("start", extractStartOutputs)
	RegisterOutputExtractor("end", func(_ string, _ map[string]any) []string {
		return []string{"output", "type"}
	})
	RegisterOutputExtractor("agent", func(_ string, _ map[string]any) []string {
		return []string{"content", "type"}
	})
	RegisterOutputExtractor("llm", extractLlmOutputs)
	RegisterOutputExtractor("tool", func(_ string, config map[string]any) []string {
		out := []string{"type", "tool", "tool_meta"}
		if name := stringValue(config["tool_name"]); name != "" {
			out = append(out, "tool_name")
		}
		return out
	})
	RegisterOutputExtractor("mcp", extractMcpOutputs)
	RegisterOutputExtractor("http", func(_ string, _ map[string]any) []string {
		return []string{"body", "status_code", "url", "method", "error", "type"}
	})
	RegisterOutputExtractor("code", extractCodeOutputs)
	RegisterOutputExtractor("variable", extractVariableOutputs)
	RegisterOutputExtractor("condition", func(_ string, _ map[string]any) []string {
		return []string{"result", "branch", "condition", "type"}
	})
	RegisterOutputExtractor("knowledge", func(_ string, _ map[string]any) []string {
		return []string{"results", "query", "top_k", "type"}
	})
	RegisterOutputExtractor("template", func(_ string, _ map[string]any) []string {
		return []string{"output", "type"}
	})
	RegisterOutputExtractor("merge", func(_ string, _ map[string]any) []string {
		return []string{"outputs", "result", "mode", "type"}
	})
}

func CollectDeclaredOutputsForNode(node *model.WorkflowNode) []string {
	if node == nil {
		return nil
	}
	return DeclaredOutputs(node.Type, node.Config)
}

func init() {
	initOutputExtractors()
}
