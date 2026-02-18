package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/srikesh3005/summer/pkg/bus"
)

// MarkdownFileTool creates a markdown file and can send it to the active chat.
type MarkdownFileTool struct {
	workspace string
	restrict  bool
	msgBus    *bus.MessageBus
	channel   string
	chatID    string
}

func NewMarkdownFileTool(workspace string, restrict bool, msgBus *bus.MessageBus) *MarkdownFileTool {
	return &MarkdownFileTool{
		workspace: workspace,
		restrict:  restrict,
		msgBus:    msgBus,
	}
}

func (t *MarkdownFileTool) Name() string {
	return "markdown_file"
}

func (t *MarkdownFileTool) Description() string {
	return "Create a markdown (.md) file and optionally send it to the current chat (Telegram supported)"
}

func (t *MarkdownFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to markdown file (defaults to reports/summary-<timestamp>.md)",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Markdown content to write",
			},
			"send": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to send the file to current chat after writing (default: true)",
			},
			"caption": map[string]interface{}{
				"type":        "string",
				"description": "Optional caption when sending the file",
			},
		},
		"required": []string{"content"},
	}
}

func (t *MarkdownFileTool) SetContext(channel, chatID string) {
	t.channel = channel
	t.chatID = chatID
}

func (t *MarkdownFileTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	content, ok := args["content"].(string)
	if !ok {
		return ErrorResult("content is required")
	}

	path, _ := args["path"].(string)
	if path == "" {
		path = filepath.Join("reports", fmt.Sprintf("summary-%d.md", time.Now().Unix()))
	}
	if !strings.HasSuffix(strings.ToLower(path), ".md") {
		path += ".md"
	}

	resolvedPath, err := validatePath(path, t.workspace, t.restrict)
	if err != nil {
		return ErrorResult(err.Error())
	}
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0755); err != nil {
		return ErrorResult(fmt.Sprintf("failed to create directory: %v", err))
	}
	if err := os.WriteFile(resolvedPath, []byte(content), 0644); err != nil {
		return ErrorResult(fmt.Sprintf("failed to write markdown file: %v", err))
	}

	send := true
	if v, ok := args["send"].(bool); ok {
		send = v
	}
	caption, _ := args["caption"].(string)
	if caption == "" {
		caption = "Here is your markdown file."
	}

	if send {
		if t.msgBus == nil {
			return ErrorResult("message bus is not configured for sending files")
		}
		if t.channel == "" || t.chatID == "" {
			return ErrorResult("no active channel/chat context to send file")
		}
		t.msgBus.PublishOutbound(bus.OutboundMessage{
			Channel:  t.channel,
			ChatID:   t.chatID,
			Content:  caption,
			FilePath: resolvedPath,
			FileName: filepath.Base(resolvedPath),
		})
		return SilentResult(fmt.Sprintf("Markdown file created and sent: %s", resolvedPath))
	}

	return SilentResult(fmt.Sprintf("Markdown file created: %s", resolvedPath))
}
