package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/fisk086/aiops/internal/logger"
	"github.com/fisk086/aiops/internal/schema"
)

func (r *Runtime) openChatStream(
	ctx context.Context,
	agent *schema.AgentWithRuntime,
	systemPrompt string,
	message string,
	historyMsgs []*einoschema.Message,
	chatTools []tool.BaseTool,
	clientType string,
	visionParts []VisionPart,
	sessionID, auditUserID string,
	stopCh <-chan struct{},
) (io.ReadCloser, error) {
	ctx = r.ensureUsageTracking(ctx, agent, auditUserID)

	ct := streamClientType(clientType)
	route := resolveChatStreamRoute(agent, ct, chatTools)
	logChatStreamRoute(agent, ct, chatTools, route)

	switch route {
	case "react_stream", "react_stream_desktop_client_tools":
		return r.runReActLoopStream(ctx, agent, systemPrompt, message, historyMsgs, chatTools, ct, visionParts, sessionID, auditUserID, stopCh)
	case "plan_execute_stream":
		return r.runPlanAndExecuteStream(ctx, agent, systemPrompt, message, historyMsgs, chatTools, ct, sessionID, auditUserID)
	case "adk_tool_loop":
		pr, pw := io.Pipe()
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("stream_chat: adk stream panic", "recover", rec)
				}
			}()
			defer pw.Close()
			defer r.flushUsageSession(ctx)
			if err := r.runAgentCoreStream(ctx, pw, agent, systemPrompt, message, historyMsgs, chatTools, ct, visionParts); err != nil {
				logger.Warn("chat stream adk", "phase", "run_failed", "agent_id", agent.ID, "err", err)
				_ = streamStaticTextAsSSE(pw, UserVisibleStreamFailure(err))
				if _, werr := fmt.Fprintf(pw, "data: [DONE]\n\n"); werr != nil {
					_ = pw.CloseWithError(werr)
					return
				}
				return
			}
			if _, err := fmt.Fprintf(pw, "data: [DONE]\n\n"); err != nil {
				_ = pw.CloseWithError(err)
			}
		}()
		return pr, nil
	}

	instruction := systemPrompt
	if instruction == "" {
		instruction = "You are a helpful AI assistant."
	}

	msgs := buildStreamChatMessages(instruction, historyMsgs, message, visionParts)

	pr, pw := io.Pipe()
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("stream_chat: plain stream panic", "recover", rec)
			}
		}()
		defer pw.Close()
		defer r.flushUsageSession(ctx)
		logger.Info("chat stream plain", "phase", "start", "history_msgs", len(historyMsgs))
		sr, err := r.chatModel.Stream(ctx, msgs)
		if err != nil {
			logger.Warn("chat stream plain", "phase", "stream_open", "err", err)
			_ = streamStaticTextAsSSE(pw, UserVisibleStreamFailure(err))
			if _, werr := fmt.Fprintf(pw, "data: [DONE]\n\n"); werr != nil {
				_ = pw.CloseWithError(werr)
			}
			return
		}
		defer sr.Close()
		chunkIdx := 0
		for {
			select {
			case <-stopCh:
				logger.Info("chat stream plain", "phase", "stopped", "chunks", chunkIdx)
				_ = pw.CloseWithError(io.EOF)
				return
			default:
			}
			chunk, err := sr.Recv()
			if errors.Is(err, io.EOF) {
				logger.Info("chat stream plain", "phase", "eof", "chunks", chunkIdx)
				break
			}
			if err != nil {
				logger.Warn("chat stream plain", "phase", "recv", "err", err, "chunks", chunkIdx)
				_ = streamStaticTextAsSSE(pw, UserVisibleStreamFailure(err))
				if _, werr := fmt.Fprintf(pw, "data: [DONE]\n\n"); werr != nil {
					_ = pw.CloseWithError(werr)
				}
				return
			}
			if chunk == nil {
				continue
			}
			chunkIdx++
			if len(chunk.ToolCalls) > 0 && strings.TrimSpace(chunk.Content) == "" && strings.TrimSpace(chunk.ReasoningContent) == "" {
				_ = streamStaticTextAsSSE(pw, UserVisibleStreamFailure(fmt.Errorf("streaming with tool calls is not supported in this runtime")))
				if _, werr := fmt.Fprintf(pw, "data: [DONE]\n\n"); werr != nil {
					_ = pw.CloseWithError(werr)
				}
				return
			}
			out := assistantChunkText(chunk)
			logger.Debug("chat stream plain", "phase", "chunk", "n", chunkIdx, "out_len", len(out), "tool_calls", len(chunk.ToolCalls))
			if err := streamStaticTextAsSSE(pw, out); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}
		if _, err := fmt.Fprintf(pw, "data: [DONE]\n\n"); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()
	return pr, nil
}

func (r *Runtime) ChatStreamWithUserProfile(ctx context.Context, agentID int64, message string, visionParts []VisionPart, userProfile string, historyMsgs []*einoschema.Message, stopCh <-chan struct{}, sessionID, auditUserID, clientType string) (io.ReadCloser, error) {
	ctx = contextWithAgentID(ctx, agentID)
	agent, ok := r.GetAgent(agentID)
	if !ok {
		return nil, fmt.Errorf("agent not found: %d", agentID)
	}
	agent = r.agentWithSkillOverrides(agent)
	if r.chatModel == nil {
		return nil, fmt.Errorf("chat model not configured")
	}

	var systemPrompt string
	if agent.RuntimeProfile != nil {
		systemPrompt = agent.RuntimeProfile.SystemPrompt
		if systemPrompt == "" {
			systemPrompt = buildSystemPrompt(agent.RuntimeProfile)
		}
	}
	if userProfile != "" {
		systemPrompt = userProfile + "\n\n" + systemPrompt
	}

	chatTools, err := r.allToolsForAgent(agent)
	if err != nil {
		return nil, err
	}
	chatTools = r.wrapToolsWithAudit(agent, sessionID, auditUserID, chatTools)

	return r.openChatStream(ctx, agent, systemPrompt, message, historyMsgs, chatTools, clientType, visionParts, sessionID, auditUserID, stopCh)
}

func (r *Runtime) ChatStreamWithMemoryContext(ctx context.Context, agentID int64, message string, visionParts []VisionPart, memContext string, historyMsgs []*einoschema.Message, stopCh <-chan struct{}, sessionID, auditUserID, clientType string) (io.ReadCloser, error) {
	ctx = contextWithAgentID(ctx, agentID)
	agent, ok := r.GetAgent(agentID)
	if !ok {
		return nil, fmt.Errorf("agent not found: %d", agentID)
	}
	agent = r.agentWithSkillOverrides(agent)
	if r.chatModel == nil {
		return nil, fmt.Errorf("chat model not configured")
	}

	var systemPrompt string
	if agent.RuntimeProfile != nil {
		systemPrompt = r.cachedSystemPrompt(agentID, agent.RuntimeProfile)
	}
	if memContext != "" {
		systemPrompt = r.mergeKBContext(ctx, agentID, message, memContext) + "\n\n" + systemPrompt
	} else if kbOnly := r.mergeKBContext(ctx, agentID, message, ""); kbOnly != "" {
		systemPrompt = kbOnly + "\n\n" + systemPrompt
	}

	chatTools, err := r.allToolsForAgent(agent)
	if err != nil {
		return nil, err
	}
	chatTools = r.wrapToolsWithAudit(agent, sessionID, auditUserID, chatTools)

	return r.openChatStream(ctx, agent, systemPrompt, message, historyMsgs, chatTools, clientType, visionParts, sessionID, auditUserID, stopCh)
}

type VisionPart struct {
	Base64 string
	Mime   string
}

func buildStreamChatMessages(instruction string, history []*einoschema.Message, userText string, visionParts []VisionPart) []*einoschema.Message {
	var msgs []*einoschema.Message
	if strings.TrimSpace(instruction) != "" {
		msgs = append(msgs, &einoschema.Message{
			Role:    einoschema.System,
			Content: instruction,
		})
	}
	if len(history) > 0 {
		msgs = append(msgs, history...)
	}
	msgs = append(msgs, buildStreamUserMessage(userText, visionParts))
	return msgs
}

func buildStreamUserMessage(userText string, visionParts []VisionPart) *einoschema.Message {
	if len(visionParts) == 0 {
		return &einoschema.Message{
			Role:    einoschema.User,
			Content: userText,
		}
	}
	var contentParts []einoschema.MessageInputPart
	if t := strings.TrimSpace(userText); t != "" {
		contentParts = append(contentParts, einoschema.MessageInputPart{Type: einoschema.ChatMessagePartTypeText, Text: userText})
	}
	for _, vp := range visionParts {
		b64 := strings.TrimSpace(vp.Base64)
		if b64 == "" {
			continue
		}
		mime := strings.TrimSpace(vp.Mime)
		if mime == "" {
			mime = "image/png"
		}
		b64Copy := b64
		contentParts = append(contentParts, einoschema.MessageInputPart{
			Type: einoschema.ChatMessagePartTypeImageURL,
			Image: &einoschema.MessageInputImage{
				MessagePartCommon: einoschema.MessagePartCommon{
					Base64Data: &b64Copy,
					MIMEType:   mime,
				},
			},
		})
	}
	return &einoschema.Message{
		Role:                  einoschema.User,
		UserInputMultiContent: contentParts,
	}
}

func (r *Runtime) streamChatModelTokensToSSE(ctx context.Context, w io.Writer, msgs []*einoschema.Message) error {
	sr, err := r.chatModel.Stream(ctx, msgs)
	if err != nil {
		_ = streamStaticTextAsSSE(w, UserVisibleStreamFailure(err))
		return err
	}
	defer sr.Close()
	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			_ = streamStaticTextAsSSE(w, UserVisibleStreamFailure(err))
			return err
		}
		if chunk == nil {
			continue
		}
		if len(chunk.ToolCalls) > 0 && strings.TrimSpace(chunk.Content) == "" && strings.TrimSpace(chunk.ReasoningContent) == "" {
			_ = streamStaticTextAsSSE(w, UserVisibleStreamFailure(fmt.Errorf("streaming with tool calls is not supported in this runtime")))
			return fmt.Errorf("streaming with tool calls is not supported in this runtime")
		}
		out := assistantChunkText(chunk)
		if err := streamStaticTextAsSSE(w, out); err != nil {
			return err
		}
	}
}

type sseWriterFlusher interface {
	Flush() error
}

func (r *Runtime) streamChatModelTokensToSSEAccumulate(ctx context.Context, w io.Writer, msgs []*einoschema.Message) (string, error) {
	sr, err := r.chatModel.Stream(ctx, msgs)
	if err != nil {
		_ = streamStaticTextAsSSE(w, UserVisibleStreamFailure(err))
		return "", err
	}
	defer sr.Close()
	var acc strings.Builder
	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			return acc.String(), nil
		}
		if err != nil {
			_ = streamStaticTextAsSSE(w, UserVisibleStreamFailure(err))
			return acc.String(), err
		}
		if chunk == nil {
			continue
		}
		if len(chunk.ToolCalls) > 0 && strings.TrimSpace(chunk.Content) == "" && strings.TrimSpace(chunk.ReasoningContent) == "" {
			_ = streamStaticTextAsSSE(w, UserVisibleStreamFailure(fmt.Errorf("streaming with tool calls is not supported in this runtime")))
			return acc.String(), fmt.Errorf("streaming with tool calls is not supported in this runtime")
		}
		out := assistantChunkText(chunk)
		if out == "" {
			continue
		}
		acc.WriteString(out)
		if err := writeSSEJSON(w, out); err != nil {
			return acc.String(), err
		}
		if f, ok := w.(sseWriterFlusher); ok {
			_ = f.Flush()
		}
	}
}
