package skills

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/aiops/internal/imoutbound"
	"github.com/fisk086/aiops/internal/logger"
)

const toolLarkOutboundFile = "builtin_lark_save_file"
const toolIMOutboundFile = "builtin_im_save_file"

func NewLarkOutboundFileTool(store *imoutbound.Store) tool.BaseTool {
	return newIMOutboundFileTool(store, toolLarkOutboundFile)
}

func NewIMOutboundFileTool(store *imoutbound.Store) tool.BaseTool {
	return newIMOutboundFileTool(store, toolIMOutboundFile)
}

func newIMOutboundFileTool(store *imoutbound.Store, name string) tool.BaseTool {
	if store == nil {
		store = imoutbound.GlobalStore()
	}
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: name,
			Desc: "Save a file for IM (Lark/Feishu/DingTalk). The bot uploads it as a separate file/image message — never put HTTP URLs in your reply. Call this tool to deliver files; [[lark_file:]] markers alone do nothing. IM sessions only.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"filename": {
					Type:     einoschema.String,
					Desc:     "File name with extension, e.g. report.txt, data.bin, chart.png",
					Required: true,
				},
				"content": {
					Type:     einoschema.String,
					Desc:     "File body: UTF-8 text, or base64, or data:image/png;base64,... (max 1MB)",
					Required: true,
				},
			}),
		},
		func(ctx context.Context, in map[string]any) (string, error) {
			scope, ok := imoutbound.ScopeFromContext(ctx)
			if !ok {
				logger.Warn("im_save_file: rejected — no IM scope on context", "tool", name)
				return "", fmt.Errorf("%s is only available in IM bot sessions", name)
			}
			filename := strArg(in, "filename", "file_name", "name")
			content := strArg(in, "content", "data", "body", "text")
			if filename == "" {
				return "", fmt.Errorf("missing filename")
			}
			if content == "" {
				return "", fmt.Errorf("missing content")
			}
			data, err := prepareIMFileBytes(filename, content)
			if err != nil {
				return "", err
			}
			if _, err := store.WriteFileBytes(scope, filename, data); err != nil {
				logger.Warn("im_save_file: outbound write failed",
					"tool", name, "file", filename, "session_id", scope.SessionID, "err", err)
				return "", err
			}
			imoutbound.RegisterWrittenFile(ctx, filename)
			logger.Info("im_save_file: file saved to outbound store",
				"tool", name, "file", filename, "session_id", scope.SessionID, "bytes", len(data))
			return fmt.Sprintf("已为 IM 登记附件 %s（%d 字节）。机器人将单独发送 file/image 消息。回复只写一句话，禁止 URL。", filename, len(data)), nil
		},
	)
}
