package diagnostics

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const maxDiagnostics = 20

// Engine 诊断引擎（全局单例）
type Engine struct {
	mu      sync.RWMutex
	parsers map[string]*gotreesitter.Language
}

var (
	defaultEngine *Engine
	engineOnce    sync.Once
)

// GetEngine 获取全局诊断引擎
func GetEngine() *Engine {
	engineOnce.Do(func() {
		defaultEngine = &Engine{
			parsers: make(map[string]*gotreesitter.Language),
		}
	})
	return defaultEngine
}

// Diagnose 对文件内容执行语法诊断
func (e *Engine) Diagnose(ctx context.Context, content []byte, lang string, filePath string) *DiagResult {
	result := &DiagResult{File: filePath, Language: lang, Supported: true}

	language := e.getLanguage(lang, filePath)
	if language == nil {
		result.Supported = false
		return result
	}

	parser := gotreesitter.NewParser(language)
	tree, err := parser.Parse(content)
	if err != nil {
		result.ParseError = err.Error()
		return result
	}
	defer tree.Release()

	root := tree.RootNode()
	if root == nil || !root.HasError() {
		return result
	}

	e.collectErrors(root, content, language, result)
	return result
}

// getLanguage 获取语言（懒加载 + 缓存）
func (e *Engine) getLanguage(lang, filePath string) *gotreesitter.Language {
	e.mu.RLock()
	if l, ok := e.parsers[lang]; ok {
		e.mu.RUnlock()
		return l
	}
	e.mu.RUnlock()

	entry := grammars.DetectLanguageByName(lang)
	if entry == nil && filePath != "" {
		entry = grammars.DetectLanguage(filePath)
	}
	if entry == nil {
		return nil
	}

	language := entry.Language()
	if language == nil {
		return nil
	}

	e.mu.Lock()
	e.parsers[lang] = language
	e.mu.Unlock()
	return language
}

// collectErrors 遍历 AST 收集错误节点
func (e *Engine) collectErrors(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, result *DiagResult) {
	if len(result.Diagnostics) >= maxDiagnostics {
		return
	}

	if node.IsMissing() {
		diag := e.buildDiagnostic(node, source, lang)
		result.Diagnostics = append(result.Diagnostics, diag)
		return
	}

	if node.IsError() {
		hasChildErrors := false
		for i := 0; i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && (child.IsError() || child.IsMissing()) {
				hasChildErrors = true
				break
			}
		}

		if hasChildErrors {
			for i := 0; i < node.ChildCount(); i++ {
				child := node.Child(i)
				if child == nil {
					continue
				}
				if child.IsError() || child.IsMissing() || child.HasError() {
					e.collectErrors(child, source, lang, result)
				}
				if len(result.Diagnostics) >= maxDiagnostics {
					return
				}
			}
		} else {
			diag := e.buildDiagnostic(node, source, lang)
			result.Diagnostics = append(result.Diagnostics, diag)
		}
		return
	}

	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.HasError() || child.IsError() || child.IsMissing() {
			e.collectErrors(child, source, lang, result)
		}
		if len(result.Diagnostics) >= maxDiagnostics {
			return
		}
	}
}

// buildDiagnostic 从错误节点构建诊断信息
func (e *Engine) buildDiagnostic(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) Diagnostic {
	start := node.StartPoint()
	end := node.EndPoint()

	diag := Diagnostic{
		Line:     int(start.Row) + 1,
		Column:   int(start.Column) + 1,
		EndLine:  int(end.Row) + 1,
		EndCol:   int(end.Column) + 1,
		Severity: SeverityError,
	}

	if node.IsMissing() {
		nodeType := node.Type(lang)
		diag.Message = fmt.Sprintf("缺少 '%s'", nodeType)
	} else {
		diag.Message = e.inferErrorMessage(node, source, lang)
	}

	diag.Context = e.extractContext(source, int(start.Row))
	return diag
}

// inferErrorMessage 根据错误节点上下文推断错误消息
func (e *Engine) inferErrorMessage(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
	text := node.Text(source)
	if len(text) > 30 {
		text = text[:30] + "..."
	}
	text = strings.TrimSpace(text)

	parent := node.Parent()
	if parent != nil && !parent.IsError() {
		parentType := parent.Type(lang)
		if parentType != "" && parentType != "ERROR" && !strings.HasPrefix(parentType, "_") && !strings.Contains(parentType, "_repeat") {
			if text != "" {
				return fmt.Sprintf("'%s' 附近存在语法错误（在 %s 中）", text, parentType)
			}
			return fmt.Sprintf("%s 中存在语法错误", parentType)
		}
	}

	if text != "" {
		return fmt.Sprintf("意外的 '%s'", text)
	}

	// 尝试从相邻节点推断上下文
	prev := node.PrevSibling()
	if prev != nil {
		prevText := strings.TrimSpace(prev.Text(source))
		if len(prevText) > 20 {
			prevText = prevText[:20] + "..."
		}
		if prevText != "" {
			return fmt.Sprintf("'%s' 之后存在语法错误", prevText)
		}
	}
	return "语法错误"
}

// extractContext 提取错误行附近的代码
func (e *Engine) extractContext(source []byte, row int) string {
	lines := strings.Split(string(source), "\n")
	if row >= len(lines) {
		return ""
	}

	line := lines[row]
	if len(line) > 80 {
		line = line[:80] + "..."
	}
	return strings.TrimRight(line, "\r")
}
