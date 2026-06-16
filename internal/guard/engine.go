package guard

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fisk086/aiops/internal/model"
)

type RuleStore interface {
	GetActiveRulesByAgent(ctx context.Context, agentID int64) ([]*model.GuardrailRule, error)
	GetRuleByID(ctx context.Context, id int64) (*model.GuardrailRule, error)
	CreateLog(ctx context.Context, log *model.GuardrailLog) error
	IncrementRuleHit(ctx context.Context, ruleID int64) error
}

type Result struct {
	Triggered bool
	RuleName  string
	RuleType  string
	Action    model.GuardrailAction
	MatchInfo map[string]any
	Blocked   bool
	Masked    string
}

type Engine struct {
	mu sync.RWMutex
	store   RuleStore
	rules   map[int64][]*compiledRule
	injectDetector *injectionDetector
	piiDetector    *piiDetector
	contentMod     *contentModerator
	topicFilter    *topicFilter
}

type compiledRule struct {
	rule   *model.GuardrailRule
	checks []func(text string) (bool, map[string]any)
}

func NewEngine(store RuleStore) *Engine {
	return &Engine{
		store:          store,
		rules:          make(map[int64][]*compiledRule),
		injectDetector: newInjectionDetector(),
		piiDetector:    newPiiDetector(),
		contentMod:     newContentModerator(),
		topicFilter:    newTopicFilter(),
	}
}

func (e *Engine) CheckInput(ctx context.Context, text string, agentID int64, userID int64, sessionID string) *Result {
	return e.check(ctx, text, agentID, "input", userID, sessionID)
}

func (e *Engine) CheckOutput(ctx context.Context, text string, agentID int64, userID int64, sessionID string) *Result {
	return e.check(ctx, text, agentID, "output", userID, sessionID)
}

func (e *Engine) check(ctx context.Context, text string, agentID int64, scope string, userID int64, sessionID string) *Result {
	if text == "" {
		return nil
	}

	rules, err := e.store.GetActiveRulesByAgent(ctx, agentID)
	if err != nil || len(rules) == 0 {
		return nil
	}

	for _, rule := range rules {
		if rule.Scope != "both" && string(rule.Scope) != scope {
			continue
		}
		if !rule.IsActive {
			continue
		}

		cr := e.compileRule(rule)
		triggered, info := cr.evaluate(text)
		if !triggered {
			continue
		}

		e.store.IncrementRuleHit(ctx, rule.ID)

		action := rule.Action
		now := time.Now()
		logEntry := &model.GuardrailLog{
			RuleID:    &rule.ID,
			RuleName:  rule.Name,
			RuleType:  rule.RuleType,
			AgentID:   agentID,
			Scope:     scope,
			Action:    action,
			Severity:  rule.Severity,
			UserID:    userID,
			SessionID: sessionID,
			Input:     text,
			MatchInfo: info,
			Blocked:   action == model.GuardrailActionBlock,
			CreatedAt: now,
		}

		masked := text
		if action == model.GuardrailActionMask {
			masked = e.applyMasking(text, info)
			logEntry.Output = masked
		}

		e.store.CreateLog(ctx, logEntry)

		return &Result{
			Triggered: true,
			RuleName:  rule.Name,
			RuleType:  rule.RuleType,
			Action:    action,
			MatchInfo: info,
			Blocked:   action == model.GuardrailActionBlock || action == model.GuardrailActionRedirect,
			Masked:    masked,
		}
	}

	return nil
}

func (e *Engine) TestRule(rule *model.GuardrailRule, text string) *Result {
	if text == "" {
		return nil
	}
	if !rule.IsActive {
		return nil
	}

	cr := e.compileRule(rule)
	triggered, info := cr.evaluate(text)
	if !triggered {
		return nil
	}

	action := rule.Action
	return &Result{
		Triggered: true,
		RuleName:  rule.Name,
		RuleType:  rule.RuleType,
		Action:    action,
		MatchInfo: info,
		Blocked:   action == model.GuardrailActionBlock || action == model.GuardrailActionRedirect,
	}
}

func (e *Engine) compileRule(rule *model.GuardrailRule) *compiledRule {
	cr := &compiledRule{rule: rule}

	switch rule.RuleType {
	case "prompt_injection":
		cr.checks = append(cr.checks, e.injectDetector.check)
	case "pii_detection":
		cr.checks = append(cr.checks, func(text string) (bool, map[string]any) {
			return e.piiDetector.check(text, rule.Config)
		})
	case "content_moderation":
		cr.checks = append(cr.checks, func(text string) (bool, map[string]any) {
			return e.contentMod.check(text, rule.Config)
		})
	case "topic_guardrail":
		cr.checks = append(cr.checks, func(text string) (bool, map[string]any) {
			return e.topicFilter.check(text, rule.Config)
		})
	case "keyword_filter":
		if keywords, ok := rule.Config["keywords"].([]interface{}); ok {
			ks := make([]string, 0, len(keywords))
			for _, k := range keywords {
				if s, ok := k.(string); ok {
					ks = append(ks, s)
				}
			}
			cr.checks = append(cr.checks, func(text string) (bool, map[string]any) {
				return keywordCheck(text, ks)
			})
		}
	case "regex_match":
		if pattern, ok := rule.Config["pattern"].(string); ok {
			cr.checks = append(cr.checks, func(text string) (bool, map[string]any) {
				return regexCheck(text, pattern)
			})
		}
	}

	return cr
}

func (cr *compiledRule) evaluate(text string) (bool, map[string]any) {
	allInfo := make(map[string]any)
	for _, check := range cr.checks {
		triggered, info := check(text)
		if triggered {
			for k, v := range info {
				allInfo[k] = v
			}
			return true, allInfo
		}
	}
	return false, nil
}

func (e *Engine) applyMasking(text string, info map[string]any) string {
	masked := text

	if matches, ok := info["pii_matches"].([]string); ok {
		for _, m := range matches {
			masked = strings.ReplaceAll(masked, m, strings.Repeat("*", len(m)))
		}
	}

	if positions, ok := info["positions"].([]map[string]int); ok {
		for _, pos := range positions {
			start := pos["start"]
			end := pos["end"]
			if start >= 0 && end <= len(masked) && start < end {
				masked = masked[:start] + strings.Repeat("*", end-start) + masked[end:]
			}
		}
	}

	return masked
}

func keywordCheck(text string, keywords []string) (bool, map[string]any) {
	lower := strings.ToLower(text)
	var found []string
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			found = append(found, kw)
		}
	}
	if len(found) > 0 {
		return true, map[string]any{"matched_keywords": found}
	}
	return false, nil
}

func regexCheck(text string, pattern string) (bool, map[string]any) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, nil
	}
	matches := re.FindAllString(text, -1)
	if len(matches) > 0 {
		return true, map[string]any{"matched_patterns": matches}
	}
	return false, nil
}
