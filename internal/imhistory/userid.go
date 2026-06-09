package imhistory

import (
	"fmt"
	"strings"
)

const userIDPrefix = "im:"

// FormatIMUserID builds the chat_sessions.user_id for an IM peer (e.g. im:lark:ou_xxx).
func FormatIMUserID(channel, externalID string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	externalID = strings.TrimSpace(externalID)
	return fmt.Sprintf("%s%s:%s", userIDPrefix, channel, externalID)
}

// ParseIMUserID splits im:channel:externalID.
func ParseIMUserID(imUserID string) (channel, external string, ok bool) {
	imUserID = strings.TrimSpace(imUserID)
	if !strings.HasPrefix(imUserID, userIDPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(imUserID, userIDPrefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// UserIDPrefixForChannel returns the LIKE prefix for listing sessions (e.g. im:lark:).
func UserIDPrefixForChannel(channel string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" || channel == "all" {
		return userIDPrefix
	}
	return FormatIMUserID(channel, "")
}

// IsIMUserID reports whether user_id belongs to an IM bot conversation.
func IsIMUserID(userID string) bool {
	_, _, ok := ParseIMUserID(userID)
	return ok
}
