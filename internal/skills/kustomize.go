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

const toolKustomize = "builtin_kustomize"

var allowedKustomizeOps = map[string]bool{
	"build":          true,
	"list_resources": true,
	"list_overlays":  true,
	"show_patches":   true,
	"edit_image":     true,
	"edit_namespace": true,
}

var writeKustomizeOps = map[string]bool{
	"edit_image":     true,
	"edit_namespace": true,
}

func execBuiltinKustomize(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list_resources"
	}

	if !allowedKustomizeOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedKustomizeOps)
	}

	path := strArg(in, "path", "dir", "directory")
	if path == "" {
		path = "."
	}

	switch op {
	case "build":
		cmd := exec.Command("kustomize", "build", path)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("kustomize build failed: %s\n%s", err.Error(), string(output)), nil
		}
		result := strings.TrimSpace(string(output))
		if result == "" {
			return "kustomize build: (no output)", nil
		}
		return fmt.Sprintf("kustomize build result:\n\n%s", result), nil

	case "list_resources":
		cmd := exec.Command("kustomize", "build", path)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("kustomize build failed: %s\n%s", err.Error(), string(output)), nil
		}
		lines := strings.Split(string(output), "\n")
		var resources []string
		var current string
		for _, line := range lines {
			if strings.HasPrefix(line, "apiVersion:") {
				if current != "" {
					resources = append(resources, strings.TrimSpace(current))
				}
				current = ""
			}
			if strings.HasPrefix(line, "kind:") {
				current = strings.TrimSpace(strings.TrimPrefix(line, "kind:"))
			}
			if strings.HasPrefix(line, "  name:") && current != "" {
				current += "/" + strings.TrimSpace(strings.TrimPrefix(line, "  name:"))
			}
		}
		if current != "" {
			resources = append(resources, strings.TrimSpace(current))
		}
		if len(resources) == 0 {
			return "kustomize list_resources: no resources found", nil
		}
		return fmt.Sprintf("kustomize resources (%d):\n\n%s", len(resources), strings.Join(resources, "\n")), nil

	case "list_overlays":
		overlay := strArg(in, "overlay", "overlay_name")
		if overlay != "" {
			cmd := exec.Command("kustomize", "build", path+"/overlays/"+overlay)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Sprintf("kustomize build overlay %s failed: %s\n%s", overlay, err.Error(), string(output)), nil
			}
			return fmt.Sprintf("kustomize overlay %s:\n\n%s", overlay, strings.TrimSpace(string(output))), nil
		}
		cmd := exec.Command("ls", "-1", path+"/overlays/")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("kustomize list_overlays failed: %s\n%s", err.Error(), string(output)), nil
		}
		result := strings.TrimSpace(string(output))
		if result == "" {
			return "kustomize list_overlays: no overlays found", nil
		}
		return fmt.Sprintf("kustomize overlays:\n\n%s", result), nil

	case "show_patches":
		cmd := exec.Command("kustomize", "build", path)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("kustomize build failed: %s\n%s", err.Error(), string(output)), nil
		}
		content := string(output)
		patches := extractPatches(content)
		if len(patches) == 0 {
			return "kustomize show_patches: no patches detected", nil
		}
		return fmt.Sprintf("kustomize patches (%d):\n\n%s", len(patches), strings.Join(patches, "\n---\n")), nil

	case "edit_image":
		image := strArg(in, "image", "image_tag")
		name := strArg(in, "name", "resource_name")
		if image == "" {
			return "", fmt.Errorf("image is required for edit_image (format: name:tag)")
		}
		args := []string{"edit", "set", "image"}
		if name != "" {
			args = append(args, name+"="+image)
		} else {
			args = append(args, image)
		}
		cmd := exec.Command("kustomize", args...)
		cmd.Dir = path
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("kustomize edit_image failed: %s\n%s", err.Error(), string(output)), nil
		}
		return "kustomize edit_image: updated successfully", nil

	case "edit_namespace":
		namespace := strArg(in, "namespace", "ns")
		if namespace == "" {
			return "", fmt.Errorf("namespace is required for edit_namespace")
		}
		cmd := exec.Command("kustomize", "edit", "set", "namespace", namespace)
		cmd.Dir = path
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("kustomize edit_namespace failed: %s\n%s", err.Error(), string(output)), nil
		}
		return "kustomize edit_namespace: updated successfully", nil
	}

	return "", nil
}

func extractPatches(content string) []string {
	var patches []string
	sections := strings.Split(content, "---\n")
	for _, s := range sections {
		if strings.Contains(s, "patches:") || strings.Contains(s, "patchesStrategicMerge:") || strings.Contains(s, "patchesJson6902:") {
			patches = append(patches, strings.TrimSpace(s))
		}
	}
	return patches
}

func NewBuiltinKustomizeTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolKustomize,
			Desc:  "Kustomize operations: build, list resources, list overlays, show patches, edit image/namespace.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: build, list_resources, list_overlays, show_patches, edit_image, edit_namespace", Required: false},
				"path":      {Type: einoschema.String, Desc: "Kustomize directory (default: current)", Required: false},
				"overlay":   {Type: einoschema.String, Desc: "Overlay name", Required: false},
				"image":     {Type: einoschema.String, Desc: "Image name:tag for edit_image", Required: false},
				"name":      {Type: einoschema.String, Desc: "Resource name for edit_image", Required: false},
				"namespace": {Type: einoschema.String, Desc: "Namespace for edit_namespace", Required: false},
			}),
		},
		execBuiltinKustomize,
	)
}
