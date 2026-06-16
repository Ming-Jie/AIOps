package imoutbound

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"
)

const minIMImageDimension = 16

var (
	pngMagic  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	jpegMagic = []byte{0xFF, 0xD8, 0xFF}
)

func isRasterImageFilename(filename string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return true
	default:
		return false
	}
}

// ValidateRasterImageBytes rejects placeholder or invalid raster images before IM upload/salvage.
func ValidateRasterImageBytes(filename string, data []byte) error {
	if !isRasterImageFilename(filename) {
		return nil
	}
	if len(data) == 0 {
		return fmt.Errorf("empty image %s", filename)
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".webp":
		if len(data) < 12 || !bytes.Equal(data[0:4], []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WEBP")) {
			return invalidRasterImageErr(filename, "invalid WEBP header")
		}
		if len(data) < 512 {
			return invalidRasterImageErr(filename, fmt.Sprintf("file too small (%d bytes)", len(data)))
		}
		return nil
	case ".bmp":
		if len(data) < 2 || data[0] != 'B' || data[1] != 'M' {
			return invalidRasterImageErr(filename, "invalid BMP header")
		}
		if len(data) < 512 {
			return invalidRasterImageErr(filename, fmt.Sprintf("file too small (%d bytes)", len(data)))
		}
		return nil
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return invalidRasterImageErr(filename, err.Error())
	}
	if cfg.Width < minIMImageDimension || cfg.Height < minIMImageDimension {
		return fmt.Errorf("image %s is only %dx%d (%s): cannot send placeholder pixels — builtin_im_save_file does not generate photos; use builtin_terminal or builtin_browser screenshot for real image bytes, or tell the user you cannot draw images",
			filename, cfg.Width, cfg.Height, format)
	}
	if ext == ".png" && !bytes.HasPrefix(data, pngMagic) {
		return invalidRasterImageErr(filename, "missing PNG signature")
	}
	if (ext == ".jpg" || ext == ".jpeg") && !bytes.HasPrefix(data, jpegMagic) {
		return invalidRasterImageErr(filename, "missing JPEG signature")
	}
	return nil
}

func invalidRasterImageErr(filename, detail string) error {
	return fmt.Errorf("invalid image %s (%s): do not fabricate base64 placeholders — use builtin_terminal or browser screenshot for real files", filename, detail)
}

// IsPlaceholderImageToolError reports validation failures from fake LLM-generated PNG/JPEG bytes.
func IsPlaceholderImageToolError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "placeholder pixels") ||
		strings.Contains(msg, "cannot send placeholder") ||
		strings.Contains(msg, "does not generate photos") ||
		strings.Contains(msg, "fabricate base64")
}
