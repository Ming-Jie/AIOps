package imoutbound

import (
	"regexp"
	"strings"
)

var (
	imFileMarkerRe = regexp.MustCompile(`\[\[(?:lark_file|dingtalk_file|im_file):([^\]]+)\]\]`)
	dataURIPrefix  = regexp.MustCompile(`data:image/[a-z+]+;base64,[A-Za-z0-9+/=]+`)
)

// StripDataURIs removes bare data:image URIs (e.g. from browser screenshots) so they are
// not sent as garbled text to IM channels.  Cleans up to one URI per line.
func StripDataURIs(s string) string {
	return strings.TrimSpace(dataURIPrefix.ReplaceAllString(s, ""))
}

// ParseFileMarkers splits agent text into visible reply and attachment filenames (basename only).
func ParseFileMarkers(text string) (clean string, files []string) {
	seen := make(map[string]struct{})
	matches := imFileMarkerRe.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		name, err := SanitizeFileName(m[1])
		if err != nil {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		files = append(files, name)
	}
	clean = strings.TrimSpace(imFileMarkerRe.ReplaceAllString(text, ""))
	return clean, files
}

// StripFileMarkers removes all [[im_file:...]] / lark / dingtalk markers from text.
func StripFileMarkers(text string) string {
	return strings.TrimSpace(imFileMarkerRe.ReplaceAllString(text, ""))
}

// ContentForSummary returns text suitable for coordinator prompts: markers become short attachment notes.
func ContentForSummary(text string) string {
	clean, files := ParseFileMarkers(text)
	for _, f := range files {
		clean += "\n[附件: " + f + "]"
	}
	return strings.TrimSpace(clean)
}
