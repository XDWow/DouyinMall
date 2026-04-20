package session

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestVisibleTranscriptSchemaMessages_keepsUserAssistantText(t *testing.T) {
	in := []*schema.Message{
		schema.UserMessage("hello"),
		schema.AssistantMessage("hi", nil),
	}
	got := VisibleTranscriptSchemaMessages(in)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestVisibleTranscriptSchemaMessages_dropsToolSystemAndToolOnlyAssistant(t *testing.T) {
	toolStub, _ := schema.AssistantMessage("", []schema.ToolCall{{ID: "1", Name: "x", Arguments: "{}"}})
	in := []*schema.Message{
		schema.SystemMessage("sys"),
		schema.UserMessage("u1"),
		toolStub,
		schema.ToolMessage(`{"ok":true}`, "1"),
		schema.AssistantMessage("visible", nil),
	}
	got := VisibleTranscriptSchemaMessages(in)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (u1 + visible)", len(got))
	}
	if got[0].Role != schema.User || got[0].Content != "u1" {
		t.Fatalf("first = %#v", got[0])
	}
	if got[1].Role != schema.Assistant || got[1].Content != "visible" {
		t.Fatalf("second = %#v", got[1])
	}
}

func TestVisibleTranscriptSchemaMessages_skipsEmptyUser(t *testing.T) {
	in := []*schema.Message{
		schema.UserMessage("   "),
		schema.AssistantMessage("only", nil),
	}
	got := VisibleTranscriptSchemaMessages(in)
	if len(got) != 1 || got[0].Content != "only" {
		t.Fatalf("got %#v", got)
	}
}
