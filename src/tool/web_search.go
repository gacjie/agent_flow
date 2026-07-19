package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// WebSearchTool 网络搜索工具（支持批量，多引擎降级）
type WebSearchTool struct {
	ConfigGetter ConfigGetter
}

type SearchEngineOption struct {
	Value string
	Label string
}

var SearchEngineOptions = []SearchEngineOption{
	{"auto", "自动切换（按优先级尝试已配置引擎）"},
	{"serper", "Serper（Google）"},
	{"google", "Google Custom Search"},
	{"brave", "Brave Search"},
	{"exa", "Exa"},
	{"tavily", "Tavily"},
	{"jina", "Jina AI"},
	{"searxng", "SearXNG（自建实例）"},
	{"bing", "Bing（HTML）"},
	{"duckduckgo", "DuckDuckGo"},
}

type searchFunc func(ctx context.Context, query string, maxResults int) ([]webSearchResult, error)
type engineEntry struct {
	name string
	fn   searchFunc
}

type webSearchResult struct {
	Title   string
	URL     string
	Content string
}

func (t *WebSearchTool) Name() string { return "web_searches" }

func (t *WebSearchTool) Description() string {
	return "在互联网上搜索信息，支持多引擎（Tavily/Serper/Google/Brave/Exa/Jina/SearXNG/Bing/DuckDuckGo）自动切换，并行执行（限制并发 ≤3），返回相关网页的标题、链接和摘要。"
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
		Queries []struct {
			Query      string `json:"query"`
			MaxResults int    `json:"max_results"`
		} `json:"queries"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}
	if len(params.Queries) == 0 {
		return ErrorResult("queries 参数不能为空（至少提供一项）")
	}
	for i, q := range params.Queries {
		if q.Query == "" {
			return ErrorResult(fmt.Sprintf("queries[%d].query 不能为空", i))
		}
	}

	n := len(params.Queries)
	results := make([]string, n)

	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for i, q := range params.Queries {
		wg.Add(1)
		go func(idx int, query string, maxResults int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res := t.searchSingle(ctx, query, maxResults)
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

func (t *WebSearchTool) searchSingle(ctx context.Context, query string, maxResults int) *Result {
	if query == "" {
		return ErrorResult("搜索关键词不能为空")
	}

	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 20 {
		maxResults = 20
	}

	httpCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	results, err := t.searchWithFallback(httpCtx, query, maxResults)
	if err != nil {
		return ErrorResult("搜索失败: " + err.Error())
	}

	if len(results) == 0 {
		return SuccessResult("未找到相关结果")
	}

	return SuccessResult(formatSearchResults(results))
}

// searchWithFallback 根据配置选择引擎并按优先级降级
func (t *WebSearchTool) searchWithFallback(ctx context.Context, query string, maxResults int) ([]webSearchResult, error) {
	engine := "auto"
	if t.ConfigGetter != nil {
		if v := t.ConfigGetter.Get("search.engine"); v != "" {
			engine = v
		}
	}

	var engines []engineEntry
	switch engine {
	case "searxng":
		engines = []engineEntry{{"SearXNG", t.searchSearXNG}}
	case "jina":
		engines = []engineEntry{{"Jina", t.searchJina}}
	case "duckduckgo":
		engines = []engineEntry{{"DuckDuckGo", t.searchDuckDuckGo}}
	case "tavily":
		engines = []engineEntry{{"Tavily", t.searchTavily}}
	case "serper":
		engines = []engineEntry{{"Serper", t.searchSerper}}
	case "google":
		engines = []engineEntry{{"Google", t.searchGoogle}}
	case "brave":
		engines = []engineEntry{{"Brave", t.searchBrave}}
	case "exa":
		engines = []engineEntry{{"Exa", t.searchExa}}
	case "bing":
		engines = []engineEntry{{"Bing", t.searchBing}}
	default: // auto
		engines = t.buildAutoEngines()
	}

	var lastErr error
	for _, eng := range engines {
		engineCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		results, err := eng.fn(engineCtx, query, maxResults)
		cancel()

		if err == nil && len(results) > 0 {
			return results, nil
		}
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", eng.name, err)
			slog.Debug("搜索引擎失败，尝试下一个", "engine", eng.name, "error", err)
		} else {
			lastErr = fmt.Errorf("%s: 无搜索结果", eng.name)
			slog.Debug("搜索引擎无结果，尝试下一个", "engine", eng.name)
		}
	}
	return nil, lastErr
}

// buildAutoEngines 按优先级构建 auto 模式引擎列表
func (t *WebSearchTool) buildAutoEngines() []engineEntry {
	var engines []engineEntry

	// 1. Tavily keyless — 零配置可用，始终作为第一选择
	engines = append(engines, engineEntry{"Tavily(keyless)", t.searchTavilyKeyless})

	// 2-7. 配置了 Key 的引擎按优先级加入
	if t.ConfigGetter != nil {
		if key := t.ConfigGetter.Get("search.serper_api_key"); key != "" {
			engines = append(engines, engineEntry{"Serper", t.searchSerper})
		}
		if key := t.ConfigGetter.Get("search.google_api_key"); key != "" {
			if cx := t.ConfigGetter.Get("search.google_cx"); cx != "" {
				engines = append(engines, engineEntry{"Google", t.searchGoogle})
			}
		}
		if key := t.ConfigGetter.Get("search.brave_api_key"); key != "" {
			engines = append(engines, engineEntry{"Brave", t.searchBrave})
		}
		if key := t.ConfigGetter.Get("search.exa_api_key"); key != "" {
			engines = append(engines, engineEntry{"Exa", t.searchExa})
		}
		if key := t.ConfigGetter.Get("search.tavily_api_key"); key != "" {
			engines = append(engines, engineEntry{"Tavily", t.searchTavily})
		}
		if key := t.ConfigGetter.Get("search.jina_api_key"); key != "" {
			engines = append(engines, engineEntry{"Jina", t.searchJina})
		}
		// 8. SearXNG 仅在配置了自建实例 URL 时启用
		if u := t.ConfigGetter.Get("search.searxng_url"); u != "" {
			engines = append(engines, engineEntry{"SearXNG", t.searchSearXNG})
		}
	}

	// 9-10. HTML 兜底（始终可用）
	engines = append(engines, engineEntry{"Bing", t.searchBing})
	engines = append(engines, engineEntry{"DuckDuckGo", t.searchDuckDuckGo})

	return engines
}

// --- 通用辅助函数 ---

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

// httpGet 通用 GET 请求辅助
func httpGet(ctx context.Context, reqURL string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return http.DefaultClient.Do(req)
}
