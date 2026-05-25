package service

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"agent_flow/src/model"

	"gorm.io/gorm"
)

// IndexerService 文件索引服务
// 扫描工作区文件，计算指纹，增量更新索引
type IndexerService struct {
	DB *gorm.DB
}

// NewIndexerService 创建索引服务
func NewIndexerService(db *gorm.DB) *IndexerService {
	return &IndexerService{DB: db}
}

// 索引配置
const (
	maxIndexFileSize = 512 * 1024 // 最大索引文件大小 512KB
	maxIndexFiles    = 5000       // 单工作区最大索引文件数
)

// 可索引的文件扩展名
var indexableExtensions = map[string]string{
	".go":    "go",
	".py":    "python",
	".js":    "javascript",
	".ts":    "typescript",
	".jsx":   "javascript",
	".tsx":   "typescript",
	".java":  "java",
	".rs":    "rust",
	".c":     "c",
	".cpp":   "cpp",
	".h":     "c",
	".hpp":   "cpp",
	".rb":    "ruby",
	".php":   "php",
	".swift": "swift",
	".kt":    "kotlin",
	".cs":    "csharp",
	".vue":   "vue",
	".sql":   "sql",
	".sh":    "shell",
	".yaml":  "yaml",
	".yml":   "yaml",
	".json":  "json",
	".toml":  "toml",
	".md":    "markdown",
	".html":  "html",
	".css":   "css",
}

// 跳过的目录
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".venv":        true,
	"__pycache__":  true,
	"dist":         true,
	"build":        true,
	"target":       true,
	".idea":        true,
	".vscode":      true,
}

// ScanResult 扫描结果
type ScanResult struct {
	Added   int // 新增文件数
	Updated int // 更新文件数
	Removed int // 删除文件数
	Skipped int // 跳过文件数
}

// ScanWorkspace 扫描工作区并增量更新索引
func (s *IndexerService) ScanWorkspace(workspaceID uint, rootDir string) (*ScanResult, error) {
	result := &ScanResult{}

	// 获取现有索引
	existingMap, err := s.getExistingIndexMap(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("获取现有索引失败: %w", err)
	}

	// 记录扫描到的文件路径
	scannedPaths := make(map[string]bool)
	fileCount := 0

	// 遍历目录
	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}

		// 跳过目录
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// 限制文件数
		if fileCount >= maxIndexFiles {
			return filepath.SkipAll
		}

		// 检查扩展名
		ext := strings.ToLower(filepath.Ext(d.Name()))
		lang, ok := indexableExtensions[ext]
		if !ok {
			result.Skipped++
			return nil
		}

		// 获取文件信息
		info, err := d.Info()
		if err != nil || info.Size() > maxIndexFileSize {
			result.Skipped++
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath) // 统一为 / 分隔符

		scannedPaths[relPath] = true
		fileCount++

		// 计算文件哈希
		hash, err := fileHash(path)
		if err != nil {
			return nil
		}

		// 检查是否需要更新
		if existing, ok := existingMap[relPath]; ok {
			if existing.FileHash == hash {
				return nil // 文件未变化，跳过
			}
			// 文件已变化，更新索引
			symbols := extractSymbols(path, lang)
			s.DB.Model(&model.FileIndex{}).Where("id = ?", existing.ID).Updates(map[string]interface{}{
				"file_hash": hash,
				"file_size": info.Size(),
				"language":  lang,
				"symbols":   symbols,
				"keywords":  "", // 清空旧关键词，待 LLM 重新提取
				"summary":   "",
			})
			result.Updated++
		} else {
			// 新文件，创建索引
			symbols := extractSymbols(path, lang)
			idx := &model.FileIndex{
				WorkspaceID: workspaceID,
				FilePath:    relPath,
				FileHash:    hash,
				FileSize:    info.Size(),
				Language:    lang,
				Symbols:     symbols,
			}
			s.DB.Create(idx)
			result.Added++
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("扫描目录失败: %w", err)
	}

	// 删除不再存在的文件索引
	for relPath, existing := range existingMap {
		if !scannedPaths[relPath] {
			s.DB.Delete(&model.FileIndex{}, existing.ID)
			result.Removed++
		}
	}

	slog.Info("工作区索引扫描完成",
		"workspace_id", workspaceID,
		"added", result.Added,
		"updated", result.Updated,
		"removed", result.Removed,
		"skipped", result.Skipped,
	)

	return result, nil
}

// UpdateKeywords 为缺少关键词的索引调用 LLM 提取（批量）
func (s *IndexerService) UpdateKeywords(workspaceID uint, rootDir string, extractor KeywordExtractor, batchSize int) (int, error) {
	var indexes []model.FileIndex
	s.DB.Where("workspace_id = ? AND keywords = ''", workspaceID).
		Limit(batchSize).Find(&indexes)

	if len(indexes) == 0 {
		return 0, nil
	}

	updated := 0
	for _, idx := range indexes {
		fullPath := filepath.Join(rootDir, filepath.FromSlash(idx.FilePath))
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		// 截断过长内容
		text := string(content)
		if len(text) > 4000 {
			text = text[:4000] + "\n...(已截断)"
		}

		result, err := extractor.Extract(idx.FilePath, idx.Language, text)
		if err != nil {
			slog.Warn("关键词提取失败", "file", idx.FilePath, "error", err)
			continue
		}

		s.DB.Model(&model.FileIndex{}).Where("id = ?", idx.ID).Updates(map[string]interface{}{
			"keywords": result.Keywords,
			"summary":  result.Summary,
		})
		updated++
	}

	return updated, nil
}

// GetIndexByWorkspace 获取工作区的所有索引
func (s *IndexerService) GetIndexByWorkspace(workspaceID uint) ([]model.FileIndex, error) {
	var indexes []model.FileIndex
	err := s.DB.Where("workspace_id = ?", workspaceID).
		Order("file_path ASC").Find(&indexes).Error
	return indexes, err
}

// SearchByKeywords 按关键词搜索索引
func (s *IndexerService) SearchByKeywords(workspaceID uint, keywords []string) ([]model.FileIndex, error) {
	if len(keywords) == 0 {
		return nil, nil
	}

	query := s.DB.Where("workspace_id = ?", workspaceID)

	// 构建 LIKE 查询（SQLite 不支持全文索引，用 LIKE 匹配）
	var conditions []string
	var args []interface{}
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		conditions = append(conditions, "(keywords LIKE ? OR symbols LIKE ? OR summary LIKE ?)")
		pattern := "%" + kw + "%"
		args = append(args, pattern, pattern, pattern)
	}

	if len(conditions) > 0 {
		query = query.Where(strings.Join(conditions, " OR "), args...)
	}

	var indexes []model.FileIndex
	err := query.Order("file_path ASC").Limit(50).Find(&indexes).Error
	return indexes, err
}

// KeywordExtractor 关键词提取器接口（由 LLM 实现）
type KeywordExtractor interface {
	Extract(filePath, language, content string) (*KeywordResult, error)
}

// KeywordResult 关键词提取结果
type KeywordResult struct {
	Keywords string // 逗号分隔的关键词
	Summary  string // 文件摘要
}

// --- 内部辅助函数 ---

// getExistingIndexMap 获取工作区现有索引（path -> FileIndex）
func (s *IndexerService) getExistingIndexMap(workspaceID uint) (map[string]model.FileIndex, error) {
	var indexes []model.FileIndex
	err := s.DB.Where("workspace_id = ?", workspaceID).Find(&indexes).Error
	if err != nil {
		return nil, err
	}

	m := make(map[string]model.FileIndex, len(indexes))
	for _, idx := range indexes {
		m[idx.FilePath] = idx
	}
	return m, nil
}

// fileHash 计算文件 SHA256 哈希
func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

// extractSymbols 从源代码中提取导出符号（简单的正则/字符串匹配）
func extractSymbols(path, lang string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	content := string(data)
	var symbols []string

	switch lang {
	case "go":
		symbols = extractGoSymbols(content)
	default:
		// 其他语言暂不支持符号提取，依赖 LLM 关键词
		return ""
	}

	if len(symbols) > 50 {
		symbols = symbols[:50]
	}
	return strings.Join(symbols, ",")
}

// extractGoSymbols 提取 Go 文件的导出符号
func extractGoSymbols(content string) []string {
	var symbols []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// func FuncName( 或 func (r *Type) MethodName(
		if strings.HasPrefix(line, "func ") {
			name := extractGoFuncName(line)
			if name != "" && isExported(name) {
				symbols = append(symbols, name)
			}
		}

		// type TypeName struct/interface
		if strings.HasPrefix(line, "type ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 && isExported(parts[1]) {
				symbols = append(symbols, parts[1])
			}
		}
	}

	return symbols
}

// extractGoFuncName 从 func 行提取函数/方法名
func extractGoFuncName(line string) string {
	// func Name(
	line = strings.TrimPrefix(line, "func ")

	// 跳过接收者 func (r *Type) Name(
	if strings.HasPrefix(line, "(") {
		idx := strings.Index(line, ")")
		if idx < 0 {
			return ""
		}
		line = strings.TrimSpace(line[idx+1:])
	}

	// 提取名称
	idx := strings.IndexByte(line, '(')
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(line[:idx])
}

// isExported 判断是否为导出标识符（首字母大写）
func isExported(name string) bool {
	if name == "" {
		return false
	}
	r := rune(name[0])
	return r >= 'A' && r <= 'Z'
}
