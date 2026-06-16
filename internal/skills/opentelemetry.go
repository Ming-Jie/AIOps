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

const toolOpenTelemetry = "builtin_opentelemetry"

var allowedOTelOps = map[string]bool{
	"list_services":  true,
	"list_traces":    true,
	"get_trace":      true,
	"search_traces":  true,
	"list_operations": true,
}

func execBuiltinOpenTelemetry(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list_services"
	}

	if !allowedOTelOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedOTelOps)
	}

	traceURL := strArg(in, "trace_url", "url", "endpoint")
	backend := strArg(in, "backend", "trace_backend")
	if backend == "" {
		backend = "jaeger"
	}
	backend = strings.ToLower(backend)

	service := strArg(in, "service", "service_name")
	traceID := strArg(in, "trace_id", "id")
	tags := strArg(in, "tags", "query_tags")
	limit := strArg(in, "limit", "max_results")
	if limit == "" {
		limit = "20"
	}
	start := strArg(in, "start", "start_time")
	end := strArg(in, "end", "end_time")

	client := &http.Client{}
	var apiURL string

	switch op {
	case "list_services":
		if backend == "tempo" {
			if traceURL == "" {
				traceURL = "http://localhost:3200"
			}
			apiURL = strings.TrimRight(traceURL, "/") + "/api/search/tags"
		} else {
			if traceURL == "" {
				traceURL = "http://localhost:16686"
			}
			apiURL = strings.TrimRight(traceURL, "/") + "/api/services"
		}
	case "list_traces":
		if service == "" {
			return "", fmt.Errorf("service is required for list_traces")
		}
		if backend == "tempo" {
			if traceURL == "" {
				traceURL = "http://localhost:3200"
			}
			apiURL = strings.TrimRight(traceURL, "/") + "/api/search?limit=" + limit + "&q=" + service
		} else {
			if traceURL == "" {
				traceURL = "http://localhost:16686"
			}
			apiURL = strings.TrimRight(traceURL, "/") + "/api/traces?service=" + service + "&limit=" + limit
		}
	case "get_trace":
		if traceID == "" {
			return "", fmt.Errorf("trace_id is required for get_trace")
		}
		if backend == "tempo" {
			if traceURL == "" {
				traceURL = "http://localhost:3200"
			}
			apiURL = strings.TrimRight(traceURL, "/") + "/api/traces/" + traceID
		} else {
			if traceURL == "" {
				traceURL = "http://localhost:16686"
			}
			apiURL = strings.TrimRight(traceURL, "/") + "/api/traces/" + traceID
		}
	case "search_traces":
		if service == "" {
			return "", fmt.Errorf("service is required for search_traces")
		}
		if backend == "tempo" {
			if traceURL == "" {
				traceURL = "http://localhost:3200"
			}
			apiURL = strings.TrimRight(traceURL, "/") + "/api/search?limit=" + limit + "&q=" + service
			if tags != "" {
				apiURL += " " + tags
			}
		} else {
			if traceURL == "" {
				traceURL = "http://localhost:16686"
			}
			apiURL = strings.TrimRight(traceURL, "/") + "/api/traces?service=" + service + "&limit=" + limit
			if tags != "" {
				apiURL += "&tags=" + tags
			}
			if start != "" {
				apiURL += "&start=" + start
			}
			if end != "" {
				apiURL += "&end=" + end
			}
		}
	case "list_operations":
		if service == "" {
			return "", fmt.Errorf("service is required for list_operations")
		}
		if backend == "tempo" {
			if traceURL == "" {
				traceURL = "http://localhost:3200"
			}
			apiURL = strings.TrimRight(traceURL, "/") + "/api/search/tag/" + service + "/values"
		} else {
			if traceURL == "" {
				traceURL = "http://localhost:16686"
			}
			apiURL = strings.TrimRight(traceURL, "/") + "/api/services/" + service + "/operations"
		}
	}

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("Failed to connect to trace backend at %s: %v", traceURL, err), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Trace backend returned HTTP %d: %s", resp.StatusCode, string(body)), nil
	}

	var result any
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	return fmt.Sprintf("opentelemetry %s result:\n\n%s", op, string(pretty)), nil
}

func NewBuiltinOpenTelemetryTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolOpenTelemetry,
			Desc:  "Read-only OpenTelemetry trace queries (Jaeger/Tempo): list services, list/search traces, get trace details, list operations.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: list_services, list_traces, get_trace, search_traces, list_operations", Required: false},
				"service":   {Type: einoschema.String, Desc: "Service name", Required: false},
				"trace_id":  {Type: einoschema.String, Desc: "Trace ID (hex string)", Required: false},
				"trace_url": {Type: einoschema.String, Desc: "Jaeger/Tempo query URL (default: http://localhost:16686 for Jaeger, http://localhost:3200 for Tempo)", Required: false},
				"backend":   {Type: einoschema.String, Desc: "Trace backend: jaeger or tempo (default: jaeger)", Required: false},
				"tags":      {Type: einoschema.String, Desc: "Query tags (for search_traces, e.g. http.status_code=500)", Required: false},
				"limit":     {Type: einoschema.String, Desc: "Max results (default: 20)", Required: false},
				"start":     {Type: einoschema.String, Desc: "Start time (Unix timestamp)", Required: false},
				"end":       {Type: einoschema.String, Desc: "End time (Unix timestamp)", Required: false},
			}),
		},
		execBuiltinOpenTelemetry,
	)
}
