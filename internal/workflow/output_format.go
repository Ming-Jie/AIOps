package workflow

import (
	"fmt"
	"strings"
)

// FormatWorkflowOutput turns nested node payloads into a user-facing string
// (e.g. code stdout, LLM content) instead of Go map formatting.
func FormatWorkflowOutput(output any) string {
	if output == nil {
		return ""
	}

	switch v := output.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]any:
		if nodeType, _ := v["type"].(string); nodeType != "" {
			switch nodeType {
			case "end":
				return FormatWorkflowOutput(v["output"])
			case "code":
				if s, ok := v["output"].(string); ok {
					return strings.TrimSpace(s)
				}
			case "llm", "agent":
				if s, ok := v["content"].(string); ok {
					return strings.TrimSpace(s)
				}
			case "http":
				if s, ok := v["body"].(string); ok {
					return strings.TrimSpace(s)
				}
			}
		}
		for _, key := range []string{"output", "content", "body", "message", "result"} {
			if s, ok := v[key].(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}

	return strings.TrimSpace(fmt.Sprintf("%v", output))
}
