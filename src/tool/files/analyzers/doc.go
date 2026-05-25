package analyzers

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reMdHeading   = regexp.MustCompile(`^(#{1,6})\s+(.+)`)
	reMdCodeFence = regexp.MustCompile("^```(\\w*)")
	reMdLink      = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	reMdImage     = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]*)\)`)
)

func AnalyzeMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var headings []SymbolEntry
	codeBlocks := map[string]int{}
	inCode := false
	linkCount, imageCount := 0, 0

	for i, line := range lines {
		ln := i + 1

		if m := reMdCodeFence.FindStringSubmatch(line); m != nil {
			if !inCode {
				inCode = true
				lang := m[1]
				if lang == "" {
					lang = "text"
				}
				codeBlocks[lang]++
			} else {
				inCode = false
			}
			continue
		}
		if inCode {
			continue
		}
		if m := reMdHeading.FindStringSubmatch(line); m != nil {
			headings = append(headings, SymbolEntry{ln, strings.TrimRight(line, "\r")})
		}
		imageCount += len(reMdImage.FindAllString(line, -1))
		linkCount += len(reMdLink.FindAllString(line, -1)) - len(reMdImage.FindAllString(line, -1))
	}

	sb.WriteString(FormatSymbols("headings", headings))

	if len(codeBlocks) > 0 {
		total := 0
		var parts []string
		for lang, cnt := range codeBlocks {
			total += cnt
			parts = append(parts, fmt.Sprintf("%s: %d", lang, cnt))
		}
		sb.WriteString(fmt.Sprintf("[code blocks] %d blocks (%s)\n", total, strings.Join(parts, ", ")))
	}

	sb.WriteString(fmt.Sprintf("[links] %d links, %d images\n", linkCount, imageCount))
	return sb.String()
}
