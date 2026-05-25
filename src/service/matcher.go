package service

import (
	"sort"
	"strings"

	"agent_flow/src/model"
)

// ContextMatcher 上下文匹配器
// 根据用户消息中的关键词，从文件索引中匹配相关上下文
type ContextMatcher struct {
	IndexerService *IndexerService
}

// NewContextMatcher 创建上下文匹配器
func NewContextMatcher(indexer *IndexerService) *ContextMatcher {
	return &ContextMatcher{IndexerService: indexer}
}

// MatchResult 匹配结果
type MatchResult struct {
	FilePath string  `json:"file_path"`
	Language string  `json:"language"`
	Summary  string  `json:"summary"`
	Score    float64 `json:"score"` // 匹配得分
}

// Match 根据查询文本匹配相关文件
func (m *ContextMatcher) Match(workspaceID uint, query string, maxResults int) ([]MatchResult, error) {
	// 从查询中提取关键词
	queryKeywords := extractQueryKeywords(query)
	if len(queryKeywords) == 0 {
		return nil, nil
	}

	// 从索引中搜索
	indexes, err := m.IndexerService.SearchByKeywords(workspaceID, queryKeywords)
	if err != nil {
		return nil, err
	}

	if len(indexes) == 0 {
		return nil, nil
	}

	// 计算相关度得分并排序
	results := scoreAndRank(indexes, queryKeywords)

	// 限制返回数量
	if maxResults > 0 && len(results) > maxResults {
		results = results[:maxResults]
	}

	return results, nil
}

// FormatMatchContext 将匹配结果格式化为上下文文本
func FormatMatchContext(results []MatchResult) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("以下是与当前对话相关的项目文件：\n\n")

	for i, r := range results {
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")
		sb.WriteString("文件: " + r.FilePath)
		if r.Language != "" {
			sb.WriteString(" (" + r.Language + ")")
		}
		sb.WriteString("\n")
		if r.Summary != "" {
			sb.WriteString("摘要: " + r.Summary + "\n")
		}
		if i < len(results)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// --- 内部函数 ---

// extractQueryKeywords 从查询文本中提取关键词
func extractQueryKeywords(query string) []string {
	// 去除常见停用词，提取有意义的词
	stopWords := map[string]bool{
		"的": true, "了": true, "是": true, "在": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "个": true, "上": true, "也": true,
		"这": true, "中": true, "他": true, "会": true, "来": true,
		"到": true, "时": true, "要": true, "可": true, "她": true,
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "can": true, "shall": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "into": true,
		"that": true, "this": true, "it": true, "and": true, "or": true,
		"not": true, "but": true, "if": true, "then": true, "than": true,
		"请": true, "帮": true, "我要": true, "怎么": true, "如何": true,
		"什么": true, "吗": true, "呢": true, "吧": true, "啊": true,
	}

	var keywords []string
	seen := make(map[string]bool)

	// 按空格和标点分词
	punctuation := []string{
		"\uff0c", "\u3002", "\uff1f", "\uff01", // ，。？！
		"\u3001", "\uff1a", "\uff1b",             // 、：；
		"\u201c", "\u201d", "\u2018", "\u2019",   // ""''
		"\uff08", "\uff09",                        // （）
		",", ".", "?", "!", ":", ";",
		"(", ")", "/", "\\", "-", "_",
	}
	text := strings.ToLower(query)
	for _, p := range punctuation {
		text = strings.ReplaceAll(text, p, " ")
	}
	words := strings.Fields(text)

	for _, w := range words {
		w = strings.TrimSpace(w)
		if len(w) < 2 || stopWords[w] || seen[w] {
			continue
		}
		seen[w] = true
		keywords = append(keywords, w)
	}

	// 限制关键词数量
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}

	return keywords
}

// scoreAndRank 计算匹配得分并排序
func scoreAndRank(indexes []model.FileIndex, queryKeywords []string) []MatchResult {
	var results []MatchResult

	for _, idx := range indexes {
		score := calculateScore(idx, queryKeywords)
		if score > 0 {
			results = append(results, MatchResult{
				FilePath: idx.FilePath,
				Language: idx.Language,
				Summary:  idx.Summary,
				Score:    score,
			})
		}
	}

	// 按得分降序排列
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// calculateScore 计算单个文件的匹配得分
func calculateScore(idx model.FileIndex, queryKeywords []string) float64 {
	var score float64
	idxKeywords := strings.ToLower(idx.Keywords)
	idxSymbols := strings.ToLower(idx.Symbols)
	idxSummary := strings.ToLower(idx.Summary)
	idxPath := strings.ToLower(idx.FilePath)

	for _, kw := range queryKeywords {
		kw = strings.ToLower(kw)
		// 关键词匹配权重最高
		if strings.Contains(idxKeywords, kw) {
			score += 3.0
		}
		// 符号匹配
		if strings.Contains(idxSymbols, kw) {
			score += 2.0
		}
		// 摘要匹配
		if strings.Contains(idxSummary, kw) {
			score += 1.5
		}
		// 路径匹配
		if strings.Contains(idxPath, kw) {
			score += 1.0
		}
	}

	return score
}
