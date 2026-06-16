package skills

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fisk086/aiops/internal/imoutbound"
)

const maxIMFileBytes = 1 << 20

// ValidateIMImageFile checks an on-disk IM attachment before upload to Lark/DingTalk.
func ValidateIMImageFile(absPath string) error {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	return imoutbound.ValidateRasterImageBytes(filepath.Base(absPath), data)
}

// prepareIMFileBytes decodes tool content: plain UTF-8, data: URI, or raw base64.
func prepareIMFileBytes(filename, content string) ([]byte, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("empty content")
	}
	var data []byte
	if strings.HasPrefix(content, "data:") {
		if idx := strings.Index(content, ";base64,"); idx >= 0 {
			raw := strings.ReplaceAll(content[idx+8:], "\n", "")
			dec, err := base64.StdEncoding.DecodeString(raw)
			if err != nil {
				return nil, fmt.Errorf("invalid data URI base64: %w", err)
			}
			data = dec
		}
	}
	if data == nil {
		compact := strings.ReplaceAll(strings.ReplaceAll(content, "\n", ""), " ", "")
		if dec, ok := tryDecodeBase64String(compact); ok {
			data = dec
		}
	}
	if data == nil {
		data = []byte(content)
	}
	if len(data) > maxIMFileBytes {
		return nil, fmt.Errorf("content too large (max %d bytes)", maxIMFileBytes)
	}
	if err := imoutbound.ValidateRasterImageBytes(filename, data); err != nil {
		return nil, err
	}
	return data, nil
}

func tryDecodeBase64String(s string) ([]byte, bool) {
	if len(s) < 8 || len(s)%4 != 0 {
		return nil, false
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '+', r == '/', r == '=':
		default:
			return nil, false
		}
	}
	dec, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(dec) == 0 {
		return nil, false
	}
	return dec, true
}
