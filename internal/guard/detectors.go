package guard

import (
	"regexp"
	"strings"
)

type injectionDetector struct {
	patterns []*regexp.Regexp
}

func newInjectionDetector() *injectionDetector {
	rawPatterns := []string{
		`(?i)(?:(?:ignore|disregard|forget|override|bypass)\s+(?:all\s+)?(?:previous|above|below|the\s+above|instructions|prompts?|directions?|commands?|rules?|system\s+prompt|context))`,
		`(?i)(?:you\s+are\s+(?:now|not\s+(?:required\s+to|bound\s+by|following?))|act\s+as\s+(?:if|though))`,
		`(?i)(?:new\s+(?:instructions?|prompts?|directions?|commands?|rules?)\s*[:：])`,
		`(?i)(?:do\s+(?:not\s+)?(?:follow|obey|listen|heed|comply))`,
		`(?i)(?:print\s+(?:the\s+)?(?:system\s+)?(?:prompt|instructions?|context|message)\b)`,
		`(?i)(?:reveal|show|display|output|leak|dump|expose)\s+(?:your|the|this)\s+(?:system\s+)?(?:prompt|instructions?|prompt|context|configuration)`,
		`(?i)(?:Ignore\s+all\s+(?:previous|prior))\s+(?:instructions|directives|input|messages)`,
		`(?i)(?:you\s+(?:have\s+)?(?:no\s+(?:longer|need\s+to)|don'?t\s+(?:need\s+to|have\s+to))\s+(?:follow|obey))`,
		`(?i)(?:role\s*play|roleplay)\s+as`,
		`(?i)(?:from\s+now\s+on|starting\s+now)\s*[,:]?\s*(?:you\s+are|you'll\s+be|you\s+will\s+be)`,
		`(?i)(?:simulate|pretend|imagine)\s+(?:that\s+)?(?:you\s+are|you're|you\s+were)`,
		`(?i)(?:DAN|do\s+anything\s+now|jailbreak|jail\s*break|system\s+compromised)`,
		`(?i)(?:I\s+(?:hack|hacked|breach|breached|crack|cracked|compromised)\s+(?:you|the\s+system))`,
		`(?i)(?:your\s+(?:rules?|instructions?|prompts?|guidelines?|protocols?)\s+(?:are\s+)?(?:wrong|bad|stupid|outdated|invalid))`,
	}

	compiled := make([]*regexp.Regexp, 0, len(rawPatterns))
	for _, p := range rawPatterns {
		re, err := regexp.Compile(p)
		if err == nil {
			compiled = append(compiled, re)
		}
	}
	return &injectionDetector{patterns: compiled}
}

func (d *injectionDetector) check(text string) (bool, map[string]any) {
	for _, re := range d.patterns {
		match := re.FindString(text)
		if match != "" {
			return true, map[string]any{
				"matched_pattern": match,
				"detail":          "检测到可能的 Prompt 注入攻击",
			}
		}
	}
	return false, nil
}

type piiDetector struct {
	phoneCN    *regexp.Regexp
	idCardCN   *regexp.Regexp
	email      *regexp.Regexp
	ipAddr     *regexp.Regexp
	creditCard *regexp.Regexp
	passport   *regexp.Regexp
}

func newPiiDetector() *piiDetector {
	return &piiDetector{
		phoneCN:    regexp.MustCompile(`1[3-9]\d{9}`),
		idCardCN:   regexp.MustCompile(`[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]`),
		email:      regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
		ipAddr:     regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		creditCard: regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`),
		passport:   regexp.MustCompile(`[A-Za-z]\d{8}`),
	}
}

func (d *piiDetector) check(text string, config map[string]any) (bool, map[string]any) {
	types := getConfigStringSlice(config, "pii_types", []string{"phone", "id_card", "email", "ip", "credit_card", "passport"})
	matches := make(map[string][]string)
	totalMatches := 0

	for _, t := range types {
		var re *regexp.Regexp
		switch t {
		case "phone":
			re = d.phoneCN
		case "id_card":
			re = d.idCardCN
		case "email":
			re = d.email
		case "ip":
			re = d.ipAddr
		case "credit_card":
			re = d.creditCard
		case "passport":
			re = d.passport
		}
		if re != nil {
			found := re.FindAllString(text, -1)
			if len(found) > 0 {
				matches[t] = found
				totalMatches += len(found)
			}
		}
	}

	if totalMatches > 0 {
		return true, map[string]any{
			"pii_matches": matches,
			"total_count": totalMatches,
			"detail":      "检测到敏感个人信息",
		}
	}
	return false, nil
}

type contentModerator struct {
	toxicPatterns []*regexp.Regexp
}

func newContentModerator() *contentModerator {
	patterns := []string{
		`(?i)\b(hate|racis|sexist|discriminat|offensive|harass)\w*\b`,
		`(?i)\b(kill\s+(yourself|myself|everyone)|suicide|self-harm|self_harm)\b`,
		`(?i)\b(child\s*(?:porn|abuse|exploit)|underage)\b`,
		`(?i)\b(terroris|extremis|radical)\w*\b`,
		`(?i)\b(violence|violent|brutal|torture|abuse)\b`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err == nil {
			compiled = append(compiled, re)
		}
	}
	return &contentModerator{toxicPatterns: compiled}
}

func (m *contentModerator) check(text string, config map[string]any) (bool, map[string]any) {
	threshold := getConfigFloat(config, "threshold", 0.5)

	var found []string
	for _, re := range m.toxicPatterns {
		match := re.FindString(text)
		if match != "" {
			found = append(found, match)
		}
	}

	if len(found) > 0 && threshold <= 1.0 {
		return true, map[string]any{
			"matched_terms": found,
			"risk_score":    float64(len(found)) / float64(len(m.toxicPatterns)),
			"detail":        "检测到不合规内容",
		}
	}
	return false, nil
}

type topicFilter struct{}

func newTopicFilter() *topicFilter {
	return &topicFilter{}
}

func (f *topicFilter) check(text string, config map[string]any) (bool, map[string]any) {
	mode := getConfigString(config, "mode", "blocklist")
	topics := getConfigStringSlice(config, "topics", nil)
	if len(topics) == 0 {
		return false, nil
	}

	lower := strings.ToLower(text)

	if mode == "allowlist" {
		for _, topic := range topics {
			if strings.Contains(lower, strings.ToLower(topic)) {
				return false, nil
			}
		}
		return true, map[string]any{
			"detail":       "内容不在允许主题列表中",
			"allow_topics": topics,
		}
	}

	// blocklist mode
	for _, topic := range topics {
		if strings.Contains(lower, strings.ToLower(topic)) {
			return true, map[string]any{
				"matched_topic": topic,
				"detail":        "内容涉及禁止主题",
			}
		}
	}
	return false, nil
}

func getConfigString(config map[string]any, key string, def string) string {
	if config == nil {
		return def
	}
	if v, ok := config[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func getConfigStringSlice(config map[string]any, key string, def []string) []string {
	if config == nil {
		return def
	}
	if v, ok := config[key]; ok {
		if items, ok := v.([]interface{}); ok {
			res := make([]string, 0, len(items))
			for _, item := range items {
				if s, ok := item.(string); ok {
					res = append(res, s)
				}
			}
			if len(res) > 0 {
				return res
			}
		}
	}
	return def
}

func getConfigFloat(config map[string]any, key string, def float64) float64 {
	if config == nil {
		return def
	}
	if v, ok := config[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		}
	}
	return def
}
