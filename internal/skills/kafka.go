package skills

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolKafka = "builtin_kafka"

var allowedKafkaOps = map[string]bool{
	"list_topics":     true,
	"describe_topic":  true,
	"list_groups":     true,
	"describe_group":  true,
	"get_messages":    true,
	"get_config":      true,
	"broker_info":     true,
}

func execBuiltinKafka(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "list_topics"
	}

	if !allowedKafkaOps[op] {
		return "", fmt.Errorf("operation %q not allowed; allowed: %v", op, allowedKafkaOps)
	}

	topic := strArg(in, "topic", "t")
	group := strArg(in, "group", "consumer_group", "consumer")
	bootstrap := strArg(in, "bootstrap_server", "bootstrap", "server", "broker")
	if bootstrap == "" {
		bootstrap = "localhost:9092"
	}
	partitionStr := strArg(in, "partition", "part")
	offsetStr := strArg(in, "offset", "start_offset")
	limitStr := strArg(in, "limit", "max_messages")

	bootstrapArg := "--bootstrap-server=" + bootstrap

	var cmdName string
	var cmdArgs []string

	switch op {
	case "list_topics":
		cmdName = "kafka-topics"
		cmdArgs = []string{bootstrapArg, "--list"}
	case "describe_topic":
		if topic == "" {
			return "", fmt.Errorf("topic is required for describe_topic")
		}
		cmdName = "kafka-topics"
		cmdArgs = []string{bootstrapArg, "--describe", "--topic", topic}
	case "list_groups":
		cmdName = "kafka-consumer-groups"
		cmdArgs = []string{bootstrapArg, "--list"}
	case "describe_group":
		if group == "" {
			return "", fmt.Errorf("group is required for describe_group")
		}
		cmdName = "kafka-consumer-groups"
		cmdArgs = []string{bootstrapArg, "--describe", "--group", group}
	case "get_messages":
		if topic == "" {
			return "", fmt.Errorf("topic is required for get_messages")
		}
		cmdName = "kafka-console-consumer"
		cmdArgs = []string{bootstrapArg, "--topic", topic}
		partition := 0
		if partitionStr != "" {
			p, err := strconv.Atoi(partitionStr)
			if err == nil {
				partition = p
			}
		}
		cmdArgs = append(cmdArgs, "--partition", strconv.Itoa(partition))
		if offsetStr != "" {
			cmdArgs = append(cmdArgs, "--offset", offsetStr)
		} else {
			cmdArgs = append(cmdArgs, "--offset", "-1")
		}
		limit := 10
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}
		cmdArgs = append(cmdArgs, "--max-messages", strconv.Itoa(limit))
	case "get_config":
		if topic == "" {
			return "", fmt.Errorf("topic is required for get_config")
		}
		cmdName = "kafka-configs"
		cmdArgs = []string{bootstrapArg, "--describe", "--entity-type", "topics", "--entity-name", topic}
	case "broker_info":
		cmdName = "kafka-broker-api-versions"
		cmdArgs = []string{bootstrapArg, "--describe"}
	}

	cmd := exec.Command(cmdName, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("kafka %s failed: %s\n%s", op, err.Error(), string(output)), nil
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("kafka %s: (no output)", op), nil
	}
	return fmt.Sprintf("kafka %s result:\n\n%s", op, result), nil
}

func NewBuiltinKafkaTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolKafka,
			Desc:  "Read-only Kafka operations: list topics, describe topic, list/describe consumer groups, get messages, get config, broker info.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":       {Type: einoschema.String, Desc: "Operation: list_topics, describe_topic, list_groups, describe_group, get_messages, get_config, broker_info", Required: false},
				"topic":           {Type: einoschema.String, Desc: "Topic name", Required: false},
				"group":           {Type: einoschema.String, Desc: "Consumer group name", Required: false},
				"bootstrap_server": {Type: einoschema.String, Desc: "Kafka bootstrap server (default: localhost:9092)", Required: false},
				"partition":       {Type: einoschema.String, Desc: "Partition number (default: 0)", Required: false},
				"offset":          {Type: einoschema.String, Desc: "Offset to start from (-1 = latest)", Required: false},
				"limit":           {Type: einoschema.String, Desc: "Max messages to fetch (default: 10, max: 100)", Required: false},
			}),
		},
		execBuiltinKafka,
	)
}
