package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

// EnvPrefix 环境变量前缀，如 AF_SERVER_PORT=9090
const EnvPrefix = "AF"

// AppConfig 应用全局配置结构体
type AppConfig struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Log        LogConfig        `mapstructure:"log"`
	Auth       AuthConfig       `mapstructure:"auth"`
	CORS       CORSConfig       `mapstructure:"cors"`
	App        AppInfo          `mapstructure:"app"`
	Pagination PaginationConfig `mapstructure:"pagination"`
	Agent      AgentConfig      `mapstructure:"agent"`
	LLM        LLMConfig        `mapstructure:"llm"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	AllowedMethods string   `mapstructure:"allowed_methods"`
	AllowedHeaders string   `mapstructure:"allowed_headers"`
}

type ServerConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Mode            string `mapstructure:"mode"`
	ReadTimeout     int    `mapstructure:"read_timeout"`
	WriteTimeout    int    `mapstructure:"write_timeout"`
	IdleTimeout     int    `mapstructure:"idle_timeout"`
	ShutdownTimeout int    `mapstructure:"shutdown_timeout"`
}

type DatabaseConfig struct {
	Driver       string `mapstructure:"driver"`
	DSN          string `mapstructure:"dsn"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

type LogConfig struct {
	Level       string `mapstructure:"level"`
	Format      string `mapstructure:"format"`
	ErrorLogDir string `mapstructure:"error_log_dir"`
}

type AuthConfig struct {
	SessionMaxAge     int    `mapstructure:"session_max_age"`
	CookieSecure      bool   `mapstructure:"cookie_secure"`
	CookieHTTPOnly    bool   `mapstructure:"cookie_http_only"`
	AdminUser         string `mapstructure:"admin_user"`
	AdminPassword     string `mapstructure:"admin_password"`
	MaxLoginAttempts  int    `mapstructure:"max_login_attempts"`
	LockDuration      int    `mapstructure:"lock_duration"`
	SessionCacheTTL   int    `mapstructure:"session_cache_ttl"`
	CaptchaTTL        int    `mapstructure:"captcha_ttl"`
	MinPasswordLength int    `mapstructure:"min_password_length"`
}

type AppInfo struct {
	Name      string `mapstructure:"name"`
	SecretKey string `mapstructure:"secret_key"`
}

// AgentConfig 智能体系统配置
type AgentConfig struct {
	ContextRoot    string `mapstructure:"context_root"`
	ProjectRoot    string `mapstructure:"project_root"`
	WorkingRoot    string `mapstructure:"working_root"`
	SystemRoot     string `mapstructure:"system_root"`
	MaxIterations  int    `mapstructure:"max_iterations"`
	TidyKeepRounds int    `mapstructure:"tidy_keep_rounds"`
}

type PaginationConfig struct {
	DefaultPageSize int `mapstructure:"default_page_size"`
	MaxPageSize     int `mapstructure:"max_page_size"`
}

// LLMConfig LLM 调用配置
type LLMConfig struct {
	Timeout           int `mapstructure:"timeout"`
	MaxRetries        int `mapstructure:"max_retries"`
	StreamIdleTimeout int `mapstructure:"stream_idle_timeout"`
}

var cfg *AppConfig

// EnsureConfigFile 确保 config.yaml 存在且字段完整
func EnsureConfigFile(configPath, defaultPath string) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// config.yaml 不存在，从 default_config.yaml 复制
		if defaultData, readErr := os.ReadFile(defaultPath); readErr == nil {
			os.MkdirAll(filepath.Dir(configPath), 0755)
			if writeErr := os.WriteFile(configPath, defaultData, 0644); writeErr != nil {
				slog.Warn("自动生成 config.yaml 失败", "error", writeErr)
			} else {
				slog.Info("已从 default_config.yaml 自动生成 config.yaml")
			}
		} else {
			slog.Warn("default_config.yaml 不存在，将使用内置默认值", "error", readErr)
		}
		return
	}

	// config.yaml 存在，检查并补全缺失字段
	if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
		return
	}
	if err := mergeNewFields(configPath, defaultPath); err != nil {
		slog.Warn("配置字段补全失败", "error", err)
	}
}

// mergeNewFields 将 default 中有但 config 中缺失的字段追加到 config 对应 section
func mergeNewFields(configPath, defaultPath string) error {
	// 用 Viper 加载两个文件获取所有 key
	vDefault := viper.New()
	vDefault.SetConfigFile(defaultPath)
	vDefault.SetConfigType("yaml")
	if err := vDefault.ReadInConfig(); err != nil {
		return fmt.Errorf("读取 default_config.yaml 失败: %w", err)
	}

	vUser := viper.New()
	vUser.SetConfigFile(configPath)
	vUser.SetConfigType("yaml")
	if err := vUser.ReadInConfig(); err != nil {
		return fmt.Errorf("读取 config.yaml 失败: %w", err)
	}

	// 找出 default 有但 user 没有的 key
	defaultKeys := vDefault.AllKeys()
	userKeySet := make(map[string]bool)
	for _, k := range vUser.AllKeys() {
		userKeySet[k] = true
	}

	// 按 section 分组收集缺失字段
	type fieldEntry struct {
		key   string
		value interface{}
	}
	missingBySection := make(map[string][]fieldEntry)

	for _, key := range defaultKeys {
		if userKeySet[key] {
			continue
		}
		parts := strings.SplitN(key, ".", 2)
		if len(parts) != 2 {
			continue
		}
		section := parts[0]
		missingBySection[section] = append(missingBySection[section], fieldEntry{
			key:   parts[1],
			value: vDefault.Get(key),
		})
	}

	if len(missingBySection) == 0 {
		return nil
	}

	// 读取 config.yaml 原始内容，逐 section 插入缺失字段
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")

	// 正则：匹配顶级 section 行（无缩进，以冒号结尾）
	sectionRe := regexp.MustCompile(`^([a-z_]+)\s*:`)

	// 找到每个 section 的行范围
	type sectionRange struct {
		name      string
		startLine int
		endLine   int // section 最后一个非空行的索引
	}
	var sections []sectionRange
	for i, line := range lines {
		if m := sectionRe.FindStringSubmatch(line); m != nil {
			sections = append(sections, sectionRange{name: m[1], startLine: i, endLine: i})
		}
	}
	// 计算每个 section 的结束行
	for i := range sections {
		var end int
		if i < len(sections)-1 {
			end = sections[i+1].startLine - 1
		} else {
			end = len(lines) - 1
		}
		// 找到最后一个非空行
		for end > sections[i].startLine && strings.TrimSpace(lines[end]) == "" {
			end--
		}
		sections[i].endLine = end
	}

	// 从后往前插入，避免行号偏移
	inserted := false
	for i := len(sections) - 1; i >= 0; i-- {
		sec := sections[i]
		missing, ok := missingBySection[sec.name]
		if !ok || len(missing) == 0 {
			continue
		}

		// 构建要插入的行
		var newLines []string
		for _, f := range missing {
			line := fmt.Sprintf("  %s: %s     # [v%s 新增]", f.key, formatYAMLValue(f.value), Version)
			newLines = append(newLines, line)
		}

		// 插入到 section 最后一行之后
		insertIdx := sec.endLine + 1
		lines = insertLines(lines, insertIdx, newLines)
		inserted = true
		delete(missingBySection, sec.name)
	}

	// 处理 config.yaml 中完全不存在的 section
	for section, fields := range missingBySection {
		var newLines []string
		newLines = append(newLines, "")
		newLines = append(newLines, section+":")
		for _, f := range fields {
			line := fmt.Sprintf("  %s: %s     # [v%s 新增]", f.key, formatYAMLValue(f.value), Version)
			newLines = append(newLines, line)
		}
		lines = append(lines, newLines...)
		inserted = true
	}

	if !inserted {
		return nil
	}

	result := strings.Join(lines, "\n")
	if err := os.WriteFile(configPath, []byte(result), 0644); err != nil {
		return err
	}
	slog.Info("已自动补全 config.yaml 缺失字段")
	return nil
}

func insertLines(lines []string, idx int, newLines []string) []string {
	if idx >= len(lines) {
		return append(lines, newLines...)
	}
	result := make([]string, 0, len(lines)+len(newLines))
	result = append(result, lines[:idx]...)
	result = append(result, newLines...)
	result = append(result, lines[idx:]...)
	return result
}

func formatYAMLValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		if val == "" {
			return `""`
		}
		if strings.ContainsAny(val, ": #{}[]|>&*!%@") || strings.Contains(val, "//") {
			return fmt.Sprintf("%q", val)
		}
		return fmt.Sprintf("%q", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int, int64:
		return fmt.Sprintf("%d", val)
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case []interface{}:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			parts = append(parts, formatYAMLValue(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Load 从配置文件和环境变量加载配置
func Load(configPath string) (*AppConfig, error) {
	v := viper.New()

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// 环境变量覆盖，前缀 AF_，如 AF_SERVER_PORT=9090
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 默认值
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.read_timeout", 15)
	v.SetDefault("server.write_timeout", 300)
	v.SetDefault("server.idle_timeout", 60)
	v.SetDefault("server.shutdown_timeout", 10)
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "data.db")
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.max_open_conns", 100)
	v.SetDefault("log.level", "debug")
	v.SetDefault("log.format", "text")
	v.SetDefault("log.error_log_dir", "logs/error")
	v.SetDefault("auth.session_max_age", 86400)
	v.SetDefault("auth.cookie_secure", false)
	v.SetDefault("auth.cookie_http_only", true)
	v.SetDefault("auth.admin_user", "admin")
	v.SetDefault("auth.admin_password", "admin123")
	v.SetDefault("auth.max_login_attempts", 5)
	v.SetDefault("auth.lock_duration", 15)
	v.SetDefault("auth.session_cache_ttl", 300)
	v.SetDefault("auth.captcha_ttl", 300)
	v.SetDefault("auth.min_password_length", 8)
	v.SetDefault("cors.allowed_origins", []string{"http://localhost:8080"})
	v.SetDefault("cors.allowed_methods", "GET, POST, PUT, DELETE, OPTIONS")
	v.SetDefault("cors.allowed_headers", "Content-Type, Authorization, X-Requested-With, X-CSRF-Token")
	v.SetDefault("app.name", "AgentFlow")
	v.SetDefault("app.secret_key", "")
	v.SetDefault("pagination.default_page_size", 20)
	v.SetDefault("pagination.max_page_size", 100)
	v.SetDefault("agent.context_root", "./context")
	v.SetDefault("agent.project_root", "./project")
	v.SetDefault("agent.working_root", "./workspace")
	v.SetDefault("agent.system_root", "./system")
	v.SetDefault("agent.max_iterations", 50)
	v.SetDefault("agent.tidy_keep_rounds", 5)
	v.SetDefault("llm.timeout", 120)
	v.SetDefault("llm.max_retries", 2)
	v.SetDefault("llm.stream_idle_timeout", 300)

	if err := v.ReadInConfig(); err != nil {
		slog.Warn("配置文件加载失败，使用默认值", "error", err)
	}

	cfg = &AppConfig{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Get 获取全局配置（需先调用 Load）
func Get() *AppConfig {
	return cfg
}
