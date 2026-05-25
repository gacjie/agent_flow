package analyzers

import (
	"fmt"
	"strings"
)

type SymbolEntry struct {
	Line      int
	Signature string
}

const MaxSymbols = 50

func TruncSig(s string, max int) string {
	s = strings.TrimRight(s, "\r\n")
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

func FormatImports(imports []string) string {
	if len(imports) == 0 {
		return ""
	}
	if len(imports) <= 5 {
		return "[imports] " + strings.Join(imports, ", ")
	}
	return fmt.Sprintf("[imports] %d items: %s, ...", len(imports), strings.Join(imports[:3], ", "))
}

func FormatSymbols(label string, symbols []SymbolEntry) string {
	if len(symbols) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("[" + label + "]\n")
	limit := len(symbols)
	truncated := false
	if limit > MaxSymbols {
		limit = MaxSymbols
		truncated = true
	}
	for _, s := range symbols[:limit] {
		sb.WriteString(fmt.Sprintf("  L%-4d %s\n", s.Line, s.Signature))
	}
	if truncated {
		sb.WriteString(fmt.Sprintf("  ... 及另外 %d 个\n", len(symbols)-MaxSymbols))
	}
	return sb.String()
}
