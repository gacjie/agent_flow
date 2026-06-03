package config

import (
	"log/slog"
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
	ErrorLogDir string `mapstructure:"error_log_dir"` // 错误日志目录，默认 logs/error
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
	Version   string `mapstructure:"version"`
	SecretKey string `mapstructure:"secret_key"`
}

// AgentConfig 智能体系统配置
type AgentConfig struct {
	ContextRoot    string `mapstructure:"context_root"`
	ProjectRoot    string `mapstructure:"project_root"`
	WorkingRoot    string `mapstructure:"working_root"`
	SystemRoot     string `mapstructure:"system_root"`      // 系统目录（工具+系统角色）
	MaxIterations  int    `mapstructure:"max_iterations"`   // 工具调用自检间隔轮数
	TidyKeepRounds int    `mapstructure:"tidy_keep_rounds"` // 上下文整理保留轮数，默认 5
}

type PaginationConfig struct {
	DefaultPageSize int `mapstructure:"default_page_size"`
	MaxPageSize     int `mapstructure:"max_page_size"`
}

// LLMConfig LLM 调用配置
type LLMConfig struct {
	Timeout    int `mapstructure:"timeout"`
	MaxRetries int `mapstructure:"max_retries"`
}

var cfg *AppConfig

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
	v.SetDefault("app.version", "1.0.0")
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
