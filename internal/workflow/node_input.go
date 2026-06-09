package workflow

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fisk086/aiops/internal/model"
)

func cloneConfigMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func prepareNodeConfig(node *model.WorkflowNode, execCtx *ExecutionContext) map[string]any {
	config := cloneConfigMap(node.Config)

	resolved := resolveInputValues(config, execCtx)
	if len(resolved) == 0 {
		return config
	}

	mapping := map[string]any{}
	if existing, ok := config["input_mapping"].(map[string]any); ok {
		for k, v := range existing {
			mapping[k] = v
		}
	}
	for k, v := range resolved {
		mapping[k] = v
	}
	config["input_mapping"] = mapping

	switch node.Type {
	case string(TaskTypeMCP):
		mergeInputValuesIntoArguments(config, resolved)
	case string(TaskTypeTool):
		if v, ok := resolved["input"]; ok {
			config["tool_input"] = fmt.Sprintf("%v", v)
		} else if v, ok := resolved["tool_input"]; ok {
			config["tool_input"] = fmt.Sprintf("%v", v)
		}
	}

	return config
}

func resolveInputValues(config map[string]any, execCtx *ExecutionContext) map[string]any {
	raw, ok := config["input_values"]
	if !ok || raw == nil {
		return nil
	}
	in, ok := raw.(map[string]any)
	if !ok {
		return nil
	}

	taskInput := &TaskInput{
		Variables:   execCtx.Variables,
		NodeOutputs: execCtx.NodeOutputs,
		UserMessage: execCtx.UserMessage,
		VarContext:  execCtx.VarContext,
		Config:      config,
	}

	out := make(map[string]any, len(in))
	for key, val := range in {
		out[key] = resolveTemplateAny(val, taskInput)
	}
	return out
}

func mergeInputValuesIntoArguments(config map[string]any, resolved map[string]any) {
	args, err := parseArgumentsObject(config)
	if err != nil {
		args = map[string]any{}
	}
	for k, v := range resolved {
		args[k] = v
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return
	}
	config["arguments"] = string(encoded)
}

func parseArgumentsObject(config map[string]any) (map[string]any, error) {
	raw, ok := config["arguments"]
	if !ok || raw == nil {
		return map[string]any{}, nil
	}
	switch v := raw.(type) {
	case map[string]any:
		return v, nil
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return map[string]any{}, nil
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(s), &parsed); err != nil {
			return nil, err
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("arguments must be a JSON object")
	}
}

func inputValuePresent(values map[string]any, field string) bool {
	if values == nil {
		return false
	}
	val, ok := values[field]
	if !ok || val == nil {
		return false
	}
	if s, ok := val.(string); ok {
		return strings.TrimSpace(s) != ""
	}
	return true
}
