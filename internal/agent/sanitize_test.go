package agent

import (
	"strings"
	"testing"
)

func TestSanitizeAssistantContent_StripsRawToolCallSectionMarkers(t *testing.T) {
	in := "Let me check:<|toolcallssectionbegin|><|toolcallbegin|>functions.skillsearch:5<|toolcallargumentbegin|>{\"query\":\"\"}<|toolcallend|><|toolcallssectionend|>"
	out := SanitizeAssistantContent(in)

	if strings.Contains(out, "<|toolcall") || strings.Contains(out, "<|toolcalls") {
		t.Fatalf("expected tool-call markers removed, got: %q", out)
	}
	if strings.TrimSpace(out) != "Let me check:" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestSanitizeAssistantContent_StripsStandaloneRawToolCallMarkers(t *testing.T) {
	in := "Hello <|toolcallbegin|>world<|toolcallend|> done"
	out := SanitizeAssistantContent(in)
	if strings.Contains(out, "<|toolcall") {
		t.Fatalf("expected standalone markers removed, got: %q", out)
	}
	if strings.TrimSpace(out) != "Hello world done" {
		t.Fatalf("unexpected output: %q", out)
	}
}
