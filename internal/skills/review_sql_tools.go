package skills

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const (
	toolCodeReview = "builtin_code_review"
	toolSQLExplain = "builtin_sql_explain"

	reviewScanMaxBytes = 768 << 10
)

var codeReviewPatterns = []namedPattern{
	{Type: "unchecked_error", Severity: "medium", Message: "Error value appears to be ignored", Re: regexp.MustCompile(`(?i)(_,\s*_\s*:=|=\s*[^,\n]+,\s*_|catch\s*\([^)]*\)\s*\{\s*\}|except\s*:\s*pass)`)},
	{Type: "panic_or_exit", Severity: "medium", Message: "Panic/fatal/exit in application path can crash the process", Re: regexp.MustCompile(`(?i)(panic\s*\(|log\.Fatal|os\.Exit\s*\(|process\.exit\s*\()`)},
	{Type: "sql_injection", Severity: "high", Message: "SQL appears to be built with string formatting or concatenation", Re: regexp.MustCompile(`(?i)(select|insert|update|delete|where).*(fmt\.Sprintf|sprintf|format\s*\(|\$\{|\+\s*(req|request|params|body|query|input|user))`)},
	{Type: "sql_injection", Severity: "high", Message: "SQL appears to be built with string formatting or interpolation", Re: regexp.MustCompile(`(?i)(fmt\.Sprintf|sprintf|format\s*\(|\$\{).*(select|insert|update|delete|where)`)},
	{Type: "command_injection", Severity: "critical", Message: "Shell command execution may include dynamic input", Re: regexp.MustCompile(`(?i)(exec\.Command|child_process\.exec|subprocess\.(run|popen|call)|os\.system|shell_exec|Runtime\.getRuntime\(\)\.exec).*(req\.|request|params|body|query|input|args|\+)`)},
	{Type: "path_traversal", Severity: "high", Message: "File path operation may use untrusted input", Re: regexp.MustCompile(`(?i)(readFile|writeFile|os\.Open|open\s*\(|send_file|filepath\.Join|path\.join).*(req\.|request|params|body|query|input|\.\./|\.\.\\)`)},
	{Type: "xss", Severity: "high", Message: "Raw HTML sink detected; ensure input is escaped or sanitized", Re: regexp.MustCompile(`(?i)(innerHTML\s*=|dangerouslySetInnerHTML|document\.write\s*\(|v-html=)`)},
	{Type: "insecure_tls", Severity: "high", Message: "TLS certificate verification appears disabled", Re: regexp.MustCompile(`(?i)(InsecureSkipVerify\s*:\s*true|verify\s*=\s*false|rejectUnauthorized\s*:\s*false)`)},
	{Type: "plaintext_http", Severity: "medium", Message: "Plain HTTP endpoint detected in code", Re: regexp.MustCompile(`http://[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=%-]+`)},
	{Type: "weak_crypto", Severity: "medium", Message: "Weak cryptographic hash or cipher detected", Re: regexp.MustCompile(`(?i)\b(md5|sha1|des|rc4)\b`)},
	{Type: "auth_bypass", Severity: "high", Message: "Authentication or authorization bypass marker detected", Re: regexp.MustCompile(`(?i)(skipAuth|disableAuth|auth\s*=\s*false|TODO.*auth|FIXME.*auth)`)},
}

var codeReviewSecurityTypes = []string{
	"sql_injection",
	"command_injection",
	"path_traversal",
	"xss",
	"insecure_tls",
	"plaintext_http",
	"weak_crypto",
	"auth_bypass",
}

func clampReviewText(s string) string {
	if len(s) <= reviewScanMaxBytes {
		return s
	}
	return s[:reviewScanMaxBytes]
}

func severityValue(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func filterFindingsBySeverity(findings []regexFinding, threshold string) []regexFinding {
	min := severityValue(threshold)
	if min == 0 {
		return findings
	}
	out := findings[:0]
	for _, f := range findings {
		if severityValue(f.Severity) >= min {
			out = append(out, f)
		}
	}
	return out
}

func expandCodeReviewFocus(only map[string]struct{}) map[string]struct{} {
	if len(only) == 0 {
		return nil
	}
	if _, ok := only["all"]; ok {
		return nil
	}
	if _, ok := only["security"]; !ok {
		return only
	}
	out := make(map[string]struct{}, len(only)+len(codeReviewSecurityTypes)+len(secretPatterns))
	for k := range only {
		if k != "security" {
			out[k] = struct{}{}
		}
	}
	for _, k := range codeReviewSecurityTypes {
		out[k] = struct{}{}
	}
	for _, p := range secretPatterns {
		out["secret_"+p.Type] = struct{}{}
	}
	return out
}

func summarizeFindings(findings []regexFinding) string {
	counts := map[string]int{
		"critical": 0,
		"high":     0,
		"medium":   0,
		"low":      0,
	}
	for _, f := range findings {
		counts[strings.ToLower(f.Severity)]++
	}
	return fmt.Sprintf("critical=%d high=%d medium=%d low=%d", counts["critical"], counts["high"], counts["medium"], counts["low"])
}

func formatReviewFindings(title string, findings []regexFinding, notes []string) string {
	sort.SliceStable(findings, func(i, j int) bool {
		if severityValue(findings[i].Severity) == severityValue(findings[j].Severity) {
			return findings[i].Line < findings[j].Line
		}
		return severityValue(findings[i].Severity) > severityValue(findings[j].Severity)
	})

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	if len(findings) == 0 {
		b.WriteString("No high-confidence issues detected by static heuristics.\n")
	} else {
		b.WriteString("Summary: ")
		b.WriteString(summarizeFindings(findings))
		b.WriteString("\n\nFindings:\n")
		for _, f := range findings {
			b.WriteString(fmt.Sprintf("- line %d [%s/%s]: %s", f.Line, f.Severity, f.Type, f.Message))
			if f.Snippet != "" {
				b.WriteString(fmt.Sprintf("\n  snippet: %s", f.Snippet))
			}
			b.WriteString("\n")
		}
	}
	if len(notes) > 0 {
		b.WriteString("\nNext steps:\n")
		for _, n := range notes {
			b.WriteString("- ")
			b.WriteString(n)
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func execBuiltinCodeReview(_ context.Context, in map[string]any) (string, error) {
	content := strArg(in, "diff", "code", "content", "patch", "input")
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("missing diff/code content")
	}

	content = clampReviewText(content)
	threshold := strArg(in, "severity_threshold", "min_severity")
	findings := scanLines(content, codeReviewPatterns, expandCodeReviewFocus(parseStringListArg(strArg(in, "focus", "checks"))))

	secretFindings := scanLines(content, secretPatterns, nil)
	for i := range secretFindings {
		if secretFindings[i].Line > 0 {
			line := strings.Split(content, "\n")[secretFindings[i].Line-1]
			secretFindings[i].Snippet = maskSecretSnippet(line)
		}
		secretFindings[i].Type = "secret_" + secretFindings[i].Type
	}
	findings = append(findings, secretFindings...)
	findings = filterFindingsBySeverity(findings, threshold)

	notes := []string{
		"Validate these findings against the surrounding code; this tool is a fast static screen, not a full semantic review.",
		"Add or update tests around any changed branch, error path, or security-sensitive behavior.",
	}
	if lang := strings.TrimSpace(strArg(in, "language")); lang != "" {
		notes = append(notes, "Language hint used: "+lang)
	}
	return formatReviewFindings("Code review scan result", findings, notes), nil
}

func NewBuiltinCodeReviewTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolCodeReview,
			Desc:  "Review pasted code or diffs for high-signal correctness, security, reliability, and test risks using server-side static heuristics.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"diff":               {Type: einoschema.String, Desc: "Patch/diff content to review", Required: false},
				"code":               {Type: einoschema.String, Desc: "Source code content to review", Required: false},
				"language":           {Type: einoschema.String, Desc: "Optional language hint", Required: false},
				"focus":              {Type: einoschema.String, Desc: "Optional comma-separated checks, e.g. security, sql_injection, xss", Required: false},
				"severity_threshold": {Type: einoschema.String, Desc: "Optional minimum severity: low, medium, high, critical", Required: false},
			}),
		},
		execBuiltinCodeReview,
	)
}

type sqlPlanFinding struct {
	Severity string
	Type     string
	Message  string
	Evidence string
}

func detectSQLEngine(engine, plan string) string {
	engine = strings.ToLower(strings.TrimSpace(engine))
	if engine == "mysql" || engine == "postgres" || engine == "postgresql" {
		if engine == "postgresql" {
			return "postgres"
		}
		return engine
	}
	lower := strings.ToLower(plan)
	switch {
	case strings.Contains(lower, "seq scan") || strings.Contains(lower, "actual time=") || strings.Contains(lower, "rows removed by filter"):
		return "postgres"
	case strings.Contains(lower, "using filesort") || strings.Contains(lower, "using temporary") || strings.Contains(lower, `"access_type"`) || strings.Contains(lower, "possible_keys"):
		return "mysql"
	default:
		return "generic"
	}
}

func addSQLFinding(findings *[]sqlPlanFinding, severity, typ, message, evidence string) {
	*findings = append(*findings, sqlPlanFinding{
		Severity: severity,
		Type:     typ,
		Message:  message,
		Evidence: shortSnippet(evidence, 180),
	})
}

func firstPlanMatch(re *regexp.Regexp, plan string) string {
	m := re.FindStringSubmatch(plan)
	if len(m) > 1 {
		return m[1]
	}
	if len(m) == 1 {
		return m[0]
	}
	return ""
}

func planNumberMatches(re *regexp.Regexp, plan string) []int {
	matches := re.FindAllStringSubmatch(plan, -1)
	out := make([]int, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		n, err := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
		if err == nil {
			out = append(out, n)
		}
	}
	return out
}

func analyzeSQLPlan(plan, engine, query string) (string, []sqlPlanFinding, []string) {
	engine = detectSQLEngine(engine, plan)
	lower := strings.ToLower(plan)
	var findings []sqlPlanFinding
	var okNotes []string

	if strings.Contains(lower, "seq scan") {
		table := firstPlanMatch(regexp.MustCompile(`(?i)Seq Scan on ([A-Za-z0-9_."-]+)`), plan)
		msg := "Sequential scan detected; add or adjust an index if the filter is selective"
		if table != "" {
			msg += " on " + table
		}
		addSQLFinding(&findings, "high", "seq_scan", msg, firstPlanMatch(regexp.MustCompile(`(?im)^.*Seq Scan.*$`), plan))
	}
	if strings.Contains(lower, "type: all") || regexp.MustCompile(`(?i)"access_type"\s*:\s*"all"`).MatchString(plan) || regexp.MustCompile(`(?im)^\s*\S+\s+\S+\s+ALL\s+`).MatchString(plan) {
		addSQLFinding(&findings, "high", "full_table_scan", "MySQL full table scan detected; check predicates and usable indexes", firstPlanMatch(regexp.MustCompile(`(?im)^.*(\bALL\b|"access_type"\s*:\s*"ALL").*$`), plan))
	}
	if strings.Contains(lower, "using filesort") || regexp.MustCompile(`(?i)"using_filesort"\s*:\s*true`).MatchString(plan) {
		addSQLFinding(&findings, "medium", "filesort", "Filesort detected; consider an index matching ORDER BY / GROUP BY order", firstPlanMatch(regexp.MustCompile(`(?im)^.*(Using filesort|"using_filesort").*$`), plan))
	}
	if strings.Contains(lower, "using temporary") || regexp.MustCompile(`(?i)"using_temporary`).MatchString(plan) {
		addSQLFinding(&findings, "medium", "temporary_table", "Temporary table detected; review grouping, ordering, and join shape", firstPlanMatch(regexp.MustCompile(`(?im)^.*(Using temporary|"using_temporary).*$`), plan))
	}
	if strings.Contains(lower, "key: null") || regexp.MustCompile(`(?i)"key"\s*:\s*null`).MatchString(plan) {
		addSQLFinding(&findings, "high", "missing_index", "No index key is selected for at least one access path", firstPlanMatch(regexp.MustCompile(`(?im)^.*(key:\s*NULL|"key"\s*:\s*null).*$`), plan))
	}
	if strings.Contains(lower, "nested loop") {
		severity := "medium"
		for _, rows := range planNumberMatches(regexp.MustCompile(`(?i)rows[=: ]+([0-9,]+)`), plan) {
			if rows >= 100000 {
				severity = "high"
				break
			}
		}
		addSQLFinding(&findings, severity, "nested_loop", "Nested loop join detected; verify join order and indexes on inner relation", firstPlanMatch(regexp.MustCompile(`(?im)^.*Nested Loop.*$`), plan))
	}
	if strings.Contains(lower, "external merge") || strings.Contains(lower, "disk:") {
		addSQLFinding(&findings, "medium", "disk_sort", "Sort spilled to disk; consider work_mem/memory settings or an order-supporting index", firstPlanMatch(regexp.MustCompile(`(?im)^.*(external merge|Disk:).*$`), plan))
	}
	if removed := planNumberMatches(regexp.MustCompile(`(?i)Rows Removed by Filter:\s*([0-9,]+)`), plan); len(removed) > 0 {
		maxRemoved := 0
		for _, n := range removed {
			if n > maxRemoved {
				maxRemoved = n
			}
		}
		if maxRemoved >= 10000 {
			addSQLFinding(&findings, "medium", "filter_waste", fmt.Sprintf("Many rows are removed after scan (%d); push selectivity into indexes when possible", maxRemoved), firstPlanMatch(regexp.MustCompile(`(?im)^.*Rows Removed by Filter.*$`), plan))
		}
	}
	for _, rows := range planNumberMatches(regexp.MustCompile(`(?i)(?:rows|rows_examined_per_scan)[=: ]+\s*([0-9,]+)`), plan) {
		if rows >= 1000000 {
			addSQLFinding(&findings, "high", "large_row_estimate", fmt.Sprintf("Large row estimate detected (%d rows)", rows), "")
			break
		}
		if rows >= 100000 {
			addSQLFinding(&findings, "medium", "large_row_estimate", fmt.Sprintf("Large row estimate detected (%d rows)", rows), "")
			break
		}
	}

	if strings.Contains(lower, "index scan") || strings.Contains(lower, "index only scan") || strings.Contains(lower, "using index") || regexp.MustCompile(`(?i)"access_type"\s*:\s*"(ref|eq_ref|range|const)"`).MatchString(plan) {
		okNotes = append(okNotes, "Plan shows at least one index-based access path.")
	}
	if strings.Contains(lower, "hash join") {
		okNotes = append(okNotes, "Hash join is present; this is often reasonable for larger unsorted joins.")
	}
	if strings.Contains(strings.ToLower(query), "select *") {
		addSQLFinding(&findings, "low", "wide_select", "Query uses SELECT *; project only needed columns to reduce IO and network cost", "SELECT *")
	}

	next := []string{
		"Compare estimated rows with actual rows when EXPLAIN ANALYZE is available; large gaps usually mean stale stats or missing extended statistics.",
		"Check indexes on join keys, selective WHERE predicates, and ORDER BY / GROUP BY columns in left-to-right order.",
	}
	if engine == "mysql" {
		next = append(next, "For MySQL, inspect `type`, `key`, `rows`, and `Extra`; avoid `ALL`, `Using temporary`, and avoidable `Using filesort` on large tables.")
	} else if engine == "postgres" {
		next = append(next, "For PostgreSQL, run `ANALYZE` after large data changes and review `actual time`, `loops`, and `Rows Removed by Filter`.")
	}
	return engine, findings, append(okNotes, next...)
}

func formatSQLPlanAnalysis(engine string, findings []sqlPlanFinding, notes []string) string {
	sort.SliceStable(findings, func(i, j int) bool {
		if severityValue(findings[i].Severity) == severityValue(findings[j].Severity) {
			return findings[i].Type < findings[j].Type
		}
		return severityValue(findings[i].Severity) > severityValue(findings[j].Severity)
	})

	var b strings.Builder
	b.WriteString("SQL explain analysis\n")
	b.WriteString("Engine: ")
	b.WriteString(engine)
	b.WriteString("\n")
	if len(findings) == 0 {
		b.WriteString("No obvious plan risks detected by static heuristics.\n")
	} else {
		b.WriteString("Summary: ")
		counts := make(map[string]int)
		for _, f := range findings {
			counts[strings.ToLower(f.Severity)]++
		}
		b.WriteString(fmt.Sprintf("critical=%d high=%d medium=%d low=%d", counts["critical"], counts["high"], counts["medium"], counts["low"]))
		b.WriteString("\n\nFindings:\n")
		for _, f := range findings {
			b.WriteString(fmt.Sprintf("- [%s/%s] %s", f.Severity, f.Type, f.Message))
			if f.Evidence != "" {
				b.WriteString("\n  evidence: ")
				b.WriteString(f.Evidence)
			}
			b.WriteString("\n")
		}
	}
	if len(notes) > 0 {
		b.WriteString("\nNext steps:\n")
		for _, n := range notes {
			b.WriteString("- ")
			b.WriteString(n)
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func execBuiltinSQLExplain(_ context.Context, in map[string]any) (string, error) {
	plan := strArg(in, "plan", "explain", "text", "content", "input")
	if strings.TrimSpace(plan) == "" {
		return "", fmt.Errorf("missing explain plan text")
	}
	engine, findings, notes := analyzeSQLPlan(clampReviewText(plan), strArg(in, "engine", "dialect", "database"), strArg(in, "query", "sql"))
	return formatSQLPlanAnalysis(engine, findings, notes), nil
}

func NewBuiltinSQLExplainTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolSQLExplain,
			Desc:  "Analyze pasted SQL EXPLAIN / query plan text for MySQL, PostgreSQL, or generic plans and suggest index/query tuning steps.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"plan":   {Type: einoschema.String, Desc: "Pasted EXPLAIN / EXPLAIN ANALYZE / query plan text", Required: true},
				"engine": {Type: einoschema.String, Desc: "Optional engine hint: mysql, postgres, generic", Required: false},
				"query":  {Type: einoschema.String, Desc: "Optional original SQL query for extra hints", Required: false},
			}),
		},
		execBuiltinSQLExplain,
	)
}
