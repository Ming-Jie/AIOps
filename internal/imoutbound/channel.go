package imoutbound

import "strings"

// ChannelUserLabel returns a user-facing app name for IM channels (lark, dingtalk, qq, telegram, …).
func ChannelUserLabel(channel string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "lark", "feishu":
		return "飞书"
	case "dingtalk":
		return "钉钉"
	case "qq":
		return "QQ"
	case "telegram":
		return "Telegram"
	case "wechat", "wecom", "wework":
		return "企业微信"
	default:
		return "即时通讯"
	}
}
