package tokenutil

import "agent_flow/src/provider"

// EstimateText 基于 BPE 预分词模拟估算 token 数
// 单遍 UTF-8 扫描，零内存分配，精度 ±10-15%
func EstimateText(text string) int {
	if text == "" {
		return 0
	}
	tokens := 0
	i := 0
	n := len(text)

	for i < n {
		b := text[i]

		// 快速路径：ASCII（0x00-0x7F）
		if b < 0x80 {
			switch {
			case isASCIILetter(b):
				start := i
				i++
				for i < n && isASCIILetter(text[i]) {
					i++
				}
				wordLen := i - start
				tokens += estimateWordTokens(wordLen)
				// 检查缩略词后缀
				if i < n && text[i] == '\'' {
					suffixLen := matchContraction(text, i, n)
					if suffixLen > 0 {
						tokens++
						i += suffixLen
					}
				}

			case isDigit(b):
				count := 0
				for i < n && isDigit(text[i]) {
					i++
					count++
					if count == 3 {
						tokens++
						count = 0
					}
				}
				if count > 0 {
					tokens++
				}

			case b == '\n':
				tokens++
				i++

			case b == '\r':
				tokens++
				i++
				if i < n && text[i] == '\n' {
					i++
				}

			case b == ' ' || b == '\t':
				i++
				for i < n && (text[i] == ' ' || text[i] == '\t') {
					i++
				}

			default:
				start := i
				i++
				for i < n && isASCIISymbol(text[i]) {
					i++
				}
				symLen := i - start
				tokens += (symLen + 1) / 2
			}
			continue
		}

		// 非 ASCII：解码 UTF-8 rune
		r, size := decodeRune(text, i)
		i += size

		if isCJK(r) {
			cjkCount := 1
			for i < n {
				r2, s2 := decodeRune(text, i)
				if !isCJK(r2) {
					break
				}
				cjkCount++
				i += s2
			}
			tokens += (cjkCount*7 + 4) / 5

		} else if isUnicodeLetter(r) {
			wordLen := 1
			for i < n {
				if text[i] < 0x80 {
					if isASCIILetter(text[i]) {
						wordLen++
						i++
						continue
					}
					break
				}
				r2, s2 := decodeRune(text, i)
				if !isUnicodeLetter(r2) {
					break
				}
				wordLen++
				i += s2
			}
			tokens += estimateWordTokens(wordLen)

		} else {
			tokens += 2
		}
	}

	if tokens == 0 && n > 0 {
		tokens = 1
	}
	return tokens
}

// EstimateMessages 估算消息列表 token 数
func EstimateMessages(messages []provider.Message) int {
	total := 0
	for _, m := range messages {
		total += EstimateText(m.Content)
		total += EstimateText(m.ReasoningContent)
		for _, tc := range m.ToolCalls {
			total += EstimateText(tc.Arguments)
			total += EstimateText(tc.Name)
		}
		total += 4
	}
	return total
}

// EstimateToolDefs 估算工具定义列表 token 数
func EstimateToolDefs(toolDefs []provider.ToolDef) int {
	total := 0
	for _, t := range toolDefs {
		total += EstimateText(t.Name)
		total += EstimateText(t.Description)
		total += EstimateText(string(t.Parameters))
		total += 10
	}
	return total
}

// EstimateContext 估算完整会话上下文（消息 + 工具定义）
func EstimateContext(messages []provider.Message, toolDefs []provider.ToolDef) int {
	return EstimateMessages(messages) + EstimateToolDefs(toolDefs)
}

func estimateWordTokens(length int) int {
	switch {
	case length <= 5:
		return 1
	case length <= 9:
		return 2
	case length <= 14:
		return 3
	default:
		return (length + 4) / 5
	}
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '_'
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isASCIISymbol(b byte) bool {
	return b >= 0x21 && b <= 0x7E && !isASCIILetter(b) && !isDigit(b) &&
		b != '\'' && b != ' ' && b != '\t' && b != '\n' && b != '\r'
}

func matchContraction(text string, i, n int) int {
	if i+1 >= n || text[i] != '\'' {
		return 0
	}
	if i+1 < n {
		c := text[i+1] | 0x20
		switch c {
		case 's', 't', 'm', 'd':
			return 2
		}
	}
	if i+2 < n {
		c1 := text[i+1] | 0x20
		c2 := text[i+2] | 0x20
		if (c1 == 'r' && c2 == 'e') || (c1 == 'v' && c2 == 'e') || (c1 == 'l' && c2 == 'l') {
			return 3
		}
	}
	return 0
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0x3000 && r <= 0x303F) ||
		(r >= 0x3040 && r <= 0x309F) ||
		(r >= 0x30A0 && r <= 0x30FF) ||
		(r >= 0xFF00 && r <= 0xFFEF)
}

func isUnicodeLetter(r rune) bool {
	return (r >= 0x00C0 && r <= 0x024F) ||
		(r >= 0x0400 && r <= 0x04FF) ||
		(r >= 0x0600 && r <= 0x06FF) ||
		(r >= 0x0E00 && r <= 0x0E7F) ||
		(r >= 0x1100 && r <= 0x11FF)
}

func decodeRune(s string, i int) (rune, int) {
	b := s[i]
	if b < 0x80 {
		return rune(b), 1
	}
	if b < 0xE0 {
		if i+1 >= len(s) {
			return 0xFFFD, 1
		}
		return rune(b&0x1F)<<6 | rune(s[i+1]&0x3F), 2
	}
	if b < 0xF0 {
		if i+2 >= len(s) {
			return 0xFFFD, 1
		}
		return rune(b&0x0F)<<12 | rune(s[i+1]&0x3F)<<6 | rune(s[i+2]&0x3F), 3
	}
	if i+3 >= len(s) {
		return 0xFFFD, 1
	}
	return rune(b&0x07)<<18 | rune(s[i+1]&0x3F)<<12 | rune(s[i+2]&0x3F)<<6 | rune(s[i+3]&0x3F), 4
}
