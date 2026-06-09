package workflow

import "testing"

func TestFormatWorkflowOutputCodeNode(t *testing.T) {
	got := FormatWorkflowOutput(map[string]any{
		"type":     "code",
		"language": "python",
		"output":   "ip地址 153.37.166.153\n",
	})
	want := "ip地址 153.37.166.153"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatWorkflowOutputEndWrapsCode(t *testing.T) {
	got := FormatWorkflowOutput(map[string]any{
		"type": "end",
		"output": map[string]any{
			"type":     "code",
			"language": "python",
			"output":   "ip地址 153.37.166.153",
		},
	})
	want := "ip地址 153.37.166.153"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatWorkflowOutputPlainString(t *testing.T) {
	got := FormatWorkflowOutput("hello")
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}
