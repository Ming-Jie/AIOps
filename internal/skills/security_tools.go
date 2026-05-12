package skills

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const (
	toolJWTTool                 = "builtin_jwt_tool"
	toolPasswordStrengthChecker = "builtin_password_strength_checker"
	toolSecretsScanner          = "builtin_secrets_scanner"
	toolSecurityHeadersChecker  = "builtin_security_headers_checker"
	toolAPISecurityChecker      = "builtin_api_security_checker"
	toolLogSecurityAnalyzer     = "builtin_log_security_analyzer"
	toolCryptoTool              = "builtin_crypto_tool"

	securityScanMaxBytes = 512 << 10
)

type regexFinding struct {
	Line     int
	Type     string
	Severity string
	Message  string
	Snippet  string
}

type namedPattern struct {
	Type     string
	Severity string
	Message  string
	Re       *regexp.Regexp
}

func clampTextForScan(s string) string {
	if len(s) <= securityScanMaxBytes {
		return s
	}
	return s[:securityScanMaxBytes]
}

func shortSnippet(s string, max int) string {
	s = strings.TrimSpace(s)
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func parseStringListArg(raw string) map[string]struct{} {
	out := make(map[string]struct{})
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out
	}
	var arr []string
	if strings.HasPrefix(raw, "[") && json.Unmarshal([]byte(raw), &arr) == nil {
		for _, s := range arr {
			if s = strings.TrimSpace(strings.ToLower(s)); s != "" {
				out[s] = struct{}{}
			}
		}
		return out
	}
	for _, part := range strings.Split(raw, ",") {
		if s := strings.TrimSpace(strings.ToLower(part)); s != "" {
			out[s] = struct{}{}
		}
	}
	return out
}

func scanLines(text string, patterns []namedPattern, only map[string]struct{}) []regexFinding {
	text = clampTextForScan(text)
	lines := strings.Split(text, "\n")
	var findings []regexFinding
	for i, line := range lines {
		for _, p := range patterns {
			if len(only) > 0 {
				if _, ok := only[strings.ToLower(p.Type)]; !ok {
					continue
				}
			}
			if !p.Re.MatchString(line) {
				continue
			}
			findings = append(findings, regexFinding{
				Line:     i + 1,
				Type:     p.Type,
				Severity: p.Severity,
				Message:  p.Message,
				Snippet:  shortSnippet(line, 180),
			})
		}
	}
	return findings
}

func formatRegexFindings(title string, findings []regexFinding) string {
	if len(findings) == 0 {
		return title + "\nNo issues detected."
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Line == findings[j].Line {
			return findings[i].Type < findings[j].Type
		}
		return findings[i].Line < findings[j].Line
	})
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	for _, f := range findings {
		b.WriteString(fmt.Sprintf("- line %d [%s/%s]: %s", f.Line, f.Severity, f.Type, f.Message))
		if f.Snippet != "" {
			b.WriteString(fmt.Sprintf("\n  snippet: %s", f.Snippet))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func decodeJWTPart(part string) ([]byte, error) {
	if b, err := base64.RawURLEncoding.DecodeString(part); err == nil {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(part)
}

func encodeJWTPart(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func jwtSigningBytes(headerPart, payloadPart string) []byte {
	return []byte(headerPart + "." + payloadPart)
}

func jwtHMACSignature(alg string, secret string, signing []byte) ([]byte, error) {
	var mac hashWriter
	switch strings.ToUpper(strings.TrimSpace(alg)) {
	case "HS256", "":
		mac = hmac.New(sha256.New, []byte(secret))
	case "HS384":
		mac = hmac.New(sha512.New384, []byte(secret))
	case "HS512":
		mac = hmac.New(sha512.New, []byte(secret))
	default:
		return nil, fmt.Errorf("unsupported HMAC JWT algorithm: %s", alg)
	}
	_, _ = mac.Write(signing)
	return mac.Sum(nil), nil
}

type hashWriter interface {
	io.Writer
	Sum([]byte) []byte
}

func jwtNumericDate(payload map[string]any, key string) (int64, bool) {
	v, ok := payload[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}

func decodeJWT(token string) (map[string]any, map[string]any, []string, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, nil, nil, fmt.Errorf("invalid JWT: expected 3 dot-separated parts, got %d", len(parts))
	}
	headerBytes, err := decodeJWTPart(parts[0])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode JWT header: %w", err)
	}
	payloadBytes, err := decodeJWTPart(parts[1])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}
	var header map[string]any
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid JWT header JSON: %w", err)
	}
	var payload map[string]any
	dec := json.NewDecoder(bytes.NewReader(payloadBytes))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid JWT payload JSON: %w", err)
	}
	now := time.Now().Unix()
	var notes []string
	if exp, ok := jwtNumericDate(payload, "exp"); ok {
		when := time.Unix(exp, 0).Format(time.RFC3339)
		if exp < now {
			notes = append(notes, fmt.Sprintf("expired at %s", when))
		} else {
			notes = append(notes, fmt.Sprintf("expires at %s (%d seconds remaining)", when, exp-now))
		}
	}
	if nbf, ok := jwtNumericDate(payload, "nbf"); ok && nbf > now {
		notes = append(notes, fmt.Sprintf("not valid before %s", time.Unix(nbf, 0).Format(time.RFC3339)))
	}
	if iat, ok := jwtNumericDate(payload, "iat"); ok {
		notes = append(notes, fmt.Sprintf("issued at %s", time.Unix(iat, 0).Format(time.RFC3339)))
	}
	if alg := fmt.Sprint(header["alg"]); strings.EqualFold(alg, "none") {
		notes = append(notes, "warning: alg=none means the token is unsigned")
	}
	return header, payload, notes, nil
}

func execBuiltinJWTTool(_ context.Context, in map[string]any) (string, error) {
	op := strings.ToLower(strArg(in, "operation", "op", "action"))
	if op == "" {
		op = "decode"
	}
	switch op {
	case "decode", "parse", "inspect":
		token := strArg(in, "token", "jwt", "jwt_token")
		if token == "" {
			return "", fmt.Errorf("missing token")
		}
		header, payload, notes, err := decodeJWT(token)
		if err != nil {
			return "", err
		}
		h, _ := json.MarshalIndent(header, "", "  ")
		p, _ := json.MarshalIndent(payload, "", "  ")
		return fmt.Sprintf("JWT header:\n%s\n\nJWT payload:\n%s\n\nNotes:\n- %s", h, p, strings.Join(appendDefault(notes, "no exp/nbf/iat timing claims found"), "\n- ")), nil
	case "verify":
		token := strArg(in, "token", "jwt", "jwt_token")
		secret := strArg(in, "secret", "key")
		if token == "" {
			return "", fmt.Errorf("missing token")
		}
		if secret == "" {
			return "", fmt.Errorf("missing secret for HS256/HS384/HS512 verification")
		}
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			return "", fmt.Errorf("invalid JWT: expected 3 parts")
		}
		header, payload, notes, err := decodeJWT(token)
		if err != nil {
			return "", err
		}
		alg := fmt.Sprint(header["alg"])
		want, err := jwtHMACSignature(alg, secret, jwtSigningBytes(parts[0], parts[1]))
		if err != nil {
			return "", err
		}
		got, err := decodeJWTPart(parts[2])
		if err != nil {
			return "", fmt.Errorf("failed to decode signature: %w", err)
		}
		validSig := hmac.Equal(got, want)
		expired := false
		if exp, ok := jwtNumericDate(payload, "exp"); ok && exp < time.Now().Unix() {
			expired = true
		}
		status := "valid"
		if !validSig || expired {
			status = "invalid"
		}
		return fmt.Sprintf("JWT verification: %s\nSignature valid: %t\nExpired: %t\nNotes:\n- %s", status, validSig, expired, strings.Join(appendDefault(notes, "no exp/nbf/iat timing claims found"), "\n- ")), nil
	case "encode":
		payloadRaw := strArg(in, "payload", "claims")
		secret := strArg(in, "secret", "key")
		alg := strings.ToUpper(strArg(in, "algorithm", "alg"))
		if alg == "" {
			alg = "HS256"
		}
		if payloadRaw == "" {
			return "", fmt.Errorf("missing payload JSON")
		}
		if secret == "" {
			return "", fmt.Errorf("missing secret")
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
			return "", fmt.Errorf("invalid payload JSON: %w", err)
		}
		header := map[string]any{"typ": "JWT", "alg": alg}
		headerJSON, _ := json.Marshal(header)
		payloadJSON, _ := json.Marshal(payload)
		headerPart := encodeJWTPart(headerJSON)
		payloadPart := encodeJWTPart(payloadJSON)
		sig, err := jwtHMACSignature(alg, secret, jwtSigningBytes(headerPart, payloadPart))
		if err != nil {
			return "", err
		}
		return headerPart + "." + payloadPart + "." + encodeJWTPart(sig), nil
	default:
		return "", fmt.Errorf("unknown JWT operation: %s", op)
	}
}

func appendDefault(items []string, fallback string) []string {
	if len(items) > 0 {
		return items
	}
	return []string{fallback}
}

func NewBuiltinJWTTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolJWTTool,
			Desc:  "Decode, inspect, verify, or encode JWT tokens. Supports HS256/HS384/HS512 verification and signing.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: decode, verify, encode", Required: true},
				"token":     {Type: einoschema.String, Desc: "JWT token for decode/verify", Required: false},
				"secret":    {Type: einoschema.String, Desc: "HMAC secret for verify/encode", Required: false},
				"payload":   {Type: einoschema.String, Desc: "JSON object payload for encode", Required: false},
				"algorithm": {Type: einoschema.String, Desc: "JWT algorithm: HS256, HS384, HS512 (default HS256)", Required: false},
			}),
		},
		execBuiltinJWTTool,
	)
}

func execBuiltinPasswordStrengthChecker(_ context.Context, in map[string]any) (string, error) {
	password := strArg(in, "password", "pass", "pwd")
	if password == "" {
		return "", fmt.Errorf("missing password")
	}
	minLen := 8
	if raw := strArg(in, "min_length", "minLength"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			minLen = n
		}
	}
	score, strength, suggestions, entropy := passwordStrength(password, minLen)
	return fmt.Sprintf("Password strength: %d/100 (%s)\nEstimated entropy: %.1f bits\nSuggestions:\n- %s", score, strength, entropy, strings.Join(appendDefault(suggestions, "No immediate improvements suggested"), "\n- ")), nil
}

func passwordStrength(password string, minLen int) (int, string, []string, float64) {
	score := 0
	var suggestions []string
	length := len([]rune(password))
	if length >= minLen {
		score += 20
	} else {
		suggestions = append(suggestions, fmt.Sprintf("Use at least %d characters", minLen))
	}
	if length >= 12 {
		score += 15
	}
	if length >= 16 {
		score += 10
	}

	var hasLower, hasUpper, hasDigit, hasSpecial bool
	var pool int
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	if hasLower {
		pool += 26
	}
	if hasUpper {
		pool += 26
	}
	if hasDigit {
		pool += 10
	}
	if hasSpecial {
		pool += 33
	}
	if hasLower && hasUpper {
		score += 15
	} else {
		suggestions = append(suggestions, "Mix lowercase and uppercase letters")
	}
	if hasDigit {
		score += 15
	} else {
		suggestions = append(suggestions, "Add numbers")
	}
	if hasSpecial {
		score += 15
	} else {
		suggestions = append(suggestions, "Add special characters")
	}
	lower := strings.ToLower(password)
	common := []string{"password", "123456", "qwerty", "admin", "welcome", "letmein", "iloveyou"}
	for _, c := range common {
		if strings.Contains(lower, c) {
			score -= 30
			suggestions = append(suggestions, "Avoid common password words or sequences")
			break
		}
	}
	if hasRepeatedRun(password, 3) {
		score -= 10
		suggestions = append(suggestions, "Avoid repeated characters")
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	entropy := 0.0
	if pool > 0 && length > 0 {
		entropy = float64(length) * math.Log2(float64(pool))
	}
	strength := "weak"
	if score >= 80 {
		strength = "strong"
	} else if score >= 50 {
		strength = "medium"
	}
	return score, strength, suggestions, entropy
}

func hasRepeatedRun(s string, minRun int) bool {
	if minRun <= 1 {
		return s != ""
	}
	var last rune
	count := 0
	for _, r := range s {
		if r == last {
			count++
		} else {
			last = r
			count = 1
		}
		if count >= minRun {
			return true
		}
	}
	return false
}

func NewBuiltinPasswordStrengthCheckerTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolPasswordStrengthChecker,
			Desc:  "Check password strength, complexity, common weak patterns, and entropy estimate.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"password":   {Type: einoschema.String, Desc: "Password to check", Required: true},
				"min_length": {Type: einoschema.String, Desc: "Minimum required length (default 8)", Required: false},
			}),
		},
		execBuiltinPasswordStrengthChecker,
	)
}

var secretPatterns = []namedPattern{
	{Type: "aws_access_key_id", Severity: "high", Message: "AWS access key id detected", Re: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{Type: "aws_secret_access_key", Severity: "critical", Message: "AWS secret access key-like value detected", Re: regexp.MustCompile(`(?i)aws_secret_access_key['"]?\s*[:=]\s*['"]?[A-Za-z0-9/+=]{40}['"]?`)},
	{Type: "github_token", Severity: "critical", Message: "GitHub token detected", Re: regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{30,255}`)},
	{Type: "slack_token", Severity: "critical", Message: "Slack token detected", Re: regexp.MustCompile(`xox[baprs]-[0-9A-Za-z-]{20,}`)},
	{Type: "slack_webhook", Severity: "critical", Message: "Slack webhook URL detected", Re: regexp.MustCompile(`https://hooks\.slack\.com/services/[A-Za-z0-9/_-]+`)},
	{Type: "google_api_key", Severity: "high", Message: "Google API key detected", Re: regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`)},
	{Type: "stripe_secret_key", Severity: "critical", Message: "Stripe live secret key detected", Re: regexp.MustCompile(`sk_live_[0-9A-Za-z]{24,}`)},
	{Type: "jwt", Severity: "medium", Message: "JWT-like token detected", Re: regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)},
	{Type: "private_key", Severity: "critical", Message: "Private key block detected", Re: regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----`)},
	{Type: "database_url", Severity: "critical", Message: "Database connection URL with credentials detected", Re: regexp.MustCompile(`(?i)(mysql|postgres|postgresql|mongodb|redis)://[^:\s]+:[^@\s]+@`)},
	{Type: "generic_api_key", Severity: "medium", Message: "Generic API key/token assignment detected", Re: regexp.MustCompile(`(?i)(api[_-]?key|api[_-]?token|access[_-]?token|secret)['"]?\s*[:=]\s*['"]?[A-Za-z0-9_\-=/+]{16,}['"]?`)},
	{Type: "password_assignment", Severity: "medium", Message: "Password assignment detected", Re: regexp.MustCompile(`(?i)(password|passwd|pwd)['"]?\s*(?::=|:=|:|=)\s*['"]?[^'"\s]{6,}['"]?`)},
}

func maskSecretSnippet(s string) string {
	for _, p := range secretPatterns {
		s = p.Re.ReplaceAllStringFunc(s, func(match string) string {
			if len(match) <= 10 {
				return strings.Repeat("*", len(match))
			}
			return match[:4] + strings.Repeat("*", 12) + match[len(match)-4:]
		})
	}
	return shortSnippet(s, 180)
}

func execBuiltinSecretsScanner(_ context.Context, in map[string]any) (string, error) {
	text := strArg(in, "text", "code", "content", "input")
	if text == "" {
		return "", fmt.Errorf("missing text to scan")
	}
	findings := scanLines(text, secretPatterns, nil)
	for i := range findings {
		if findings[i].Line > 0 {
			line := strings.Split(clampTextForScan(text), "\n")[findings[i].Line-1]
			findings[i].Snippet = maskSecretSnippet(line)
		}
	}
	return formatRegexFindings("Secrets scan result", findings), nil
}

func NewBuiltinSecretsScannerTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolSecretsScanner,
			Desc:  "Scan text/code for common secrets such as API keys, tokens, private keys, passwords, and credential URLs. Matches are masked in output.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"text": {Type: einoschema.String, Desc: "Text or code to scan", Required: true},
			}),
		},
		execBuiltinSecretsScanner,
	)
}

func execBuiltinSecurityHeadersChecker(ctx context.Context, in map[string]any) (string, error) {
	rawHeaders := strArg(in, "headers", "raw_headers")
	targetURL := strArg(in, "url", "target", "endpoint")
	var headers http.Header
	status := ""
	scheme := ""
	if rawHeaders != "" {
		headers = parseHTTPHeaders(rawHeaders)
	} else {
		if targetURL == "" {
			return "", fmt.Errorf("missing url or headers")
		}
		fetchedHeaders, fetchedStatus, fetchedScheme, err := fetchSecurityHeaders(ctx, targetURL)
		if err != nil {
			return "", err
		}
		headers = fetchedHeaders
		status = fetchedStatus
		scheme = fetchedScheme
	}
	return analyzeSecurityHeaders(headers, status, scheme), nil
}

func parseHTTPHeaders(raw string) http.Header {
	h := make(http.Header)
	var obj map[string]string
	if strings.HasPrefix(strings.TrimSpace(raw), "{") && json.Unmarshal([]byte(raw), &obj) == nil {
		for k, v := range obj {
			h.Set(k, v)
		}
		return h
	}
	for _, line := range strings.Split(raw, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if ok {
			h.Add(strings.TrimSpace(k), strings.TrimSpace(v))
		}
	}
	return h
}

func fetchSecurityHeaders(ctx context.Context, rawURL string) (http.Header, string, string, error) {
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, "", "", fmt.Errorf("only http and https URLs are allowed")
	}
	if hostLooksUnsafe(u.Hostname()) {
		return nil, "", "", fmt.Errorf("host is not allowed (private or local addresses blocked)")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u.String(), nil)
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("User-Agent", "sya-security-headers-checker/1.0")
	resp, err := client.Do(req)
	if err == nil && resp != nil && resp.StatusCode != http.StatusMethodNotAllowed {
		defer resp.Body.Close()
		return resp.Header, resp.Status, u.Scheme, nil
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("User-Agent", "sya-security-headers-checker/1.0")
	resp, err = client.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp == nil {
		return nil, "", "", fmt.Errorf("request failed: empty response")
	}
	return resp.Header, resp.Status, u.Scheme, nil
}

func analyzeSecurityHeaders(headers http.Header, status, scheme string) string {
	checks := []struct {
		Header string
		Why    string
	}{
		{"Strict-Transport-Security", "enforces HTTPS on future visits"},
		{"Content-Security-Policy", "reduces XSS and injection impact"},
		{"X-Content-Type-Options", "prevents MIME sniffing"},
		{"X-Frame-Options", "reduces clickjacking risk"},
		{"Referrer-Policy", "controls referrer leakage"},
		{"Permissions-Policy", "limits browser feature access"},
		{"Cross-Origin-Opener-Policy", "isolates browsing context"},
		{"Cross-Origin-Resource-Policy", "limits cross-origin resource use"},
	}
	score := 100
	var b strings.Builder
	b.WriteString("Security headers analysis\n")
	if status != "" {
		b.WriteString("Status: " + status + "\n")
	}
	if scheme == "http" {
		score -= 25
		b.WriteString("- [high] URL uses plain HTTP; serve security-sensitive pages over HTTPS.\n")
	}
	for _, c := range checks {
		v := strings.TrimSpace(headers.Get(c.Header))
		if v == "" {
			score -= 10
			if c.Header == "Strict-Transport-Security" && scheme == "http" {
				continue
			}
			b.WriteString(fmt.Sprintf("- [missing] %s: %s.\n", c.Header, c.Why))
			continue
		}
		if c.Header == "Strict-Transport-Security" && strings.Contains(strings.ToLower(v), "max-age=0") {
			score -= 10
			b.WriteString(fmt.Sprintf("- [weak] %s disables HSTS: %s\n", c.Header, shortSnippet(v, 120)))
			continue
		}
		if c.Header == "Content-Security-Policy" && (strings.Contains(v, "'unsafe-inline'") || strings.Contains(v, "*")) {
			score -= 5
			b.WriteString(fmt.Sprintf("- [weak] %s is present but broad: %s\n", c.Header, shortSnippet(v, 120)))
			continue
		}
		b.WriteString(fmt.Sprintf("- [ok] %s: %s\n", c.Header, shortSnippet(v, 120)))
	}
	if score < 0 {
		score = 0
	}
	return fmt.Sprintf("Score: %d/100\n%s", score, strings.TrimRight(b.String(), "\n"))
}

func NewBuiltinSecurityHeadersCheckerTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolSecurityHeadersChecker,
			Desc:  "Check HTTP security response headers from a URL or raw header text.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"url":     {Type: einoschema.String, Desc: "HTTP/HTTPS URL to check", Required: false},
				"headers": {Type: einoschema.String, Desc: "Optional raw HTTP headers or JSON header object", Required: false},
			}),
		},
		execBuiltinSecurityHeadersChecker,
	)
}

var apiSecurityPatterns = []namedPattern{
	{Type: "sql_injection", Severity: "high", Message: "SQL query appears to concatenate user-controlled input", Re: regexp.MustCompile(`(?i)(select|insert|update|delete).*(\+|fmt\.Sprintf|sprintf|format\(|\$\{)`)},
	{Type: "sql_injection", Severity: "high", Message: "Database execution uses a formatted SQL string", Re: regexp.MustCompile(`(?i)(db\.query|execute|exec)\s*\([^)]*(fmt\.Sprintf|sprintf|format\()`)},
	{Type: "xss", Severity: "high", Message: "HTML is written without clear escaping", Re: regexp.MustCompile(`(?i)(innerHTML\s*=|dangerouslySetInnerHTML|document\.write\(|v-html=)`)},
	{Type: "command_injection", Severity: "critical", Message: "Shell command execution may include dynamic input", Re: regexp.MustCompile(`(?i)(exec|system|shell_exec|popen|Command)\s*\([^)]*(req\.|request|params|body|query|\+)`)},
	{Type: "path_traversal", Severity: "high", Message: "File path operation may use request input or traversal sequences", Re: regexp.MustCompile(`(?i)(readFile|open|os\.Open|send_file|filepath\.Join).*(\.\./|req\.|request|params|query)`)},
	{Type: "unsafe_deserialization", Severity: "critical", Message: "Unsafe deserialization primitive detected", Re: regexp.MustCompile(`(?i)(pickle\.load|yaml\.load\(|ObjectInputStream|XMLDecoder|unserialize\()`)},
	{Type: "weak_auth", Severity: "medium", Message: "Hard-coded permissive CORS or missing credential boundary suspected", Re: regexp.MustCompile(`(?i)(Access-Control-Allow-Origin['"]?\s*[:=]\s*['"]?\*|cors\(\s*\))`)},
}

func execBuiltinAPISecurityChecker(_ context.Context, in map[string]any) (string, error) {
	code := strArg(in, "code", "source", "content", "input")
	if code == "" {
		return "", fmt.Errorf("missing code or request text to check")
	}
	only := parseStringListArg(strArg(in, "check_types", "checks"))
	findings := scanLines(code, apiSecurityPatterns, only)
	return formatRegexFindings("API security scan result", findings), nil
}

func NewBuiltinAPISecurityCheckerTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolAPISecurityChecker,
			Desc:  "Static scan for common API security risks: SQL injection, XSS, command injection, path traversal, unsafe deserialization, and weak CORS.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"code":        {Type: einoschema.String, Desc: "Code, handler, route, or request text to inspect", Required: true},
				"language":    {Type: einoschema.String, Desc: "Optional language hint", Required: false},
				"check_types": {Type: einoschema.String, Desc: "Optional comma-separated or JSON array of check types", Required: false},
			}),
		},
		execBuiltinAPISecurityChecker,
	)
}

var logSecurityPatterns = []namedPattern{
	{Type: "failed_login", Severity: "medium", Message: "Failed authentication attempt", Re: regexp.MustCompile(`(?i)(failed|invalid|incorrect).{0,40}(login|password|auth|user)`)},
	{Type: "brute_force", Severity: "high", Message: "Possible repeated authentication failures", Re: regexp.MustCompile(`(?i)(too many|multiple|repeated|brute).{0,40}(login|auth|attempt|failure)`)},
	{Type: "sql_injection_attempt", Severity: "high", Message: "Possible SQL injection payload in log", Re: regexp.MustCompile(`(?i)(union\s+select|or\s+1=1|drop\s+table|sleep\(|benchmark\(|--\s)`)},
	{Type: "path_traversal", Severity: "high", Message: "Path traversal pattern detected", Re: regexp.MustCompile(`(\.\./|\.\.\\|%2e%2e%2f|%252e%252e%252f)`)},
	{Type: "xss_attempt", Severity: "medium", Message: "Possible XSS payload in request/log", Re: regexp.MustCompile(`(?i)(<script|javascript:|onerror=|onload=)`)},
	{Type: "privilege_escalation", Severity: "high", Message: "Privilege escalation or sudo failure signal", Re: regexp.MustCompile(`(?i)(sudo|su|root|privilege).{0,40}(denied|failed|not allowed|error)`)},
	{Type: "secret_leak", Severity: "critical", Message: "Secret-like value appears in logs", Re: regexp.MustCompile(`(?i)(authorization:\s*bearer|api[_-]?key|password=|token=)`)},
}

func execBuiltinLogSecurityAnalyzer(_ context.Context, in map[string]any) (string, error) {
	logText := strArg(in, "log_text", "log", "content", "text")
	if logText == "" {
		return "", fmt.Errorf("missing log_text")
	}
	findings := scanLines(logText, logSecurityPatterns, nil)
	return formatRegexFindings("Log security analysis result", findings), nil
}

func NewBuiltinLogSecurityAnalyzerTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolLogSecurityAnalyzer,
			Desc:  "Analyze logs for security events such as failed login, brute force, SQL injection attempts, path traversal, XSS, and leaked secrets.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"log_text":     {Type: einoschema.String, Desc: "Log content to analyze", Required: true},
				"log_type":     {Type: einoschema.String, Desc: "Optional log type hint", Required: false},
				"threat_level": {Type: einoschema.String, Desc: "Optional sensitivity: low, medium, high", Required: false},
			}),
		},
		execBuiltinLogSecurityAnalyzer,
	)
}

func execBuiltinCryptoTool(_ context.Context, in map[string]any) (string, error) {
	op := strings.ToLower(strArg(in, "operation", "op", "action"))
	algorithm := strings.ToUpper(strArg(in, "algorithm", "algo", "cipher"))
	data := strArg(in, "data", "text", "input", "message")
	key := strArg(in, "key", "secret", "password")
	switch op {
	case "hash":
		if data == "" {
			return "", fmt.Errorf("missing data")
		}
		if algorithm == "" {
			algorithm = "SHA256"
		}
		return hashString(algorithm, data)
	case "hmac":
		if data == "" || key == "" {
			return "", fmt.Errorf("missing data or key")
		}
		if algorithm == "" {
			algorithm = "SHA256"
		}
		return hmacString(algorithm, key, data)
	case "encrypt":
		if data == "" || key == "" {
			return "", fmt.Errorf("missing data or key")
		}
		return aesGCMEncrypt(key, []byte(data))
	case "decrypt":
		if data == "" || key == "" {
			return "", fmt.Errorf("missing data or key")
		}
		plain, err := aesGCMDecrypt(key, data)
		if err != nil {
			return "", err
		}
		return string(plain), nil
	case "random":
		length := 32
		if raw := strArg(in, "length"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 4096 {
				length = n
			}
		}
		buf := make([]byte, length)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(buf), nil
	case "base64_encode", "encode":
		return base64.StdEncoding.EncodeToString([]byte(data)), nil
	case "base64_decode", "decode":
		b, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return "", fmt.Errorf("unknown crypto operation: %s (supported: hash, hmac, encrypt, decrypt, random, base64_encode, base64_decode)", op)
	}
}

func hashString(algorithm, data string) (string, error) {
	switch strings.ToUpper(algorithm) {
	case "MD5":
		sum := md5.Sum([]byte(data))
		return hex.EncodeToString(sum[:]), nil
	case "SHA1", "SHA_1":
		sum := sha1.Sum([]byte(data))
		return hex.EncodeToString(sum[:]), nil
	case "SHA256", "SHA_256", "":
		sum := sha256.Sum256([]byte(data))
		return hex.EncodeToString(sum[:]), nil
	case "SHA512", "SHA_512":
		sum := sha512.Sum512([]byte(data))
		return hex.EncodeToString(sum[:]), nil
	default:
		return "", fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}
}

func hmacString(algorithm, key, data string) (string, error) {
	var mac hashWriter
	switch strings.ToUpper(algorithm) {
	case "SHA1", "SHA_1":
		mac = hmac.New(sha1.New, []byte(key))
	case "SHA256", "SHA_256", "":
		mac = hmac.New(sha256.New, []byte(key))
	case "SHA512", "SHA_512":
		mac = hmac.New(sha512.New, []byte(key))
	default:
		return "", fmt.Errorf("unsupported HMAC algorithm: %s", algorithm)
	}
	_, _ = mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func aesKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func aesGCMEncrypt(secret string, plain []byte) (string, error) {
	block, err := aes.NewCipher(aesKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, plain, nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func aesGCMDecrypt(secret string, encoded string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(aesKey(secret))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func NewBuiltinCryptoTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolCryptoTool,
			Desc:  "Server-side crypto utilities: hash, HMAC, AES-GCM encrypt/decrypt, random bytes, and Base64 encode/decode.",
			Extra: map[string]any{"execution_mode": "server"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: hash, hmac, encrypt, decrypt, random, base64_encode, base64_decode", Required: true},
				"algorithm": {Type: einoschema.String, Desc: "Algorithm: MD5, SHA1, SHA256, SHA512, AES-GCM", Required: false},
				"data":      {Type: einoschema.String, Desc: "Input data", Required: false},
				"key":       {Type: einoschema.String, Desc: "Secret key for hmac/encrypt/decrypt", Required: false},
				"length":    {Type: einoschema.String, Desc: "Random byte length (default 32, max 4096)", Required: false},
			}),
		},
		execBuiltinCryptoTool,
	)
}
