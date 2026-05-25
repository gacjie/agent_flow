package diagnostics

// Severity 诊断严重程度
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic 单条诊断信息
type Diagnostic struct {
	Line     int      // 行号（从 1 开始）
	Column   int      // 列号（从 1 开始）
	EndLine  int      // 结束行号
	EndCol   int      // 结束列号
	Severity Severity // 严重程度
	Message  string   // 诊断信息
	Context  string   // 错误位置附近的代码片段
}

// DiagResult 单个文件的诊断结果
type DiagResult struct {
	File        string
	Language    string
	Diagnostics []Diagnostic
	Supported   bool
	ParseError  string // 引擎级错误（非语法错误）
}
