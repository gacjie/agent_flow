package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// searchTavily 通过 Tavily API 搜索（有 Key 模式）
func (t *WebSearchTool) searchTavily(ctx context.Context, query string, maxResults int) ([]webSearchResult, error) {
	apiKey := ""
	if t.ConfigGetter != nil {
		apiKey = t.ConfigGetter.Get("search.tavily_api_key")
	}
	return t.doTavilySearch(ctx, query, maxResults, apiKey)
}

// searchTavilyKeyless 通过 Tavily API 搜索（keyless 模式）
func (t *WebSearchTool) searchTavilyKeyless(ctx context.Context, query string, maxResults int) ([]webSearchResult, error) {
	return t.doTavilySearch(ctx, query, maxResults, "")
}

func (t *WebSearchTool) doTavilySearch(ctx context.Context, query string, maxResults int, apiKey string) ([]webSearchResult, error) {
	payload := map[string]any{
		"query":        query,
		"max_results":  maxResults,
		"search_depth": "basic",
	}
	if apiKey != "" {
		payload["api_key"] = apiKey
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tavily 返回 %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var parsed struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	var results []webSearchResult
	for _, r := range parsed.Results {
		if len(results) >= maxResults {
			break
		}
		if r.URL != "" && r.Title != "" {
			results = append(results, webSearchResult{
				Title:   r.Title,
				URL:     r.URL,
				Content: r.Content,
			})
		}
	}
	return results, nil
}

// searchSerper 通过 Serper.dev 搜索（Google 结果）
func (t *WebSearchTool) searchSerper(ctx context.Context, query string, maxResults int) ([]webSearchResult, error) {
	apiKey := ""
	if t.ConfigGetter != nil {
		apiKey = t.ConfigGetter.Get("search.serper_api_key")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 Serper API Key")
	}

	payload := map[string]any{
		"q":   query,
		"num": maxResults,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://google.serper.dev/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Serper 返回 %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var parsed struct {
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	var results []webSearchResult
	for _, r := range parsed.Organic {
		if len(results) >= maxResults {
			break
		}
		if r.Link != "" && r.Title != "" {
			results = append(results, webSearchResult{
				Title:   r.Title,
				URL:     r.Link,
				Content: r.Snippet,
			})
		}
	}
	return results, nil
}

// searchGoogle 通过 Google Custom Search API 搜索
func (t *WebSearchTool) searchGoogle(ctx context.Context, query string, maxResults int) ([]webSearchResult, error) {
	var apiKey, cx string
	if t.ConfigGetter != nil {
		apiKey = t.ConfigGetter.Get("search.google_api_key")
		cx = t.ConfigGetter.Get("search.google_cx")
	}
	if apiKey == "" || cx == "" {
		return nil, fmt.Errorf("未配置 Google API Key 或 CX")
	}

	params := url.Values{}
	params.Set("key", apiKey)
	params.Set("cx", cx)
	params.Set("q", query)
	params.Set("num", fmt.Sprintf("%d", maxResults))

	reqURL := "https://www.googleapis.com/customsearch/v1?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google 返回 %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var parsed struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	var results []webSearchResult
	for _, r := range parsed.Items {
		if len(results) >= maxResults {
			break
		}
		if r.Link != "" && r.Title != "" {
			results = append(results, webSearchResult{
				Title:   r.Title,
				URL:     r.Link,
				Content: r.Snippet,
			})
		}
	}
	return results, nil
}

// searchBrave 通过 Brave Search API 搜索
func (t *WebSearchTool) searchBrave(ctx context.Context, query string, maxResults int) ([]webSearchResult, error) {
	apiKey := ""
	if t.ConfigGetter != nil {
		apiKey = t.ConfigGetter.Get("search.brave_api_key")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 Brave Search API Key")
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("count", fmt.Sprintf("%d", maxResults))

	reqURL := "https://api.search.brave.com/res/v1/web/search?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Subscription-Token", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Brave 返回 %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var parsed struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	var results []webSearchResult
	for _, r := range parsed.Web.Results {
		if len(results) >= maxResults {
			break
		}
		if r.URL != "" && r.Title != "" {
			results = append(results, webSearchResult{
				Title:   r.Title,
				URL:     r.URL,
				Content: r.Description,
			})
		}
	}
	return results, nil
}

// searchExa 通过 Exa API 搜索（语义搜索）
func (t *WebSearchTool) searchExa(ctx context.Context, query string, maxResults int) ([]webSearchResult, error) {
	apiKey := ""
	if t.ConfigGetter != nil {
		apiKey = t.ConfigGetter.Get("search.exa_api_key")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 Exa API Key")
	}

	payload := map[string]any{
		"query":       query,
		"num_results": maxResults,
		"type":        "keyword",
		"contents": map[string]any{
			"text": true,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.exa.ai/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Exa 返回 %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var parsed struct {
		Results []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
			Text  string `json:"text"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	var results []webSearchResult
	for _, r := range parsed.Results {
		if len(results) >= maxResults {
			break
		}
		snippet := r.Text
		if len([]rune(snippet)) > 200 {
			snippet = string([]rune(snippet)[:200]) + "..."
		}
		if r.URL != "" && r.Title != "" {
			results = append(results, webSearchResult{
				Title:   r.Title,
				URL:     r.URL,
				Content: snippet,
			})
		}
	}
	return results, nil
}

// searchBing 通过 Bing HTML 页面搜索
func (t *WebSearchTool) searchBing(ctx context.Context, query string, maxResults int) ([]webSearchResult, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("count", fmt.Sprintf("%d", maxResults))

	reqURL := "https://www.bing.com/search?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bing 返回 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	return parseBingHTML(string(body), maxResults), nil
}

// parseBingHTML 从 Bing HTML 响应中提取搜索结果
func parseBingHTML(html string, maxResults int) []webSearchResult {
	var results []webSearchResult
	remaining := html

	for len(results) < maxResults {
		idx := strings.Index(remaining, `class="b_algo"`)
		if idx == -1 {
			break
		}
		remaining = remaining[idx:]

		h2Idx := strings.Index(remaining, "<h2")
		if h2Idx == -1 {
			remaining = remaining[14:]
			continue
		}
		block := remaining[h2Idx:]

		link := extractAttr(block, "href")
		if link == "" || !strings.HasPrefix(link, "http") {
			remaining = remaining[14:]
			continue
		}

		aStart := strings.Index(block, "<a ")
		title := ""
		if aStart != -1 {
			title = stripHTML(extractTagText(block[aStart:]))
		}

		snippet := ""
		capIdx := strings.Index(remaining, `class="b_caption"`)
		if capIdx == -1 {
			capIdx = strings.Index(remaining, `<p`)
		}
		if capIdx != -1 && capIdx < 3000 {
			pIdx := strings.Index(remaining[capIdx:], "<p")
			if pIdx != -1 {
				pBlock := remaining[capIdx+pIdx:]
				pEnd := strings.Index(pBlock, "</p>")
				if pEnd != -1 && pEnd < 1000 {
					snippet = stripHTML(pBlock[:pEnd])
				}
			}
		}

		if link != "" && title != "" {
			results = append(results, webSearchResult{
				Title:   title,
				URL:     link,
				Content: snippet,
			})
		}

		remaining = remaining[14:]
	}

	return results
}

// searchSearXNG 通过 SearXNG 实例搜索（JSON API）
func (t *WebSearchTool) searchSearXNG(ctx context.Context, query string, maxResults int) ([]webSearchResult, error) {
	baseURL := ""
	if t.ConfigGetter != nil {
		baseURL = t.ConfigGetter.Get("search.searxng_url")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("未配置 SearXNG 实例地址")
	}
	baseURL = strings.TrimRight(baseURL, "/")

	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("language", "auto")

	reqURL := baseURL + "/search?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("返回 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var parsed struct {
		Results []struct {
			URL     string `json:"url"`
			Title   string `json:"title"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	var results []webSearchResult
	for _, r := range parsed.Results {
		if len(results) >= maxResults {
			break
		}
		if r.URL != "" && r.Title != "" {
			results = append(results, webSearchResult{
				Title:   r.Title,
				URL:     r.URL,
				Content: r.Content,
			})
		}
	}
	return results, nil
}

// searchJina 通过 Jina AI 搜索
func (t *WebSearchTool) searchJina(ctx context.Context, query string, maxResults int) ([]webSearchResult, error) {
	reqURL := "https://s.jina.ai/" + url.PathEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	if t.ConfigGetter != nil {
		if key := t.ConfigGetter.Get("search.jina_api_key"); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("返回 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	return parseJinaResponse(body, maxResults)
}

// parseJinaResponse 解析 Jina 搜索响应
func parseJinaResponse(body []byte, maxResults int) ([]webSearchResult, error) {
	var jsonResp struct {
		Data []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			Content     string `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &jsonResp); err == nil && len(jsonResp.Data) > 0 {
		var results []webSearchResult
		for _, item := range jsonResp.Data {
			if len(results) >= maxResults {
				break
			}
			snippet := item.Description
			if snippet == "" {
				snippet = item.Content
				if len([]rune(snippet)) > 200 {
					snippet = string([]rune(snippet)[:200]) + "..."
				}
			}
			if item.URL != "" && item.Title != "" {
				results = append(results, webSearchResult{
					Title:   item.Title,
					URL:     item.URL,
					Content: snippet,
				})
			}
		}
		return results, nil
	}

	return parseJinaMarkdown(string(body), maxResults), nil
}

// parseJinaMarkdown 从 Jina 的 Markdown 响应中提取结果
func parseJinaMarkdown(text string, maxResults int) []webSearchResult {
	var results []webSearchResult
	lines := strings.Split(text, "\n")

	for i := 0; i < len(lines) && len(results) < maxResults; i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "[") {
			continue
		}
		closeBracket := strings.Index(line, "](")
		if closeBracket == -1 {
			continue
		}
		title := line[1:closeBracket]
		rest := line[closeBracket+2:]
		closeParen := strings.Index(rest, ")")
		if closeParen == -1 {
			continue
		}
		link := rest[:closeParen]
		if !strings.HasPrefix(link, "http") {
			continue
		}

		snippet := ""
		if i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if next != "" && !strings.HasPrefix(next, "[") && !strings.HasPrefix(next, "#") {
				snippet = next
			}
		}

		results = append(results, webSearchResult{
			Title:   title,
			URL:     link,
			Content: snippet,
		})
	}
	return results
}

// searchDuckDuckGo 通过 DuckDuckGo HTML 页面搜索（兜底）
func (t *WebSearchTool) searchDuckDuckGo(ctx context.Context, query string, maxResults int) ([]webSearchResult, error) {
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
