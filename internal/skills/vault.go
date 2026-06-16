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

const toolVault = "builtin_vault"

var allowedVaultOps = map[string]bool{
	"status":        true,
	"list_secrets":  true,
	"read_secret":   true,
	"list_auth":     true,
	"list_policies": true,
	"read_policy":   true,
}

func execBuiltinVault(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "status"
	}

	if !allowedVaultOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedVaultOps)
	}

	vaultURL := strArg(in, "vault_url", "url", "endpoint")
	if vaultURL == "" {
		vaultURL = "http://127.0.0.1:8200"
	}
	vaultURL = strings.TrimRight(vaultURL, "/")

	token := strArg(in, "token", "vault_token")
	secretPath := strArg(in, "path", "secret_path", "secret")
	engine := strArg(in, "engine", "secrets_engine")
	if engine == "" {
		engine = "secret"
	}

	client := &http.Client{}
	var req *http.Request
	var err error

	switch op {
	case "status":
		req, err = http.NewRequest(http.MethodGet, vaultURL+"/v1/sys/seal-status", nil)
	case "list_secrets":
		if secretPath == "" {
			return "", fmt.Errorf("path is required for list_secrets")
		}
		req, err = http.NewRequest("LIST", vaultURL+"/v1/"+engine+"/metadata/"+strings.Trim(secretPath, "/"), nil)
	case "read_secret":
		if secretPath == "" {
			return "", fmt.Errorf("path is required for read_secret")
		}
		req, err = http.NewRequest(http.MethodGet, vaultURL+"/v1/"+engine+"/data/"+strings.Trim(secretPath, "/"), nil)
	case "list_auth":
		req, err = http.NewRequest(http.MethodGet, vaultURL+"/v1/sys/auth", nil)
	case "list_policies":
		req, err = http.NewRequest(http.MethodGet, vaultURL+"/v1/sys/policies/acl", nil)
	case "read_policy":
		if secretPath == "" {
			return "", fmt.Errorf("policy name is required for read_policy")
		}
		req, err = http.NewRequest(http.MethodGet, vaultURL+"/v1/sys/policies/acl/"+secretPath, nil)
	}

	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	if token != "" {
		req.Header.Set("X-Vault-Token", token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Failed to connect to Vault at %s: %v", vaultURL, err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Vault returned HTTP %d: %s", resp.StatusCode, string(body)), nil
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	return fmt.Sprintf("vault %s result:\n\n%s", op, string(pretty)), nil
}

func NewBuiltinVaultTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolVault,
			Desc:  "Read-only HashiCorp Vault operations: check status, list/read KV secrets, list auth methods, list/read policies.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: status, list_secrets, read_secret, list_auth, list_policies, read_policy", Required: false},
				"path":      {Type: einoschema.String, Desc: "Secret path (for list_secrets/read_secret) or policy name (for read_policy)", Required: false},
				"vault_url": {Type: einoschema.String, Desc: "Vault server URL (default: http://127.0.0.1:8200)", Required: false},
				"token":     {Type: einoschema.String, Desc: "Vault token (uses VAULT_TOKEN env if empty)", Required: false},
				"engine":    {Type: einoschema.String, Desc: "KV secrets engine name (default: secret)", Required: false},
			}),
		},
		execBuiltinVault,
	)
}
