package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// WebSearchTool 网络搜索工具（支持批量）
type WebSearchTool struct{}

func (t *WebSearchTool) Name() string { return "web_searches" }

func (t *WebSearchTool) Description() string {
	return "在互联网上搜索信息（DuckDuckGo），并行执行（限制并发 ≤3 避免限流），返回相关网页的标题、链接和摘要。"
}

func (t *WebSearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"queries": {
				"type": "array",
				"description": "搜索配置列表，并行执行（限并发 ≤3）",
				"items": {
					"type": "object",
					"properties": {
						"query": {"type": "string", "description": "搜索关键词"},
						"max_results": {"type": "integer", "description": "最大返回结果数（默认 5，最大 20）"}
					},
					"required": ["query"]
				}
			}
		},
		"required": ["queries"]
	}`)
}

func (t *WebSearchTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
		Queries    []struct {
			Query      string `json:"query"`
			MaxResults int    `json:"max_results"`
		} `json:"queries"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	// 批量模式
	if len(params.Queries) > 0 {
		n := len(params.Queries)
		results := make([]string, n)

		// 信号量限制并发数 ≤3，避免 DuckDuckGo 限流
		sem := make(chan struct{}, 3)
		var wg sync.WaitGroup
		for i, q := range params.Queries {
			wg.Add(1)
			go func(idx int, query string, maxResults int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				res := searchSingle(ctx, query, maxResults)
				if res.IsError {
					results[idx] = fmt.Sprintf("=== [%d/%d] 搜索: %q（错误）===\n%s", idx+1, n, query, res.Content)
				} else {
					results[idx] = fmt.Sprintf("=== [%d/%d] 搜索: %q ===\n%s", idx+1, n, query, res.Content)
				}
			}(i, q.Query, q.MaxResults)
		}
		wg.Wait()
		return SuccessResult(strings.Join(results, "\n\n"))
	}

	// 单次搜索模式
	return searchSingle(ctx, params.Query, params.MaxResults)
}

func searchSingle(ctx context.Context, query string, maxResults int) *Result {
	if query == "" {
		return ErrorResult("搜索关键词不能为空")
	}

	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 20 {
		maxResults = 20
	}

	httpCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	results, err := searchDuckDuckGo(httpCtx, query, maxResults)
	if err != nil {
		return ErrorResult("搜索失败: " + err.Error())
	}

	if len(results) == 0 {
		return SuccessResult("未找到相关结果")
	}

	return SuccessResult(formatSearchResults(results))
}

// webSearchResult 搜索结果
type webSearchResult struct {
	Title   string
	URL     string
	Content string
}

// searchDuckDuckGo 通过 DuckDuckGo HTML 页面搜索
func searchDuckDuckGo(ctx context.Context, query string, maxResults int) ([]webSearchResult, error) {
	form := url.Values{}
	form.Set("q", query)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://html.duckduckgo.com/html/", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DuckDuckGo 返回 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	return parseDDGHTML(string(body), maxResults), nil
}

// parseDDGHTML 从 DuckDuckGo HTML 响应中提取搜索结果
func parseDDGHTML(html string, maxResults int) []webSearchResult {
	var results []webSearchResult
	remaining := html

	for len(results) < maxResults {
		idx := strings.Index(remaining, `class="result__a"`)
		if idx == -1 {
			break
		}
		remaining = remaining[idx:]

		link := extractAttr(remaining, "href")
		if link == "" {
			remaining = remaining[17:]
			continue
		}

		link = extractRealURL(link)
		title := extractTagText(remaining)

		snippetIdx := strings.Index(remaining, `class="result__snippet"`)
		snippet := ""
		if snippetIdx != -1 && snippetIdx < 2000 {
			snippet = stripHTML(extractTagText(remaining[snippetIdx:]))
		}

		if link != "" && title != "" {
			results = append(results, webSearchResult{
				Title:   stripHTML(title),
				URL:     link,
				Content: snippet,
			})
		}

		remaining = remaining[17:]
	}

	return results
}

func extractAttr(s, attr string) string {
	search := attr + `="`
	idx := strings.Index(s, search)
	if idx == -1 {
		return ""
	}
	start := idx + len(search)
	end := strings.Index(s[start:], `"`)
	if end == -1 {
		return ""
	}
	return s[start : start+end]
}

func extractTagText(s string) string {
	gtIdx := strings.Index(s, ">")
	if gtIdx == -1 {
		return ""
	}
	s = s[gtIdx+1:]

	endIdx := strings.Index(s, "</a>")
	if endIdx == -1 {
		endIdx = strings.Index(s, "</span>")
	}
	if endIdx == -1 {
		endIdx = strings.Index(s, "</")
	}
	if endIdx == -1 || endIdx > 1000 {
		return ""
	}

	return s[:endIdx]
}

func extractRealURL(link string) string {
	if strings.Contains(link, "uddg=") {
		u, err := url.Parse(link)
		if err == nil {
			if uddg := u.Query().Get("uddg"); uddg != "" {
				return uddg
			}
		}
	}
	if strings.HasPrefix(link, "//") {
		return "https:" + link
	}
	return link
}

func stripHTML(s string) string {
	var result strings.Builder
	inTag := false
	for _, ch := range s {
		if ch == '<' {
			inTag = true
			continue
		}
		if ch == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(ch)
		}
	}
	return strings.TrimSpace(result.String())
}

func formatSearchResults(results []webSearchResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 条结果：\n\n", len(results)))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("链接: %s\n", r.URL))
		if r.Content != "" {
			sb.WriteString(fmt.Sprintf("摘要: %s\n", r.Content))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
