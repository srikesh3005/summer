package providers

import (
	"strings"
	"testing"
)

func TestExtractTaggedToolCalls(t *testing.T) {
	text := `I'll update that.
<append_file>{"path":"/tmp/a.txt","content":"hello"}</append_file>
Done.`

	calls := ExtractTaggedToolCalls(text)
	if len(calls) != 1 {
		t.Fatalf("ExtractTaggedToolCalls() len = %d, want 1", len(calls))
	}
	if calls[0].Name != "append_file" {
		t.Errorf("Name = %q, want %q", calls[0].Name, "append_file")
	}
	if calls[0].Arguments["path"] != "/tmp/a.txt" {
		t.Errorf("Arguments[path] = %v, want /tmp/a.txt", calls[0].Arguments["path"])
	}
}

func TestExtractTaggedToolCalls_WithoutClosingTag(t *testing.T) {
	text := `<append_file>{"path":"/tmp/a.txt","content":"hello"}`
	calls := ExtractTaggedToolCalls(text)
	if len(calls) != 1 {
		t.Fatalf("ExtractTaggedToolCalls() len = %d, want 1", len(calls))
	}
}

func TestStripTaggedToolCallsProvider(t *testing.T) {
	text := `before
<append_file>{"path":"/tmp/a.txt","content":"hello"}</append_file>
after`
	clean := StripTaggedToolCalls(text)

	if strings.Contains(clean, "<append_file>") {
		t.Errorf("StripTaggedToolCalls() still contains tag: %q", clean)
	}
	if !strings.Contains(clean, "before") || !strings.Contains(clean, "after") {
		t.Errorf("StripTaggedToolCalls() should preserve surrounding text, got %q", clean)
	}
}
