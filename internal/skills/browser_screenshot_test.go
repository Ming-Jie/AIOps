package skills

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestExtractScreenshotFilename_trailingDot(t *testing.T) {
	in := "title: 百度一下\nScreenshot saved as screenshot_1781348443299.png."
	got := ExtractScreenshotFilename(in)
	if got != "screenshot_1781348443299.png" {
		t.Fatalf("got %q", got)
	}
}

func TestEnrichScreenshotToolResult_embedsDataURI(t *testing.T) {
	fname := "screenshot_999.png"
	b64 := base64.StdEncoding.EncodeToString([]byte("png-bytes"))
	screenshotStore.Store(fname, b64)
	defer screenshotStore.Delete(fname)

	raw := "Screenshot saved as " + fname
	got := EnrichScreenshotToolResult(raw)
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("missing data uri prefix: %q", got)
	}
	if !strings.Contains(got, raw) {
		t.Fatalf("should keep original tool text: %q", got)
	}
	if _, ok := screenshotStore.Load(fname); ok {
		t.Fatal("screenshot should be popped after enrich")
	}
}
