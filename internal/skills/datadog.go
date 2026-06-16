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

const toolDatadog = "builtin_datadog"

var allowedDatadogOps = map[string]bool{
	"list_monitors":  true,
	"get_monitor":    true,
	"list_dashboards": true,
	"query_metrics":  true,
	"list_incidents": true,
	"list_slos":      true,
}

func execBuiltinDatadog(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list_monitors"
	}

	if !allowedDatadogOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedDatadogOps)
	}

	ddSite := strArg(in, "dd_site", "site")
	if ddSite == "" {
		ddSite = "datadoghq.com"
	}

	apiKey := strArg(in, "api_key", "apiKey", "dd_api_key")
	appKey := strArg(in, "app_key", "appKey", "dd_app_key")
	monitorID := strArg(in, "monitor_id", "monitor", "id")
	query := strArg(in, "query", "metric_query")
	perPage := strArg(in, "per_page", "per_page", "limit")
	if perPage == "" {
		perPage = "20"
	}

	if apiKey == "" || appKey == "" {
		return "", fmt.Errorf("api_key and app_key are required for Datadog API")
	}

	baseURL := "https://api." + ddSite
	client := &http.Client{}
	var apiPath string

	switch op {
	case "list_monitors":
		apiPath = baseURL + "/api/v1/monitor?per_page=" + perPage
	case "get_monitor":
		if monitorID == "" {
			return "", fmt.Errorf("monitor_id is required for get_monitor")
		}
		apiPath = baseURL + "/api/v1/monitor/" + monitorID
	case "list_dashboards":
		apiPath = baseURL + "/api/v1/dashboard?count=" + perPage
	case "query_metrics":
		if query == "" {
			return "", fmt.Errorf("query is required for query_metrics (e.g. 'avg:system.cpu.user{*}by{host}')")
		}
		apiPath = baseURL + "/api/v1/query?query=" + query
	case "list_incidents":
		apiPath = baseURL + "/api/v2/incidents?page[size]=" + perPage
	case "list_slos":
		apiPath = baseURL + "/api/v1/slo?per_page=" + perPage
	}

	req, err := http.NewRequest(http.MethodGet, apiPath, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("DD-API-KEY", apiKey)
	req.Header.Set("DD-APPLICATION-KEY", appKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Failed to connect to Datadog at %s: %v", baseURL, err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Datadog returned HTTP %d: %s", resp.StatusCode, string(body)), nil
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	return fmt.Sprintf("datadog %s result:\n\n%s", op, string(pretty)), nil
}

func NewBuiltinDatadogTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolDatadog,
			Desc:  "Read-only Datadog operations: list/get monitors, list dashboards, query metrics, list incidents, list SLOs.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":  {Type: einoschema.String, Desc: "Operation: list_monitors, get_monitor, list_dashboards, query_metrics, list_incidents, list_slos", Required: false},
				"monitor_id": {Type: einoschema.String, Desc: "Monitor ID (for get_monitor)", Required: false},
				"dd_site":    {Type: einoschema.String, Desc: "Datadog site (default: datadoghq.com, use datadoghq.eu for EU)", Required: false},
				"api_key":    {Type: einoschema.String, Desc: "Datadog API key (from DD_API_KEY env)", Required: false},
				"app_key":    {Type: einoschema.String, Desc: "Datadog Application key (from DD_APP_KEY env)", Required: false},
				"query":      {Type: einoschema.String, Desc: "Metric query (for query_metrics, e.g. 'avg:system.cpu.user{*}by{host}')", Required: false},
				"per_page":   {Type: einoschema.String, Desc: "Results per page (default: 20)", Required: false},
			}),
		},
		execBuiltinDatadog,
	)
}
