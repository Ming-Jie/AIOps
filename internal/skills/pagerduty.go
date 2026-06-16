package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolPagerDuty = "builtin_pagerduty"

var allowedPagerDutyOps = map[string]bool{
	"list_incidents":          true,
	"get_incident":            true,
	"list_services":           true,
	"list_schedules":          true,
	"on_call":                 true,
	"list_escalation_policies": true,
}

func execBuiltinPagerDuty(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list_incidents"
	}

	if !allowedPagerDutyOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedPagerDutyOps)
	}

	apiKey := strArg(in, "api_key", "apiKey", "pagerduty_api_key")
	incidentID := strArg(in, "incident_id", "incident", "id")
	status := strArg(in, "status", "incident_status")
	serviceID := strArg(in, "service_id", "service")
	perPage := strArg(in, "per_page", "per_page", "limit")
	if perPage == "" {
		perPage = "20"
	}

	if apiKey == "" {
		return "", fmt.Errorf("api_key is required for PagerDuty API")
	}

	client := &http.Client{}
	apiPath := "https://api.pagerduty.com"

	switch op {
	case "list_incidents":
		apiPath += "/incidents?limit=" + perPage
		if status != "" {
			apiPath += "&statuses[]=" + status
		}
		if serviceID != "" {
			apiPath += "&service_ids[]=" + serviceID
		}
	case "get_incident":
		if incidentID == "" {
			return "", fmt.Errorf("incident_id is required for get_incident")
		}
		apiPath += "/incidents/" + incidentID
	case "list_services":
		apiPath += "/services?limit=" + perPage
	case "list_schedules":
		apiPath += "/schedules?limit=" + perPage
	case "on_call":
		apiPath += "/oncalls?limit=" + perPage
	case "list_escalation_policies":
		apiPath += "/escalation_policies?limit=" + perPage
	}

	req, err := http.NewRequest(http.MethodGet, apiPath, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Token token="+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Failed to connect to PagerDuty: %v", err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("PagerDuty returned HTTP %d: %s", resp.StatusCode, string(body)), nil
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	return fmt.Sprintf("pagerduty %s result:\n\n%s", op, string(pretty)), nil
}

func NewBuiltinPagerDutyTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolPagerDuty,
			Desc:  "Read-only PagerDuty operations: list/get incidents, list services, list schedules, who is on-call, list escalation policies.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":   {Type: einoschema.String, Desc: "Operation: list_incidents, get_incident, list_services, list_schedules, on_call, list_escalation_policies", Required: false},
				"incident_id": {Type: einoschema.String, Desc: "Incident ID (for get_incident)", Required: false},
				"api_key":     {Type: einoschema.String, Desc: "PagerDuty API key (from PAGERDUTY_API_KEY env)", Required: false},
				"status":      {Type: einoschema.String, Desc: "Filter incidents by status: triggered, acknowledged, resolved", Required: false},
				"service_id":  {Type: einoschema.String, Desc: "Filter by service ID", Required: false},
				"per_page":    {Type: einoschema.String, Desc: "Results per page (default: 20)", Required: false},
			}),
		},
		execBuiltinPagerDuty,
	)
}
