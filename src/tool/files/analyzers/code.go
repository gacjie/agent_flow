package analyzers

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reGoPackage   = regexp.MustCompile(`^package\s+(\w+)`)
	reGoImport    = regexp.MustCompile(`^\s*"([^"]+)"`)
	reGoFunc      = regexp.MustCompile(`^func\s+(\([^)]*\)\s*)?(\w+)\s*(\(.*)?$`)
	reGoType      = regexp.MustCompile(`^type\s+(\w+)\s+(struct|interface)`)
	reGoTypeOther = regexp.MustCompile(`^type\s+(\w+)\s+`)
	reGoConst     = regexp.MustCompile(`^(?:const|var)\s+(\w+)`)

	rePyClass     = regexp.MustCompile(`^class\s+(\w+)`)
	rePyFunc      = regexp.MustCompile(`^(\s*)def\s+(\w+)\s*\((.*)$`)
	rePyImport    = regexp.MustCompile(`^(?:from\s+\S+\s+)?import\s+(.+)`)
	rePyDecorator = regexp.MustCompile(`^\s*@(\w+)`)

	reJSImport    = regexp.MustCompile(`^import\s+`)
	reJSRequire   = regexp.MustCompile(`require\(['"]([^'"]+)['"]\)`)
	reJSFunc      = regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+(\w+)`)
	reJSClass     = regexp.MustCompile(`^(?:export\s+)?class\s+(\w+)`)
	reJSArrow     = regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?(?:\([^)]*\)|[^=])\s*=>`)
	reTSInterface = regexp.MustCompile(`^(?:export\s+)?interface\s+(\w+)`)
	reTSType      = regexp.MustCompile(`^(?:export\s+)?type\s+(\w+)`)

	reJavaPackage = regexp.MustCompile(`^package\s+([\w.]+)`)
	reJavaImport  = regexp.MustCompile(`^import\s+([\w.*]+)`)
	reJavaClass   = regexp.MustCompile(`(?:public|private|protected)?\s*(?:abstract\s+|static\s+|final\s+)*(class|interface|enum|record)\s+(\w+)`)
	reJavaMethod  = regexp.MustCompile(`^\s+(?:public|private|protected)\s+(?:static\s+|final\s+|abstract\s+|synchronized\s+)*(?:\w+(?:<[^>]+>)?)\s+(\w+)\s*\(`)

	reRustUse    = regexp.MustCompile(`^use\s+(.+);`)
	reRustFn     = regexp.MustCompile(`^(\s*)(?:pub\s+)?(?:async\s+)?fn\s+(\w+)`)
	reRustStruct = regexp.MustCompile(`^(?:pub\s+)?struct\s+(\w+)`)
	reRustEnum   = regexp.MustCompile(`^(?:pub\s+)?enum\s+(\w+)`)
	reRustTrait  = regexp.MustCompile(`^(?:pub\s+)?trait\s+(\w+)`)
	reRustImpl   = regexp.MustCompile(`^impl(?:<[^>]+>)?\s+(?:(\w+)\s+for\s+)?(\w+)`)

	reCInclude = regexp.MustCompile(`^#include\s+[<"]([^>"]+)[>"]`)
	reCFunc    = regexp.MustCompile(`^(?:static\s+|inline\s+|extern\s+|const\s+)*(\w[\w\s*&]+?)\s+(\w+)\s*\(`)
	reCStruct  = regexp.MustCompile(`^(?:typedef\s+)?struct\s+(\w+)`)
	reCppClass = regexp.MustCompile(`^class\s+(\w+)`)
	reCDefine  = regexp.MustCompile(`^#define\s+(\w+)`)
)

func AnalyzeCode(content, lang string) string {
	switch lang {
	case "go":
		return analyzeGo(content)
	case "python":
		return analyzePython(content)
	case "javascript", "jsx", "typescript", "tsx", "vue":
		return analyzeJS(content, lang)
	case "java":
		return analyzeJava(content)
	case "rust":
		return analyzeRust(content)
	case "c", "c-header", "cpp", "cpp-header":
		return analyzeC(content, lang)
	default:
		return analyzeGenericCode(content)
	}
}

func analyzeGo(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var pkg string
	var imports []string
	var types, funcs []SymbolEntry
	inImport := false

	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)

		if m := reGoPackage.FindStringSubmatch(trimmed); m != nil {
			pkg = m[1]
			continue
		}
		if trimmed == "import (" {
			inImport = true
			continue
		}
		if inImport {
			if trimmed == ")" {
				inImport = false
				continue
			}
			if m := reGoImport.FindStringSubmatch(line); m != nil {
				parts := strings.Split(m[1], "/")
				imports = append(imports, parts[len(parts)-1])
			}
			continue
		}
		if strings.HasPrefix(trimmed, "import \"") {
			if m := reGoImport.FindStringSubmatch(trimmed); m != nil {
				parts := strings.Split(m[1], "/")
				imports = append(imports, parts[len(parts)-1])
			}
			continue
		}
		if m := reGoType.FindStringSubmatch(trimmed); m != nil {
			types = append(types, SymbolEntry{ln, fmt.Sprintf("%s %s", m[2], m[1])})
			continue
		}
		if m := reGoTypeOther.FindStringSubmatch(trimmed); m != nil && !reGoType.MatchString(trimmed) {
			types = append(types, SymbolEntry{ln, "type " + m[1]})
			continue
		}
		if m := reGoFunc.FindStringSubmatch(trimmed); m != nil {
			sig := TruncSig(trimmed, 120)
			funcs = append(funcs, SymbolEntry{ln, sig})
			continue
		}
	}

	if pkg != "" {
		sb.WriteString("[package] " + pkg + "\n")
	}
	if s := FormatImports(imports); s != "" {
		sb.WriteString(s + "\n")
	}
	sb.WriteString(FormatSymbols("types", types))
	sb.WriteString(FormatSymbols("functions", funcs))
	return sb.String()
}

func analyzePython(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var imports []string
	var classes, funcs []SymbolEntry
	currentClass := ""
	classIndent := 0

	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if m := rePyImport.FindStringSubmatch(trimmed); m != nil {
			imports = append(imports, TruncSig(trimmed, 80))
			continue
		}
		if m := rePyClass.FindStringSubmatch(trimmed); m != nil {
			currentClass = m[1]
			classIndent = len(line) - len(strings.TrimLeft(line, " \t"))
			classes = append(classes, SymbolEntry{ln, "class " + TruncSig(trimmed[6:], 100)})
			continue
		}
		if m := rePyFunc.FindStringSubmatch(line); m != nil {
			indent := len(m[1])
			name := m[2]
			if currentClass != "" && indent > classIndent {
				sig := fmt.Sprintf("def %s.%s(%s", currentClass, name, m[3])
				funcs = append(funcs, SymbolEntry{ln, TruncSig(sig, 120)})
			} else {
				currentClass = ""
				funcs = append(funcs, SymbolEntry{ln, TruncSig(trimmed, 120)})
			}
			continue
		}
		if currentClass != "" && trimmed != "" {
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			if indent <= classIndent && !rePyDecorator.MatchString(trimmed) {
				currentClass = ""
			}
		}
	}

	if s := FormatImports(imports); s != "" {
		sb.WriteString(s + "\n")
	}
	sb.WriteString(FormatSymbols("classes", classes))
	sb.WriteString(FormatSymbols("functions", funcs))
	return sb.String()
}

func analyzeJS(content, lang string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var imports []string
	var types, funcs []SymbolEntry

	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)

		if reJSImport.MatchString(trimmed) {
			imports = append(imports, TruncSig(trimmed, 80))
			continue
		}
		if m := reJSRequire.FindStringSubmatch(trimmed); m != nil {
			imports = append(imports, m[1])
			continue
		}
		if m := reTSInterface.FindStringSubmatch(trimmed); m != nil {
			types = append(types, SymbolEntry{ln, "interface " + m[1]})
			continue
		}
		if m := reTSType.FindStringSubmatch(trimmed); m != nil {
			types = append(types, SymbolEntry{ln, "type " + m[1]})
			continue
		}
		if m := reJSClass.FindStringSubmatch(trimmed); m != nil {
			types = append(types, SymbolEntry{ln, "class " + m[1]})
			continue
		}
		if m := reJSFunc.FindStringSubmatch(trimmed); m != nil {
			funcs = append(funcs, SymbolEntry{ln, TruncSig(trimmed, 120)})
			continue
		}
		if m := reJSArrow.FindStringSubmatch(trimmed); m != nil {
			funcs = append(funcs, SymbolEntry{ln, TruncSig(trimmed, 120)})
			continue
		}
	}

	if s := FormatImports(imports); s != "" {
		sb.WriteString(s + "\n")
	}
	sb.WriteString(FormatSymbols("types", types))
	sb.WriteString(FormatSymbols("functions", funcs))
	return sb.String()
}

func analyzeJava(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var pkg string
	var imports []string
	var types, funcs []SymbolEntry

	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)

		if m := reJavaPackage.FindStringSubmatch(trimmed); m != nil {
			pkg = m[1]
			continue
		}
		if m := reJavaImport.FindStringSubmatch(trimmed); m != nil {
			imports = append(imports, m[1])
			continue
		}
		if m := reJavaClass.FindStringSubmatch(trimmed); m != nil {
			types = append(types, SymbolEntry{ln, fmt.Sprintf("%s %s", m[1], m[2])})
			continue
		}
		if m := reJavaMethod.FindStringSubmatch(trimmed); m != nil {
			funcs = append(funcs, SymbolEntry{ln, TruncSig(trimmed, 120)})
			continue
		}
	}

	if pkg != "" {
		sb.WriteString("[package] " + pkg + "\n")
	}
	if s := FormatImports(imports); s != "" {
		sb.WriteString(s + "\n")
	}
	sb.WriteString(FormatSymbols("types", types))
	sb.WriteString(FormatSymbols("methods", funcs))
	return sb.String()
}

func analyzeRust(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var imports []string
	var types, funcs []SymbolEntry
	currentImpl := ""

	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)

		if m := reRustUse.FindStringSubmatch(trimmed); m != nil {
			imports = append(imports, m[1])
			continue
		}
		if m := reRustStruct.FindStringSubmatch(trimmed); m != nil {
			types = append(types, SymbolEntry{ln, "struct " + m[1]})
			continue
		}
		if m := reRustEnum.FindStringSubmatch(trimmed); m != nil {
			types = append(types, SymbolEntry{ln, "enum " + m[1]})
			continue
		}
		if m := reRustTrait.FindStringSubmatch(trimmed); m != nil {
			types = append(types, SymbolEntry{ln, "trait " + m[1]})
			continue
		}
		if m := reRustImpl.FindStringSubmatch(trimmed); m != nil {
			currentImpl = m[2]
			if m[1] != "" {
				types = append(types, SymbolEntry{ln, fmt.Sprintf("impl %s for %s", m[1], m[2])})
			} else {
				types = append(types, SymbolEntry{ln, "impl " + m[2]})
			}
			continue
		}
		if m := reRustFn.FindStringSubmatch(line); m != nil {
			indent := len(m[1])
			name := m[2]
			if currentImpl != "" && indent > 0 {
				sig := fmt.Sprintf("fn %s.%s", currentImpl, name)
				funcs = append(funcs, SymbolEntry{ln, TruncSig(sig, 120)})
			} else {
				funcs = append(funcs, SymbolEntry{ln, TruncSig(trimmed, 120)})
				if indent == 0 {
					currentImpl = ""
				}
			}
			continue
		}
		if trimmed == "}" && !strings.Contains(line, "    ") {
			currentImpl = ""
		}
	}

	if s := FormatImports(imports); s != "" {
		sb.WriteString(s + "\n")
	}
	sb.WriteString(FormatSymbols("types", types))
	sb.WriteString(FormatSymbols("functions", funcs))
	return sb.String()
}

func analyzeC(content, lang string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var includes []string
	var types, funcs []SymbolEntry
	var defines []SymbolEntry

	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)

		if m := reCInclude.FindStringSubmatch(trimmed); m != nil {
			includes = append(includes, m[1])
			continue
		}
		if m := reCDefine.FindStringSubmatch(trimmed); m != nil {
			defines = append(defines, SymbolEntry{ln, "#define " + m[1]})
			continue
		}
		if m := reCppClass.FindStringSubmatch(trimmed); m != nil {
			types = append(types, SymbolEntry{ln, "class " + m[1]})
			continue
		}
		if m := reCStruct.FindStringSubmatch(trimmed); m != nil {
			types = append(types, SymbolEntry{ln, "struct " + m[1]})
			continue
		}
		if m := reCFunc.FindStringSubmatch(trimmed); m != nil {
			if !strings.HasPrefix(trimmed, "if") && !strings.HasPrefix(trimmed, "for") &&
				!strings.HasPrefix(trimmed, "while") && !strings.HasPrefix(trimmed, "switch") &&
				!strings.HasPrefix(trimmed, "return") {
				funcs = append(funcs, SymbolEntry{ln, TruncSig(trimmed, 120)})
			}
			continue
		}
	}

	if len(includes) > 0 {
		if len(includes) <= 5 {
			sb.WriteString("[includes] " + strings.Join(includes, ", ") + "\n")
		} else {
			sb.WriteString(fmt.Sprintf("[includes] %d items: %s, ...\n", len(includes), strings.Join(includes[:3], ", ")))
		}
	}
	sb.WriteString(FormatSymbols("types", types))
	sb.WriteString(FormatSymbols("functions", funcs))
	if len(defines) > 0 && len(defines) <= 20 {
		sb.WriteString(FormatSymbols("defines", defines))
	} else if len(defines) > 20 {
		sb.WriteString(fmt.Sprintf("[defines] %d macros\n", len(defines)))
	}
	return sb.String()
}

func analyzeGenericCode(content string) string {
	lines := strings.Split(content, "\n")
	var sb strings.Builder
	var funcs []SymbolEntry

	genericFunc := regexp.MustCompile(`(?:func|def|function|fn)\s+(\w+)`)
	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)
		if m := genericFunc.FindStringSubmatch(trimmed); m != nil {
			funcs = append(funcs, SymbolEntry{ln, TruncSig(trimmed, 120)})
		}
	}

	sb.WriteString(FormatSymbols("functions", funcs))
	return sb.String()
}
