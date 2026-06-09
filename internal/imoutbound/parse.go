package imoutbound

import (
	"regexp"
	"strings"
)

var imFileMarkerRe = regexp.MustCompile(`\[\[(?:lark_file|dingtalk_file|im_file):([^\]]+)\]\]`)

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
