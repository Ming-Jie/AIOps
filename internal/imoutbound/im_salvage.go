package imoutbound

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fisk086/aiops/internal/logger"
)

const maxSalvageFileBytes = 1 << 20

var (
	imDataURIRe          = regexp.MustCompile(`data:([a-zA-Z0-9+.-]+)/([a-zA-Z0-9+.-]+);base64,([A-Za-z0-9+/=\s]+)`)
	imStandaloneBase64Re = regexp.MustCompile(`(?m)^[A-Za-z0-9+/]{20,}={0,2}\s*$`)
)

// ContainsSalvageableIMPayload reports inline file body, data URI, or base64 in IM assistant text.
func ContainsSalvageableIMPayload(text string) bool {
	if ContainsPastedFileContent(text) {
		return true
	}
	if _, _, ok := ExtractDataURIBlob(text); ok {
		return true
	}
	return ExtractStandaloneBase64(text) != ""
}

// ExtractDataURIBlob pulls mime/subtype and raw bytes from a data: URI in text.
func ExtractDataURIBlob(text string) (filename string, data []byte, ok bool) {
	m := imDataURIRe.FindStringSubmatch(text)
	if len(m) < 4 {
		return "", nil, false
	}
	sub := strings.ToLower(strings.TrimSpace(m[2]))
	if sub == "jpeg" {
		sub = "jpg"
	}
	raw := strings.ReplaceAll(strings.ReplaceAll(m[3], "\n", ""), " ", "")
	dec, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(dec) == 0 || len(dec) > maxSalvageFileBytes {
		return "", nil, false
	}
	name := "image." + sub
	if sub == "octet-stream" || sub == "plain" {
		name = "attachment.bin"
	}
	safe, err := SanitizeFileName(name)
	if err != nil {
		return "", nil, false
	}
	return safe, dec, true
}

// ExtractStandaloneBase64 returns concatenated base64 payload from reply lines.
func ExtractStandaloneBase64(text string) string {
	var parts []string
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if imStandaloneBase64Re.MatchString(trim) || (len(trim) >= 20 && isLikelyBase64Line(trim)) {
			parts = append(parts, strings.ReplaceAll(trim, " ", ""))
		}
	}
	return strings.Join(parts, "")
}

func isLikelyBase64Line(s string) bool {
	if len(s) < 20 {
		return false
	}
	valid := 0
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '+', r == '/', r == '=', r == ' ', r == '\t':
			valid++
		default:
			return false
		}
	}
	return valid >= len(s)*9/10
}

func guessSalvageFilename(userRequest string, ext string) string {
	userRequest = strings.ToLower(userRequest)
	for _, token := range strings.FieldsFunc(userRequest, func(r rune) bool {
		return r == ' ' || r == '"' || r == '\'' || r == '，' || r == '。'
	}) {
		if n, err := SanitizeFileName(token); err == nil && strings.Contains(token, ".") {
			return n
		}
	}
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")
	if ext == "" {
		ext = "txt"
	}
	return fmt.Sprintf("attachment_%d.%s", time.Now().Unix()%100000, ext)
}

// SalvageIMInlinePayload writes pasted text/base64/data-URI from the reply into outbound storage.
func SalvageIMInlinePayload(ctx context.Context, store *Store, scope Scope, replyText, userRequest string) (string, bool) {
	if store == nil {
		store = GlobalStore()
	}
	if name, ok := SalvagePastedIMFile(ctx, store, scope, replyText); ok {
		return name, true
	}
	if name, data, ok := ExtractDataURIBlob(replyText); ok {
		return writeSalvagedBytes(ctx, store, scope, name, data, "data-uri")
	}
	b64 := ExtractStandaloneBase64(replyText)
	if b64 == "" {
		return "", false
	}
	dec, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(dec) == 0 || len(dec) > maxSalvageFileBytes {
		return "", false
	}
	ext := "txt"
	if !utf8LooksLikeText(dec) {
		ext = "bin"
	}
	name := guessSalvageFilename(userRequest, ext)
	return writeSalvagedBytes(ctx, store, scope, name, dec, "base64")
}

func writeSalvagedBytes(ctx context.Context, store *Store, scope Scope, name string, data []byte, kind string) (string, bool) {
	safe, err := SanitizeFileName(name)
	if err != nil {
		return "", false
	}
	if err := ValidateRasterImageBytes(safe, data); err != nil {
		logger.Warn("imoutbound: salvage rejected invalid image bytes", "file", safe, "kind", kind, "err", err)
		return "", false
	}
	if _, err := store.WriteFileBytes(scope, safe, data); err != nil {
		logger.Warn("imoutbound: salvage write failed", "file", safe, "kind", kind, "err", err)
		return "", false
	}
	RegisterWrittenFile(ctx, safe)
	logger.Warn("imoutbound: salvaged LLM inline payload into outbound (prefer builtin_im_save_file)",
		"file", safe, "kind", kind, "bytes", len(data), "session_id", scope.SessionID)
	return safe, true
}

func utf8LooksLikeText(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	printable := 0
	for _, b := range data {
		if b == '\n' || b == '\r' || b == '\t' || (b >= 32 && b < 127) || b >= 0xc0 {
			printable++
		}
	}
	return printable >= len(data)*9/10
}

// StripIMInlinePayload removes pasted file bodies, base64 blobs, and data URIs from IM-visible text.
func StripIMInlinePayload(text string) string {
	text = StripPastedFileBody(text)
	text = imDataURIRe.ReplaceAllString(text, "")
	text = imStandaloneBase64Re.ReplaceAllString(text, "")
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			if len(kept) > 0 && kept[len(kept)-1] != "" {
				kept = append(kept, "")
			}
			continue
		}
		if isLikelyBase64Line(trim) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// ExtFromFilename returns a normalized extension including dot.
func ExtFromFilename(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return ""
	}
	return strings.ToLower(ext)
}
