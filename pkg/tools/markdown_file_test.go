package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/srikesh3005/summer/pkg/bus"
)

func TestMarkdownFileTool_CreateAndSend(t *testing.T) {
	tmpDir := t.TempDir()
	mb := bus.NewMessageBus()
	tool := NewMarkdownFileTool(tmpDir, true, mb)
	tool.SetContext("telegram", "12345")

	result := tool.Execute(context.Background(), map[string]interface{}{
		"path":    "reports/test-summary.md",
		"content": "# Summary\n\nhello",
		"send":    true,
		"caption": "summary file",
	})
	if result.IsError {
		t.Fatalf("expected no error, got: %s", result.ForLLM)
	}

	outPath := filepath.Join(tmpDir, "reports", "test-summary.md")
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected file to exist, stat error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	out, ok := mb.SubscribeOutbound(ctx)
	if !ok {
		t.Fatal("expected outbound message")
	}
	if out.Channel != "telegram" || out.ChatID != "12345" {
		t.Fatalf("unexpected destination: %s:%s", out.Channel, out.ChatID)
	}
	if out.FilePath != outPath {
		t.Fatalf("unexpected file path: %s", out.FilePath)
	}
	if out.FileName != "test-summary.md" {
		t.Fatalf("unexpected file name: %s", out.FileName)
	}
}

func TestMarkdownFileTool_CreateOnly(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewMarkdownFileTool(tmpDir, true, nil)

	result := tool.Execute(context.Background(), map[string]interface{}{
		"path":    "notes/daily",
		"content": "ok",
		"send":    false,
	})
	if result.IsError {
		t.Fatalf("expected no error, got: %s", result.ForLLM)
	}

	outPath := filepath.Join(tmpDir, "notes", "daily.md")
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected file to exist, stat error: %v", err)
	}
}
