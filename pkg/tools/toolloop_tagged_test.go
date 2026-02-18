package tools

import (
	"context"
	"testing"

	"github.com/srikesh3005/summer/pkg/providers"
)

type taggedMockProvider struct{}

func (p *taggedMockProvider) Chat(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, model string, options map[string]interface{}) (*providers.LLMResponse, error) {
	if len(messages) == 1 {
		return &providers.LLMResponse{
			Content: `<append_file>{"path":"/tmp/test.txt","content":"hello"}</append_file>`,
		}, nil
	}
	return &providers.LLMResponse{
		Content: "done",
	}, nil
}

func (p *taggedMockProvider) GetDefaultModel() string { return "mock" }

type taggedMockTool struct {
	executed bool
}

func (t *taggedMockTool) Name() string { return "append_file" }

func (t *taggedMockTool) Description() string { return "mock append file tool" }

func (t *taggedMockTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":    map[string]interface{}{"type": "string"},
			"content": map[string]interface{}{"type": "string"},
		},
	}
}

func (t *taggedMockTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	t.executed = true
	return NewToolResult("ok")
}

func TestRunToolLoop_ParsesTaggedToolCalls(t *testing.T) {
	reg := NewToolRegistry()
	tool := &taggedMockTool{}
	reg.Register(tool)

	result, err := RunToolLoop(context.Background(), ToolLoopConfig{
		Provider:      &taggedMockProvider{},
		Model:         "mock",
		Tools:         reg,
		MaxIterations: 4,
	}, []providers.Message{
		{Role: "user", Content: "test"},
	}, "", "")
	if err != nil {
		t.Fatalf("RunToolLoop() error = %v", err)
	}

	if !tool.executed {
		t.Fatal("expected append_file tool to be executed from tagged output")
	}
	if result.Content != "done" {
		t.Errorf("result.Content = %q, want %q", result.Content, "done")
	}
	if result.Iterations != 2 {
		t.Errorf("result.Iterations = %d, want 2", result.Iterations)
	}
}
