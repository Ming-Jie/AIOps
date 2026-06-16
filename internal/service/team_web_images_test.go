package service

import (
	"strings"
	"testing"

	"github.com/fisk086/aiops/internal/imoutbound"
)

func TestExpandTeamMarkdownImageRefsForWeb_OutboundFile(t *testing.T) {
	dir := t.TempDir()
	store := imoutbound.GlobalStore()
	store.SetBase(dir)

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	scope := imoutbound.Scope{AgentID: 7, SessionID: imoutbound.TeamConvSessionID(12)}
	if _, err := store.WriteFileBytes(scope, "screenshot_99.png", png); err != nil {
		t.Fatal(err)
	}

	in := "截图：\n\n![百度首页截图](screenshot_99.png)\n"
	out := expandTeamMarkdownImageRefsForWeb(in, 12, []int64{7}, nil)
	if out == in {
		t.Fatalf("expected expansion, got %q", out)
	}
	if !strings.Contains(out, "data:image/png;base64,") {
		t.Fatalf("missing data uri: %q", out)
	}
}

func TestExpandTeamIMFileMarkersForWeb(t *testing.T) {
	dir := t.TempDir()
	store := imoutbound.GlobalStore()
	store.SetBase(dir)

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	scope := imoutbound.Scope{AgentID: 3, SessionID: imoutbound.TeamConvSessionID(5)}
	if _, err := store.WriteFileBytes(scope, "screenshot_1781261814307.png", png); err != nil {
		t.Fatal(err)
	}

	in := "请查收：[[im_file:screenshot_1781261814307.png]]"
	out := expandTeamIMFileMarkersForWeb(in, 5, []int64{3}, nil)
	if strings.Contains(out, "[[im_file:") {
		t.Fatalf("marker should be replaced: %q", out)
	}
	if !strings.Contains(out, "![截图](data:image/png;base64,") {
		t.Fatalf("expected markdown image data uri: %q", out)
	}
}

func TestExpandTeamImagesForWeb_dedupesSameFile(t *testing.T) {
	dir := t.TempDir()
	store := imoutbound.GlobalStore()
	store.SetBase(dir)

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	scope := imoutbound.Scope{AgentID: 2, SessionID: imoutbound.TeamConvSessionID(9)}
	if _, err := store.WriteFileBytes(scope, "screenshot_a.png", png); err != nil {
		t.Fatal(err)
	}

	seen := make(map[string]struct{})
	first := expandTeamImagesForWeb("请查收：[[im_file:screenshot_a.png]]", 9, []int64{2}, seen)
	second := expandTeamImagesForWeb("总结：[[im_file:screenshot_a.png]]", 9, []int64{2}, seen)

	if !strings.Contains(first, "data:image/png;base64,") {
		t.Fatalf("first should expand image: %q", first)
	}
	if strings.Contains(second, "data:image/png;base64,") {
		t.Fatalf("second should not repeat image: %q", second)
	}
}
