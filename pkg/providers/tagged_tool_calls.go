package providers

import (
	"encoding/json"
	"fmt"
	"strings"
)

type taggedToolCallSpan struct {
	start int
	end   int
	call  ToolCall
}

// ExtractTaggedToolCalls parses XML-like inline tool calls from text.
// Example: <append_file>{"path":"/tmp/a","content":"x"}</append_file>
func ExtractTaggedToolCalls(text string) []ToolCall {
	spans := parseTaggedToolCallSpans(text)
	if len(spans) == 0 {
		return nil
	}

	result := make([]ToolCall, 0, len(spans))
	for _, span := range spans {
		result = append(result, span.call)
	}
	return result
}

// StripTaggedToolCalls removes XML-like inline tool calls from text.
func StripTaggedToolCalls(text string) string {
	spans := parseTaggedToolCallSpans(text)
	if len(spans) == 0 {
		return strings.TrimSpace(text)
	}

	var sb strings.Builder
	last := 0
	for _, span := range spans {
		if span.start > last {
			sb.WriteString(text[last:span.start])
		}
		last = span.end
	}
	if last < len(text) {
		sb.WriteString(text[last:])
	}
	return strings.TrimSpace(sb.String())
}

func parseTaggedToolCallSpans(text string) []taggedToolCallSpan {
	var spans []taggedToolCallSpan
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

		if text[start+1] == '/' || !ptcIsTagNameStart(text[start+1]) {
			i = start + 1
			continue
		}

		nameStart := start + 1
		nameEnd := nameStart
		for nameEnd < len(text) && ptcIsTagNameChar(text[nameEnd]) {
			nameEnd++
		}
		if nameEnd >= len(text) || text[nameEnd] != '>' {
			i = start + 1
			continue
		}
		name := text[nameStart:nameEnd]

		jsonStart := nameEnd + 1
		for jsonStart < len(text) && ptcIsWhitespace(text[jsonStart]) {
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

		spanEnd := jsonEnd
		afterJSON := jsonEnd
		for afterJSON < len(text) && ptcIsWhitespace(text[afterJSON]) {
			afterJSON++
		}
		closingTag := "</" + name + ">"
		if strings.HasPrefix(text[afterJSON:], closingTag) {
			spanEnd = afterJSON + len(closingTag)
		}

		spans = append(spans, taggedToolCallSpan{
			start: start,
			end:   spanEnd,
			call: ToolCall{
				ID:        fmt.Sprintf("call_tag_%d", len(spans)+1),
				Type:      "function",
				Name:      name,
				Arguments: args,
				Function: &FunctionCall{
					Name:      name,
					Arguments: rawArgs,
				},
			},
		})
		i = spanEnd
	}
	return spans
}

func ptcIsWhitespace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t'
}

func ptcIsTagNameStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

func ptcIsTagNameChar(b byte) bool {
	return ptcIsTagNameStart(b) || (b >= '0' && b <= '9')
}
