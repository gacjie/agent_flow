package analyzers

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reYAMLKey     = regexp.MustCompile(`^(\s*)(\w[\w.-]*):\s*(.*)`)
	reYAMLComment = regexp.MustCompile(`^\s*#`)
	reJSONKey     = regexp.MustCompile(`^\s*"([^"]+)"\s*:`)
	reTOMLSection = regexp.MustCompile(`^\s*\[{1,2}([^\]]+)\]{1,2}`)
	reTOMLKey     = regexp.MustCompile(`^\s*(\w[\w.-]*)\s*=`)
)

func AnalyzeConfig(content, lang string) string {
	switch lang {
	case "yaml":
		return analyzeYAML(content)
	case "json":
		return analyzeJSON(content)
	case "toml":
		return analyzeTOML(content)
	}
	return ""
}

func analyzeYAML(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var keys []SymbolEntry
	maxDepth := 0
	commentCount := 0

	for i, line := range lines {
		ln := i + 1
		if reYAMLComment.MatchString(line) {
			commentCount++
			continue
		}
		if m := reYAMLKey.FindStringSubmatch(line); m != nil {
			indent := len(m[1])
			depth := indent/2 + 1
			if depth > maxDepth {
				maxDepth = depth
			}
			if depth <= 2 {
				display := strings.TrimRight(line, "\r\n")
				keys = append(keys, SymbolEntry{ln, TruncSig(display, 80)})
			}
		}
	}

	sb.WriteString(FormatSymbols("keys", keys))
	sb.WriteString(fmt.Sprintf("[depth] max %d levels\n", maxDepth))
	if commentCount > 0 {
		sb.WriteString(fmt.Sprintf("[comments] %d lines\n", commentCount))
	}
	return sb.String()
}

func analyzeJSON(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	depth := 0
	maxDepth := 0
	var topKeys []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, ch := range trimmed {
			if ch == '{' || ch == '[' {
				depth++
				if depth > maxDepth {
					maxDepth = depth
				}
			} else if ch == '}' || ch == ']' {
				depth--
			}
		}
		if m := reJSONKey.FindStringSubmatch(line); m != nil {
			currentDepth := 0
			for _, ch := range line[:strings.Index(line, "\"")] {
				if ch == ' ' {
					currentDepth++
				} else if ch == '\t' {
					currentDepth += 2
				}
			}
			if currentDepth <= 2 {
				topKeys = append(topKeys, m[1])
			}
		}
	}

	if len(topKeys) > 0 {
		sb.WriteString(fmt.Sprintf("[structure] object, %d top-level keys\n", len(topKeys)))
		limit := len(topKeys)
		if limit > 20 {
			limit = 20
		}
		for _, k := range topKeys[:limit] {
			sb.WriteString(fmt.Sprintf("  \"%s\"\n", k))
		}
		if len(topKeys) > 20 {
			sb.WriteString(fmt.Sprintf("  ... 及另外 %d 个\n", len(topKeys)-20))
		}
	}
	sb.WriteString(fmt.Sprintf("[depth] max %d levels\n", maxDepth))
	return sb.String()
}

func analyzeTOML(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var sections []SymbolEntry
	keyCount := 0

	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if m := reTOMLSection.FindStringSubmatch(trimmed); m != nil {
			sections = append(sections, SymbolEntry{ln, trimmed})
			continue
		}
		if reTOMLKey.MatchString(trimmed) {
			keyCount++
		}
	}

	sb.WriteString(FormatSymbols("sections", sections))
	sb.WriteString(fmt.Sprintf("[keys] %d keys across %d sections\n", keyCount, len(sections)))
	return sb.String()
}
