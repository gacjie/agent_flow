package analyzers

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reHTMLTag      = regexp.MustCompile(`<(script|style|template|head|body|main|nav|header|footer|section|article|form)[\s>]`)
	reHTMLTemplate = regexp.MustCompile(`\{\{[-\s]*(.+?)[-\s]*\}\}`)
	reCSSSelector  = regexp.MustCompile(`^([.#\w][\w\s.#:>,*\[\]=~^|$-]*)\s*\{`)
	reCSSMedia     = regexp.MustCompile(`^@media\s+(.+)\s*\{`)
	reCSSVar       = regexp.MustCompile(`--[\w-]+\s*:`)
	reSQLCreate    = regexp.MustCompile(`(?i)^CREATE\s+(TABLE|VIEW|INDEX|PROCEDURE|FUNCTION|TRIGGER)\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)
)

func AnalyzeWeb(content, lang string) string {
	switch lang {
	case "html":
		return analyzeHTML(content)
	case "css":
		return analyzeCSS(content)
	case "sql":
		return analyzeSQL(content)
	}
	return ""
}

func analyzeHTML(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var tags []SymbolEntry
	templateDirs := 0

	for i, line := range lines {
		ln := i + 1
		if m := reHTMLTag.FindStringSubmatch(line); m != nil {
			tags = append(tags, SymbolEntry{ln, "<" + m[1] + ">"})
		}
		templateDirs += len(reHTMLTemplate.FindAllString(line, -1))
	}

	sb.WriteString(FormatSymbols("structure", tags))
	if templateDirs > 0 {
		sb.WriteString(fmt.Sprintf("[template directives] %d\n", templateDirs))
	}
	return sb.String()
}

func analyzeCSS(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var selectors, mediaQueries []SymbolEntry
	varCount := 0

	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)
		if m := reCSSMedia.FindStringSubmatch(trimmed); m != nil {
			mediaQueries = append(mediaQueries, SymbolEntry{ln, "@media " + TruncSig(m[1], 60)})
			continue
		}
		if m := reCSSSelector.FindStringSubmatch(trimmed); m != nil {
			selectors = append(selectors, SymbolEntry{ln, TruncSig(m[1], 80)})
		}
		varCount += len(reCSSVar.FindAllString(line, -1))
	}

	sb.WriteString(FormatSymbols("selectors", selectors))
	if len(mediaQueries) > 0 {
		sb.WriteString(FormatSymbols("media queries", mediaQueries))
	}
	if varCount > 0 {
		sb.WriteString(fmt.Sprintf("[css variables] %d\n", varCount))
	}
	return sb.String()
}

func analyzeSQL(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var objects []SymbolEntry

	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)
		if m := reSQLCreate.FindStringSubmatch(trimmed); m != nil {
			objects = append(objects, SymbolEntry{ln, fmt.Sprintf("%s %s", strings.ToLower(m[1]), m[2])})
		}
	}

	sb.WriteString(FormatSymbols("objects", objects))
	return sb.String()
}
