package skills

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/aiops/internal/imoutbound"
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
			Desc: "Save a text file to send to the user in IM (Lark/Feishu or DingTalk). Returns [[lark_file:filename]] — include that marker verbatim in your final reply so the bot uploads and sends the file. Only works during IM bot conversations.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"filename": {
					Type:     einoschema.String,
					Desc:     "File name with extension, e.g. report.txt or data.csv",
					Required: true,
				},
				"content": {
					Type:     einoschema.String,
					Desc:     "Full file body (UTF-8 text)",
					Required: true,
				},
			}),
		},
		func(ctx context.Context, in map[string]any) (string, error) {
			scope, ok := imoutbound.ScopeFromContext(ctx)
			if !ok {
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
			marker, err := store.WriteFile(scope, filename, content)
			if err != nil {
				return "", err
			}
			return "File saved for IM delivery. Include this marker in your final answer: " + marker, nil
		},
	)
}
