package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ArXivTool searches for research papers on arXiv
type ArXivTool struct{}

func NewArXivTool() *ArXivTool {
	return &ArXivTool{}
}

func (t *ArXivTool) Name() string {
	return "arxiv_search"
}

func (t *ArXivTool) Description() string {
	return "Search arXiv for research papers. Returns paper titles, authors, abstracts, and PDF links. Use this for academic research and scientific papers."
}

func (t *ArXivTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query (e.g., 'machine learning', 'quantum computing')",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results to return (default: 5, max: 20)",
				"minimum":     1.0,
				"maximum":     20.0,
			},
		},
		"required": []string{"query"},
	}
}

func (t *ArXivTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return ErrorResult("query is required")
	}

	maxResults := 5
	if mr, ok := args["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
		if maxResults > 20 {
			maxResults = 20
		}
	}

	searchURL := fmt.Sprintf("http://export.arxiv.org/api/query?search_query=all:%s&start=0&max_results=%d",
		url.QueryEscape(query), maxResults)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create request: %v", err))
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("request failed: %v", err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to read response: %v", err))
	}

	results, err := t.parseArXivXML(string(body))
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to parse results: %v", err))
	}

	if len(results) == 0 {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("No results found on arXiv for: %s", query),
			ForUser: fmt.Sprintf("No results found on arXiv for: %s", query),
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("arXiv Results for: %s\n", query))

	for i, result := range results {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, result.Title))
		lines = append(lines, fmt.Sprintf("   Authors: %s", result.Authors))
		lines = append(lines, fmt.Sprintf("   Published: %s", result.Published))
		lines = append(lines, fmt.Sprintf("   Link: %s", result.Link))
		lines = append(lines, fmt.Sprintf("   PDF: %s", result.PDF))
		if result.Abstract != "" {
			abstract := result.Abstract
			if len(abstract) > 300 {
				abstract = abstract[:297] + "..."
			}
			lines = append(lines, fmt.Sprintf("   Abstract: %s", abstract))
		}
		lines = append(lines, "")
	}

	output := strings.Join(lines, "\n")
	return &ToolResult{
		ForLLM:  output,
		ForUser: output,
	}
}

type arxivResult struct {
	Title     string
	Authors   string
	Abstract  string
	Published string
	Link      string
	PDF       string
}

func (t *ArXivTool) parseArXivXML(xml string) ([]arxivResult, error) {
	var results []arxivResult

	// Simple XML parsing without external dependencies
	entries := strings.Split(xml, "<entry>")
	for i, entry := range entries {
		if i == 0 {
			continue // Skip header
		}

		result := arxivResult{}

		// Extract title
		if start := strings.Index(entry, "<title>"); start != -1 {
			start += 7
			if end := strings.Index(entry[start:], "</title>"); end != -1 {
				result.Title = strings.TrimSpace(entry[start : start+end])
				result.Title = strings.ReplaceAll(result.Title, "\n", " ")
			}
		}

		// Extract authors
		var authors []string
		authorEntries := strings.Split(entry, "<author>")
		for j, authorEntry := range authorEntries {
			if j == 0 {
				continue
			}
			if start := strings.Index(authorEntry, "<name>"); start != -1 {
				start += 6
				if end := strings.Index(authorEntry[start:], "</name>"); end != -1 {
					authors = append(authors, strings.TrimSpace(authorEntry[start:start+end]))
				}
			}
		}
		result.Authors = strings.Join(authors, ", ")

		// Extract abstract
		if start := strings.Index(entry, "<summary>"); start != -1 {
			start += 9
			if end := strings.Index(entry[start:], "</summary>"); end != -1 {
				result.Abstract = strings.TrimSpace(entry[start : start+end])
				result.Abstract = strings.ReplaceAll(result.Abstract, "\n", " ")
			}
		}

		// Extract published date
		if start := strings.Index(entry, "<published>"); start != -1 {
			start += 11
			if end := strings.Index(entry[start:], "</published>"); end != -1 {
				result.Published = strings.TrimSpace(entry[start : start+end])
			}
		}

		// Extract links
		linkEntries := strings.Split(entry, "<link ")
		for _, linkEntry := range linkEntries {
			if strings.Contains(linkEntry, `type="text/html"`) {
				if start := strings.Index(linkEntry, `href="`); start != -1 {
					start += 6
					if end := strings.Index(linkEntry[start:], `"`); end != -1 {
						result.Link = linkEntry[start : start+end]
					}
				}
			}
			if strings.Contains(linkEntry, `type="application/pdf"`) {
				if start := strings.Index(linkEntry, `href="`); start != -1 {
					start += 6
					if end := strings.Index(linkEntry[start:], `"`); end != -1 {
						result.PDF = linkEntry[start : start+end]
					}
				}
			}
		}

		if result.Title != "" {
			results = append(results, result)
		}
	}

	return results, nil
}

// CrossrefTool searches for paper metadata via Crossref API
type CrossrefTool struct{}

func NewCrossrefTool() *CrossrefTool {
	return &CrossrefTool{}
}

func (t *CrossrefTool) Name() string {
	return "crossref_search"
}

func (t *CrossrefTool) Description() string {
	return "Search Crossref for paper metadata, DOI lookup, and citation information. Use this for published academic papers and journal articles."
}

func (t *CrossrefTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query or DOI (e.g., '10.1038/nature12373' or 'deep learning')",
			},
			"rows": map[string]interface{}{
				"type":        "integer",
				"description": "Number of results to return (default: 5, max: 20)",
				"minimum":     1.0,
				"maximum":     20.0,
			},
		},
		"required": []string{"query"},
	}
}

func (t *CrossrefTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return ErrorResult("query is required")
	}

	rows := 5
	if r, ok := args["rows"].(float64); ok && r > 0 {
		rows = int(r)
		if rows > 20 {
			rows = 20
		}
	}

	// Check if query is a DOI
	var searchURL string
	if strings.HasPrefix(query, "10.") {
		// DOI lookup
		searchURL = fmt.Sprintf("https://api.crossref.org/works/%s", url.PathEscape(query))
	} else {
		// General search
		searchURL = fmt.Sprintf("https://api.crossref.org/works?query=%s&rows=%d",
			url.QueryEscape(query), rows)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create request: %v", err))
	}

	req.Header.Set("User-Agent", "Summer AI Assistant (mailto:research@example.com)")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("request failed: %v", err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to read response: %v", err))
	}

	if resp.StatusCode != http.StatusOK {
		return ErrorResult(fmt.Sprintf("API error: %d - %s", resp.StatusCode, string(body)))
	}

	var apiResp struct {
		Message struct {
			Items []struct {
				DOI       string   `json:"DOI"`
				Title     []string `json:"title"`
				Author    []struct {
					Given  string `json:"given"`
					Family string `json:"family"`
				} `json:"author"`
				Published struct {
					DateParts [][]int `json:"date-parts"`
				} `json:"published-print"`
				URL              string `json:"URL"`
				ContainerTitle   []string `json:"container-title"`
				IsReferencedByCount int `json:"is-referenced-by-count"`
			} `json:"items"`
		} `json:"message"`
	}

	// Handle single DOI response
	var singleItemResp struct {
		Message struct {
			DOI       string   `json:"DOI"`
			Title     []string `json:"title"`
			Author    []struct {
				Given  string `json:"given"`
				Family string `json:"family"`
			} `json:"author"`
			Published struct {
				DateParts [][]int `json:"date-parts"`
			} `json:"published-print"`
			URL              string   `json:"URL"`
			ContainerTitle   []string `json:"container-title"`
			IsReferencedByCount int `json:"is-referenced-by-count"`
		} `json:"message"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		// Try single item response
		if err := json.Unmarshal(body, &singleItemResp); err != nil {
			return ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
		}
		// Convert single item to items array
		apiResp.Message.Items = []struct {
			DOI       string   `json:"DOI"`
			Title     []string `json:"title"`
			Author    []struct {
				Given  string `json:"given"`
				Family string `json:"family"`
			} `json:"author"`
			Published struct {
				DateParts [][]int `json:"date-parts"`
			} `json:"published-print"`
			URL              string   `json:"URL"`
			ContainerTitle   []string   `json:"container-title"`
			IsReferencedByCount int `json:"is-referenced-by-count"`
		}{
			{
				DOI:       singleItemResp.Message.DOI,
				Title:     singleItemResp.Message.Title,
				Author:    singleItemResp.Message.Author,
				Published: singleItemResp.Message.Published,
				URL:       singleItemResp.Message.URL,
				ContainerTitle: singleItemResp.Message.ContainerTitle,
				IsReferencedByCount: singleItemResp.Message.IsReferencedByCount,
			},
		}
	}

	items := apiResp.Message.Items
	if len(items) == 0 {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("No results found on Crossref for: %s", query),
			ForUser: fmt.Sprintf("No results found on Crossref for: %s", query),
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Crossref Results for: %s\n", query))

	for i, item := range items {
		title := "Untitled"
		if len(item.Title) > 0 {
			title = item.Title[0]
		}

		var authors []string
		for _, author := range item.Author {
			authors = append(authors, fmt.Sprintf("%s %s", author.Given, author.Family))
		}
		authorStr := strings.Join(authors, ", ")
		if authorStr == "" {
			authorStr = "Unknown"
		}

		year := "Unknown"
		if len(item.Published.DateParts) > 0 && len(item.Published.DateParts[0]) > 0 {
			year = fmt.Sprintf("%d", item.Published.DateParts[0][0])
		}

		journal := ""
		if len(item.ContainerTitle) > 0 {
			journal = item.ContainerTitle[0]
		}

		lines = append(lines, fmt.Sprintf("%d. %s", i+1, title))
		lines = append(lines, fmt.Sprintf("   Authors: %s", authorStr))
		if journal != "" {
			lines = append(lines, fmt.Sprintf("   Journal: %s", journal))
		}
		lines = append(lines, fmt.Sprintf("   Year: %s", year))
		lines = append(lines, fmt.Sprintf("   DOI: %s", item.DOI))
		if item.URL != "" {
			lines = append(lines, fmt.Sprintf("   URL: %s", item.URL))
		}
		if item.IsReferencedByCount > 0 {
			lines = append(lines, fmt.Sprintf("   Citations: %d", item.IsReferencedByCount))
		}
		lines = append(lines, "")
	}

	output := strings.Join(lines, "\n")
	return &ToolResult{
		ForLLM:  output,
		ForUser: output,
	}
}

// DuckDuckGoInstantAnswerTool uses DDG Instant Answer API
type DuckDuckGoInstantAnswerTool struct{}

func NewDuckDuckGoInstantAnswerTool() *DuckDuckGoInstantAnswerTool {
	return &DuckDuckGoInstantAnswerTool{}
}

func (t *DuckDuckGoInstantAnswerTool) Name() string {
	return "ddg_instant_answer"
}

func (t *DuckDuckGoInstantAnswerTool) Description() string {
	return "Get instant answers from DuckDuckGo for quick facts, definitions, calculations, and summaries. Great for quick information lookup."
}

func (t *DuckDuckGoInstantAnswerTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Query for instant answer (e.g., 'what is photosynthesis', 'define quantum', 'weather london')",
			},
		},
		"required": []string{"query"},
	}
}

func (t *DuckDuckGoInstantAnswerTool) Execute(ctx context.Context, args map[string]interface{}) *ToolResult {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return ErrorResult("query is required")
	}

	searchURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1",
		url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to create request: %v", err))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrorResult(fmt.Sprintf("request failed: %v", err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to read response: %v", err))
	}

	var result struct {
		Abstract       string `json:"Abstract"`
		AbstractText   string `json:"AbstractText"`
		AbstractSource string `json:"AbstractSource"`
		AbstractURL    string `json:"AbstractURL"`
		Answer         string `json:"Answer"`
		AnswerType     string `json:"AnswerType"`
		Definition     string `json:"Definition"`
		DefinitionURL  string `json:"DefinitionURL"`
		Heading        string `json:"Heading"`
		Image          string `json:"Image"`
		RelatedTopics  []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return ErrorResult(fmt.Sprintf("failed to parse response: %v", err))
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("DuckDuckGo Instant Answer for: %s\n", query))

	hasContent := false

	if result.Answer != "" {
		lines = append(lines, fmt.Sprintf("Answer: %s", result.Answer))
		hasContent = true
	}

	if result.AbstractText != "" {
		lines = append(lines, fmt.Sprintf("Summary: %s", result.AbstractText))
		if result.AbstractSource != "" {
			lines = append(lines, fmt.Sprintf("Source: %s", result.AbstractSource))
		}
		if result.AbstractURL != "" {
			lines = append(lines, fmt.Sprintf("URL: %s", result.AbstractURL))
		}
		hasContent = true
	}

	if result.Definition != "" {
		lines = append(lines, fmt.Sprintf("Definition: %s", result.Definition))
		if result.DefinitionURL != "" {
			lines = append(lines, fmt.Sprintf("URL: %s", result.DefinitionURL))
		}
		hasContent = true
	}

	if len(result.RelatedTopics) > 0 {
		lines = append(lines, "\nRelated Topics:")
		for i, topic := range result.RelatedTopics {
			if i >= 5 {
				break
			}
			if topic.Text != "" {
				lines = append(lines, fmt.Sprintf("• %s", topic.Text))
				if topic.FirstURL != "" {
					lines = append(lines, fmt.Sprintf("  %s", topic.FirstURL))
				}
			}
		}
		hasContent = true
	}

	if !hasContent {
		return &ToolResult{
			ForLLM:  fmt.Sprintf("No instant answer available for: %s. Try using web_search for broader results.", query),
			ForUser: fmt.Sprintf("No instant answer available for: %s", query),
		}
	}

	output := strings.Join(lines, "\n")
	return &ToolResult{
		ForLLM:  output,
		ForUser: output,
	}
}
