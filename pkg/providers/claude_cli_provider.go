package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ClaudeCliProvider implements LLMProvider using the claude CLI as a subprocess.
type ClaudeCliProvider struct {
	command   string
	workspace string
}

// NewClaudeCliProvider creates a new Claude CLI provider.
func NewClaudeCliProvider(workspace string) *ClaudeCliProvider {
	return &ClaudeCliProvider{
		command:   "claude",
		workspace: workspace,
	}
}

// Chat implements LLMProvider.Chat by executing the claude CLI.
func (p *ClaudeCliProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]interface{}) (*LLMResponse, error) {
	systemPrompt := p.buildSystemPrompt(messages, tools)
	prompt := p.messagesToPrompt(messages)

	args := []string{"-p", "--output-format", "json", "--dangerously-skip-permissions", "--no-chrome"}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}
	if model != "" && model != "claude-code" {
		args = append(args, "--model", model)
	}
	args = append(args, "-") // read from stdin

	cmd := exec.CommandContext(ctx, p.command, args...)
	if p.workspace != "" {
		cmd.Dir = p.workspace
	}
	cmd.Stdin = bytes.NewReader([]byte(prompt))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderrStr := stderr.String(); stderrStr != "" {
			return nil, fmt.Errorf("claude cli error: %s", stderrStr)
		}
		return nil, fmt.Errorf("claude cli error: %w", err)
	}

	return p.parseClaudeCliResponse(stdout.String())
}

// GetDefaultModel returns the default model identifier.
func (p *ClaudeCliProvider) GetDefaultModel() string {
	return "claude-code"
}

// messagesToPrompt converts messages to a CLI-compatible prompt string.
func (p *ClaudeCliProvider) messagesToPrompt(messages []Message) string {
	var parts []string

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			// handled via --system-prompt flag
		case "user":
			parts = append(parts, "User: "+msg.Content)
		case "assistant":
			parts = append(parts, "Assistant: "+msg.Content)
		case "tool":
			parts = append(parts, fmt.Sprintf("[Tool Result for %s]: %s", msg.ToolCallID, msg.Content))
		}
	}

	// Simplify single user message
	if len(parts) == 1 && strings.HasPrefix(parts[0], "User: ") {
		return strings.TrimPrefix(parts[0], "User: ")
	}

	return strings.Join(parts, "\n")
}

// buildSystemPrompt combines system messages and tool definitions.
func (p *ClaudeCliProvider) buildSystemPrompt(messages []Message, tools []ToolDefinition) string {
	var parts []string

	for _, msg := range messages {
		if msg.Role == "system" {
			parts = append(parts, msg.Content)
		}
	}

	if len(tools) > 0 {
		parts = append(parts, p.buildToolsPrompt(tools))
	}

	return strings.Join(parts, "\n\n")
}

// buildToolsPrompt creates the tool definitions section for the system prompt.
func (p *ClaudeCliProvider) buildToolsPrompt(tools []ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString("## Available Tools\n\n")
	sb.WriteString("When you need to use a tool, respond with ONLY a JSON object:\n\n")
	sb.WriteString("```json\n")
	sb.WriteString(`{"tool_calls":[{"id":"call_xxx","type":"function","function":{"name":"tool_name","arguments":"{...}"}}]}`)
	sb.WriteString("\n```\n\n")
	sb.WriteString("CRITICAL: The 'arguments' field MUST be a JSON-encoded STRING.\n\n")
	sb.WriteString("### Tool Definitions:\n\n")

	for _, tool := range tools {
		if tool.Type != "function" {
			continue
		}
		sb.WriteString(fmt.Sprintf("#### %s\n", tool.Function.Name))
		if tool.Function.Description != "" {
			sb.WriteString(fmt.Sprintf("Description: %s\n", tool.Function.Description))
		}
		if len(tool.Function.Parameters) > 0 {
			paramsJSON, _ := json.Marshal(tool.Function.Parameters)
			sb.WriteString(fmt.Sprintf("Parameters:\n```json\n%s\n```\n", string(paramsJSON)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// parseClaudeCliResponse parses the JSON output from the claude CLI.
func (p *ClaudeCliProvider) parseClaudeCliResponse(output string) (*LLMResponse, error) {
	var resp claudeCliJSONResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse claude cli response: %w", err)
	}

	if resp.IsError {
		return nil, fmt.Errorf("claude cli returned error: %s", resp.Result)
	}

	toolCalls := p.extractToolCalls(resp.Result)

	finishReason := "stop"
	content := resp.Result
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
		content = p.stripToolCallsJSON(resp.Result)
		content = p.stripTaggedToolCalls(content)
	}

	var usage *UsageInfo
	if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
		usage = &UsageInfo{
			PromptTokens:     resp.Usage.InputTokens + resp.Usage.CacheCreationInputTokens + resp.Usage.CacheReadInputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.CacheCreationInputTokens + resp.Usage.CacheReadInputTokens + resp.Usage.OutputTokens,
		}
	}

	return &LLMResponse{
		Content:      strings.TrimSpace(content),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}

// extractToolCalls parses tool call JSON from the response text.
func (p *ClaudeCliProvider) extractToolCalls(text string) []ToolCall {
	start := strings.Index(text, `{"tool_calls"`)
	if start != -1 {
		end := findMatchingBrace(text, start)
		if end == start {
			return nil
		}

		jsonStr := text[start:end]

		var wrapper struct {
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &wrapper); err == nil {
			var result []ToolCall
			for _, tc := range wrapper.ToolCalls {
				var args map[string]interface{}
				json.Unmarshal([]byte(tc.Function.Arguments), &args)

				result = append(result, ToolCall{
					ID:        tc.ID,
					Type:      tc.Type,
					Name:      tc.Function.Name,
					Arguments: args,
					Function: &FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
			return result
		}
	}

	// Fallback: support XML-like tool syntax such as:
	// <append_file>{"path":"...","content":"..."}</append_file>
	return p.extractTaggedToolCalls(text)
}

// stripToolCallsJSON removes tool call JSON from response text.
func (p *ClaudeCliProvider) stripToolCallsJSON(text string) string {
	start := strings.Index(text, `{"tool_calls"`)
	if start == -1 {
		return text
	}

	end := findMatchingBrace(text, start)
	if end == start {
		return text
	}

	return strings.TrimSpace(text[:start] + text[end:])
}

type taggedToolCallSegment struct {
	start int
	end   int
	call  ToolCall
}

func (p *ClaudeCliProvider) parseTaggedToolCallSegments(text string) []taggedToolCallSegment {
	var segments []taggedToolCallSegment
	i := 0
	for i < len(text) {
		rel := strings.IndexByte(text[i:], '<')
		if rel == -1 {
			break
		}
		start := i + rel
		if start+1 >= len(text) {
			break
		}

		// Skip closing tags or invalid tag starts.
		if text[start+1] == '/' || !isToolTagNameStart(text[start+1]) {
			i = start + 1
			continue
		}

		nameStart := start + 1
		nameEnd := nameStart
		for nameEnd < len(text) && isToolTagNameChar(text[nameEnd]) {
			nameEnd++
		}
		if nameEnd >= len(text) || text[nameEnd] != '>' {
			i = start + 1
			continue
		}
		name := text[nameStart:nameEnd]

		jsonStart := nameEnd + 1
		for jsonStart < len(text) && isWhitespaceByte(text[jsonStart]) {
			jsonStart++
		}
		if jsonStart >= len(text) || text[jsonStart] != '{' {
			i = start + 1
			continue
		}

		jsonEnd := findMatchingBrace(text, jsonStart)
		if jsonEnd == jsonStart {
			i = start + 1
			continue
		}
		rawArgs := text[jsonStart:jsonEnd]
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			i = start + 1
			continue
		}

		segmentEnd := jsonEnd
		afterJSON := jsonEnd
		for afterJSON < len(text) && isWhitespaceByte(text[afterJSON]) {
			afterJSON++
		}
		closing := "</" + name + ">"
		if strings.HasPrefix(text[afterJSON:], closing) {
			segmentEnd = afterJSON + len(closing)
		}

		segments = append(segments, taggedToolCallSegment{
			start: start,
			end:   segmentEnd,
			call: ToolCall{
				ID:        fmt.Sprintf("call_tag_%d", len(segments)+1),
				Type:      "function",
				Name:      name,
				Arguments: args,
				Function: &FunctionCall{
					Name:      name,
					Arguments: rawArgs,
				},
			},
		})
		i = segmentEnd
	}
	return segments
}

func (p *ClaudeCliProvider) extractTaggedToolCalls(text string) []ToolCall {
	segments := p.parseTaggedToolCallSegments(text)
	if len(segments) == 0 {
		return nil
	}

	result := make([]ToolCall, 0, len(segments))
	for _, seg := range segments {
		result = append(result, seg.call)
	}
	return result
}

func (p *ClaudeCliProvider) stripTaggedToolCalls(text string) string {
	segments := p.parseTaggedToolCallSegments(text)
	if len(segments) == 0 {
		return strings.TrimSpace(text)
	}

	var sb strings.Builder
	last := 0
	for _, seg := range segments {
		if seg.start > last {
			sb.WriteString(text[last:seg.start])
		}
		last = seg.end
	}
	if last < len(text) {
		sb.WriteString(text[last:])
	}

	return strings.TrimSpace(sb.String())
}

func isWhitespaceByte(b byte) bool {
	return b == ' ' || b == '\n' || b == '\t' || b == '\r'
}

func isToolTagNameStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

func isToolTagNameChar(b byte) bool {
	return isToolTagNameStart(b) || (b >= '0' && b <= '9')
}

// findMatchingBrace finds the index after the closing brace matching the opening brace at pos.
func findMatchingBrace(text string, pos int) int {
	depth := 0
	inString := false
	escaped := false
	for i := pos; i < len(text); i++ {
		ch := text[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return pos
}

// claudeCliJSONResponse represents the JSON output from the claude CLI.
// Matches the real claude CLI v2.x output format.
type claudeCliJSONResponse struct {
	Type         string             `json:"type"`
	Subtype      string             `json:"subtype"`
	IsError      bool               `json:"is_error"`
	Result       string             `json:"result"`
	SessionID    string             `json:"session_id"`
	TotalCostUSD float64            `json:"total_cost_usd"`
	DurationMS   int                `json:"duration_ms"`
	DurationAPI  int                `json:"duration_api_ms"`
	NumTurns     int                `json:"num_turns"`
	Usage        claudeCliUsageInfo `json:"usage"`
}

// claudeCliUsageInfo represents token usage from the claude CLI response.
type claudeCliUsageInfo struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}
