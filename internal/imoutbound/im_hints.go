package imoutbound

import (
	"fmt"
	"strings"
)

// IMAgentInstructionSuffix is appended to the agent system instruction during IM bot turns.
func IMAgentInstructionSuffix() string {
	return `

## IM 通道（飞书/钉钉）— 必须遵守
当前对话在即时通讯 App 中，**不是** Web 网页聊天。
- 用户**无法**打开任何 HTTP/HTTPS 链接或 /api/v1/chat/files/ 地址；**禁止**在回复里写下载链接、URL、Markdown 链接。
- 需要发文件/图片：**必须调用** builtin_im_save_file（filename + content，content 可为 UTF-8 正文或 base64）或 builtin_terminal；机器人会**单独**发 file/image 附件消息。
- **位图图片**（.png/.jpg 等）：content 必须是**真实图片字节**（如 builtin_browser 截图、terminal 生成的文件）；**禁止**编造 1×1 或极小的占位 base64——那不是照片，客户端会显示空白图。
- **无法 AI 凭空画图**：用户要「生成小猫图片」等时，不要调用 builtin_im_save_file 造假图；应说明限制，或用 browser 截图真实网页。
- 最终文字回复**只能**一句「已生成 xxx，请查收附件」；不要把文件正文、base64、数字列表贴在回复里（系统会从工具结果发附件，不是从 URL）。`
}

// IMMemoryContextHint is injected as memory context on IM bot turns (Lark/DingTalk/QQ/Telegram).
func IMMemoryContextHint() string {
	return `（系统提示）当前为即时通讯对话（与 Web 网页不同）。
- 文件/图片由机器人**单独发 file/image 附件消息**，不是链接，不是写在文字里。
- 必须用 builtin_im_save_file（filename + content，content 可写 UTF-8 或 base64）或 builtin_terminal 生成文件。
- **禁止**在回复里写任何 URL、/api/v1/chat/ 路径、下载链接；最终只写一句话，如「已生成 xxx.txt，请查收附件」。`
}

// BuildFileStagingRetryPrompt asks the agent to actually invoke IM file tools after a hollow marker reply.
func BuildFileStagingRetryPrompt(userRequest string, markerFiles []string) string {
	if LooksLikeImageGenerationRequest(userRequest) {
		return buildImageGenerationRetryPrompt(userRequest)
	}
	names := strings.Join(markerFiles, ", ")
	if names == "" {
		names = "(见用户请求)"
	}
	return fmt.Sprintf(`[系统纠正] 上一轮你没有正确交付 IM 附件。IM 用户打不开任何 URL，只能收到单独的 file/image 消息。

用户原始请求：%s
文件名：%s

现在你必须：
1. 调用 builtin_im_save_file（filename + content，content 放完整 UTF-8 或 base64）或 builtin_terminal 生成真实文件；
2. 最终回复**只能有一句话**，例如「已生成 %s，请查收附件」——禁止 URL、禁止下载链接、禁止 [[lark_file:]]、禁止在正文贴文件内容。`, userRequest, names, names)
}

// BuildIMRetryPrompt picks the best retry instruction for the user request.
func BuildIMRetryPrompt(userRequest string, markerFiles []string) string {
	return BuildFileStagingRetryPrompt(userRequest, markerFiles)
}

func buildImageGenerationRetryPrompt(userRequest string) string {
	return fmt.Sprintf(`[系统纠正] 用户在 IM 里要一张位图/照片，不是文本文件。

用户请求：%s

重要限制：
- **禁止**用 builtin_im_save_file 编造 1×1 或极小的占位 PNG/JPEG base64（系统会拒绝，用户会看到空白图）。
- 你无法凭空 AI 画出小猫/插画；不要再次尝试假 base64。

请二选一：
1. 若能用 builtin_browser 打开网页并 screenshot 得到真实 PNG，再交付该文件；
2. 否则直接回复一句话说明：「暂时无法生成图片，你可以发一张图让我处理，或让我截图指定网页。」

禁止 URL、禁止下载链接、禁止在正文贴 base64。`, userRequest)
}

// LooksLikeImageGenerationRequest detects requests expecting a generated/drawn image.
func LooksLikeImageGenerationRequest(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	hasImage := strings.Contains(text, "图片") || strings.Contains(text, "照片") ||
		strings.Contains(text, "png") || strings.Contains(text, "jpg") || strings.Contains(text, "jpeg")
	if !hasImage {
		return false
	}
	for _, kw := range []string{"生成", "画", "绘制", "做一张", "来一张", "弄一张", "给我一张"} {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return strings.Contains(text, "发给我") || strings.Contains(text, "发我")
}

// ImageGenerationUnavailableUserText is the canonical IM reply when the user asks for AI-generated images.
func ImageGenerationUnavailableUserText(channel string) string {
	app := ChannelUserLabel(channel)
	return fmt.Sprintf("抱歉，当前智能体不支持 AI 生成图片，无法在%s里凭空画出照片或插画。\n\n", app) +
		"原因：未接入图像生成服务；对话模型只能输出文字，或通过工具保存文件、浏览器截图，不能凭空绘图。\n\n" +
		"你可以：直接发一张图片让我处理；或告诉我网页地址，我用浏览器打开后截图发给你。"
}

// RetryFailureUserText returns a user-visible reply when IM attachment retry failed.
func RetryFailureUserText(userRequest, channel string, retryErr error) string {
	if retryErr == nil {
		return ""
	}
	if IsPlaceholderImageToolError(retryErr) || LooksLikeImageGenerationRequest(userRequest) {
		return ImageGenerationUnavailableUserText(channel)
	}
	return "抱歉，附件未能生成。请再试一次，或换一种描述方式。"
}

// LooksLikeFileDeliveryRequest is a coarse check for user messages expecting a file attachment.
func LooksLikeFileDeliveryRequest(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	for _, kw := range []string{"文件", "附件", "发我", "发给我", "下载", "导出", "保存", "random", ".txt", ".bin", ".csv", ".pdf"} {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

// IMSkillUsageHints returns tool hints for IM sessions (no web download URLs).
func IMSkillUsageHints(hasTerminal bool) string {
	parts := []string{
		"- **IM Save File**: Use `builtin_im_save_file` with filename + content (UTF-8 text or base64). The bot sends a separate file/image message automatically. **Never** put HTTP URLs or /api/v1/chat/ links in your reply.",
		"- **Raster images** (.png/.jpg): content must be **real image bytes** from terminal/browser screenshot — **never** invent tiny placeholder base64 (users will see a blank image).",
	}
	if hasTerminal {
		parts = append(parts, "- **Terminal (IM)**: Use `builtin_terminal` to run shell commands. New/updated files in the working directory (max 1MB) are sent as **separate IM file messages**. Reply with brief text only — never URLs.")
	}
	return strings.Join(parts, "\n")
}
