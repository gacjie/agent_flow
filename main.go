package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"agent_flow/src/common"
	"agent_flow/src/config"
	"agent_flow/src/database"
	applogger "agent_flow/src/logger"
	"agent_flow/src/model"
	"agent_flow/src/router"
	"agent_flow/src/service"
	"agent_flow/src/tool"
	"agent_flow/src/view"
)

func main() {
	// 1. 确保配置文件存在并补全缺失字段
	config.EnsureConfigFile("config/config.yaml", "config/default_config.yaml")

	// 2. 加载配置
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		slog.Error("配置加载失败", "error", err)
		os.Exit(1)
	}

	// 2. 初始化日志
	fileHandler := initLogger(cfg.Log)
	if fileHandler != nil {
		defer fileHandler.Close()
	}
	slog.Info("配置加载完成", "mode", cfg.Server.Mode)

	// 3. 初始化数据库
	if err := database.Init(&cfg.Database); err != nil {
		slog.Error("数据库初始化失败", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// 4. 迁移：将 "default" 模式统一为 "auto"
	database.Get().Exec("UPDATE agents SET model_id = 'auto' WHERE model_id = 'default'")
	database.Get().Exec("UPDATE agents SET model_mode = 'auto' WHERE model_mode = 'default'")

	// 4.2 迁移：删除 model_id 的历史唯一索引（旧版曾使用 uniqueIndex，允许多条相同 model_id 实现负载均衡）
	database.Get().Exec("DROP INDEX IF EXISTS `idx_llm_models_model_id`")

	// 4.3 迁移：supports_vision → capabilities 字段
	database.Get().Exec("UPDATE llm_models SET capabilities = 'vision,tools' WHERE supports_vision = 1 AND (capabilities = '' OR capabilities IS NULL)")
	database.Get().Exec("UPDATE llm_models SET capabilities = 'tools' WHERE (supports_vision = 0 OR supports_vision IS NULL) AND (capabilities = '' OR capabilities IS NULL)")

	// 5. 自动迁移
	if err := database.AutoMigrate(
		&model.Admin{},
		&model.Session{},
		&model.Role{},
		&model.Permission{},
		&model.RolePermission{},
		&model.SysConfig{},
		// AI 自动化系统
		&model.LLMModel{},
		&model.Skill{},
		&model.Agent{},
		&model.AgentSkill{},
		&model.Project{},
		&model.Workspace{},
		// 任务数据迁移至工作区 working.db，主 DB 不再存储任务
		// Phase 8: 文件索引
		&model.FileIndex{},
		// 工具系统（system_tools: 内置工具，mcp_tools: MCP工具）
		&model.SystemTool{},
		&model.MCPTool{},
		// APP API Token
		&model.APIToken{},
	); err != nil {
		slog.Error("数据库迁移失败", "error", err)
		os.Exit(1)
	}

	// 5. 种子数据
	seedData(cfg)

	// 5.1 初始化 WorkingDBManager（每个工作区独立 working.db）
	workingMgr := service.NewWorkingDBManager(cfg.Agent.WorkingRoot)
	defer workingMgr.CloseAll()

	// 6. 初始化工具注册表
	toolRegistry := tool.DefaultRegistry()
	mcpManager := tool.NewMCPManager(toolRegistry)
	toolService := service.NewToolService(database.Get(), toolRegistry)
	toolService.MCPManager = mcpManager

	// 7. 初始化服务
	permService := service.NewPermissionService(database.Get())
	configService := service.NewSysConfigService(database.Get())

	// 8. 初始化模板引擎
	engine, err := view.NewEngine()
	if err != nil {
		slog.Error("模板引擎初始化失败", "error", err)
		os.Exit(1)
	}

	// 9. 创建路由
	handler := router.New(database.Get(), engine, permService, configService, toolRegistry, toolService, workingMgr)

	// 9.1 同步系统工具到数据库（必须在 router.New 之后，确保管理工具已注册到 Registry）
	if err := toolService.SyncBuiltins(toolRegistry); err != nil {
		slog.Warn("系统工具同步失败", "error", err)
	}
	// 9.2 注册已启用的 MCP 工具到 Registry
	if err := toolService.SyncMCPEnabled(toolRegistry); err != nil {
		slog.Warn("MCP工具同步失败", "error", err)
	}

	// 10. 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	go func() {
		slog.Info("服务器启动", "addr", "http://"+addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("服务器异常退出", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("正在关闭服务器...")

	mcpManager.ShutdownAll()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("服务器关闭异常", "error", err)
	}
	slog.Info("服务器已关闭")
}

// seedData 种子全部初始数据（权限/角色/管理员/配置）
func seedData(cfg *config.AppConfig) {
	db := database.Get()

	// --- 权限节点 ---
	permissions := []model.Permission{
		{Code: "admin.dashboard", Label: "访问管理面板", Group: "后台"},
		{Code: "admin_mgmt.view", Label: "查看管理员列表", Group: "管理员"},
		{Code: "admin_mgmt.create", Label: "创建管理员", Group: "管理员"},
		{Code: "admin_mgmt.edit", Label: "编辑管理员", Group: "管理员"},
		{Code: "admin_mgmt.delete", Label: "删除管理员", Group: "管理员"},
		{Code: "role.view", Label: "查看角色", Group: "角色"},
		{Code: "role.edit", Label: "编辑角色权限", Group: "角色"},
		{Code: "system.config", Label: "系统配置", Group: "系统"},
		// 模型管理权限
		{Code: "model.view", Label: "查看模型", Group: "模型管理"},
		{Code: "model.create", Label: "添加模型", Group: "模型管理"},
		{Code: "model.edit", Label: "编辑模型", Group: "模型管理"},
		{Code: "model.delete", Label: "删除模型", Group: "模型管理"},
		{Code: "skill.view", Label: "查看技能", Group: "技能管理"},
		{Code: "skill.create", Label: "创建技能", Group: "技能管理"},
		{Code: "skill.edit", Label: "编辑技能", Group: "技能管理"},
		{Code: "skill.delete", Label: "删除技能", Group: "技能管理"},
		{Code: "agent.view", Label: "查看 AI 智能体", Group: "AI 智能体"},
		{Code: "agent.create", Label: "创建 AI 智能体", Group: "AI 智能体"},
		{Code: "agent.edit", Label: "编辑 AI 智能体", Group: "AI 智能体"},
		{Code: "agent.delete", Label: "删除 AI 智能体", Group: "AI 智能体"},
		{Code: "project.view", Label: "查看项目", Group: "项目管理"},
		{Code: "project.create", Label: "创建项目", Group: "项目管理"},
		{Code: "project.edit", Label: "编辑项目", Group: "项目管理"},
		{Code: "project.delete", Label: "删除项目", Group: "项目管理"},
		// Phase 4: 对话系统权限已并入工作台（workbench.*）
		// 工具管理权限
		{Code: "tool_mgmt.view", Label: "查看系统工具", Group: "系统工具"},
		{Code: "tool_mgmt.edit", Label: "管理系统工具", Group: "系统工具"},
		// 系统提示词权限
		{Code: "prompt.view", Label: "查看系统提示词", Group: "系统提示词"},
		{Code: "prompt.edit", Label: "编辑系统提示词", Group: "系统提示词"},
		// MCP工具权限
		{Code: "mcp_tool.view", Label: "查看MCP工具", Group: "MCP工具"},
		{Code: "mcp_tool.create", Label: "创建MCP工具", Group: "MCP工具"},
		{Code: "mcp_tool.edit", Label: "编辑MCP工具", Group: "MCP工具"},
		{Code: "mcp_tool.delete", Label: "删除MCP工具", Group: "MCP工具"},
		// 工作台权限
		{Code: "workbench.view", Label: "访问工作台", Group: "工作台"},
		{Code: "workbench.create", Label: "工作台操作", Group: "工作台"},
	}
	for _, p := range permissions {
		db.Where("code = ?", p.Code).FirstOrCreate(&p)
	}

	// --- 角色 ---
	roles := []model.Role{
		{Name: "admin", Label: "管理员", Sort: 1},
		{Name: "editor", Label: "编辑", Sort: 2},
		{Name: "user", Label: "普通用户", Sort: 3},
	}
	for i := range roles {
		db.Where("name = ?", roles[i].Name).FirstOrCreate(&roles[i])
	}

	// --- 角色-权限关联 ---
	var adminRole, editorRole model.Role
	db.Where("name = ?", "admin").First(&adminRole)
	db.Where("name = ?", "editor").First(&editorRole)

	// admin 角色：全部权限
	var existingRP int64
	db.Model(&model.RolePermission{}).Where("role_id = ?", adminRole.ID).Count(&existingRP)
	if existingRP == 0 {
		for _, p := range permissions {
			db.Create(&model.RolePermission{RoleID: adminRole.ID, PermissionCode: p.Code})
		}
		slog.Info("管理员角色权限已初始化")
	} else {
		// 补充新增的权限节点到 admin 角色
		for _, p := range permissions {
			rp := model.RolePermission{RoleID: adminRole.ID, PermissionCode: p.Code}
			db.Where("role_id = ? AND permission_code = ?", adminRole.ID, p.Code).FirstOrCreate(&rp)
		}
	}

	// editor 角色：部分权限
	db.Model(&model.RolePermission{}).Where("role_id = ?", editorRole.ID).Count(&existingRP)
	if existingRP == 0 {
		editorCodes := []string{"admin.dashboard", "admin_mgmt.view", "admin_mgmt.edit"}
		for _, code := range editorCodes {
			db.Create(&model.RolePermission{RoleID: editorRole.ID, PermissionCode: code})
		}
		slog.Info("编辑角色权限已初始化")
	}

	// --- 默认管理员 ---
	var adminCount int64
	db.Model(&model.Admin{}).Where("username = ?", cfg.Auth.AdminUser).Count(&adminCount)
	if adminCount == 0 {
		hashedPwd, err := common.HashPassword(cfg.Auth.AdminPassword)
		if err != nil {
			slog.Error("管理员密码哈希失败", "error", err)
			return
		}
		admin := &model.Admin{
			Username:           cfg.Auth.AdminUser,
			Email:              cfg.Auth.AdminUser + "@localhost",
			Password:           hashedPwd,
			Nickname:           "管理员",
			RoleID:             adminRole.ID,
			Status:             1,
			MustChangePassword: true,
		}
		db.Create(admin)
		slog.Info("默认管理员已创建", "username", cfg.Auth.AdminUser)
	}

	// --- 系统配置 ---
	configs := []model.SysConfig{
		{Key: "site.name", Value: "AgentFlow", Label: "站点名称", Group: "站点", Type: "text", Sort: 1},
		{Key: "site.description", Value: "智能体工作流系统", Label: "站点描述", Group: "站点", Type: "text", Sort: 2},
		{Key: "auth.session_max_age", Value: "86400", Label: "会话有效期(秒)", Group: "安全", Type: "number", Sort: 1},
		{Key: "auth.max_login_attempts", Value: "5", Label: "最大登录失败次数", Group: "安全", Type: "number", Sort: 2},
		{Key: "ai.vision_model", Value: "", Label: "视觉解析模型", Group: "AI 服务", Type: "model_select", Sort: 1},
		{Key: "ai.image_gen_model", Value: "", Label: "图片生成模型", Group: "AI 服务", Type: "model_select", Sort: 2},
		{Key: "ai.prompt_enhance_model", Value: "auto", Label: "提示词增强模型", Group: "AI 服务", Type: "model_select", Sort: 3},
		{Key: "search.engine", Value: "auto", Label: "搜索引擎", Group: "搜索", Type: "select", Options: `[{"value":"auto","label":"自动切换（SearXNG→Jina→DuckDuckGo）"},{"value":"searxng","label":"SearXNG"},{"value":"jina","label":"Jina AI"},{"value":"duckduckgo","label":"DuckDuckGo"}]`, Sort: 1},
		{Key: "search.searxng_url", Value: "https://search.inetol.net", Label: "SearXNG 实例地址", Group: "搜索", Type: "text", Sort: 2},
		{Key: "search.jina_api_key", Value: "", Label: "Jina API Key（可选，提高额度）", Group: "搜索", Type: "text", Sort: 3},
	}
	for _, c := range configs {
		db.Where("`key` = ?", c.Key).FirstOrCreate(&c)
	}
	// 确保 AI 模型配置字段类型为 model_select（升级兼容）
	db.Model(&model.SysConfig{}).Where("`key` LIKE ?", "ai.%").Update("type", "model_select")

	// --- SecretKey 持久化（config.yaml 未配置时，生成后直接写回配置文件）---
	if cfg.App.SecretKey == "" {
		key, err := common.GenerateSecretKey()
		if err != nil {
			slog.Error("SecretKey 生成失败", "error", err)
		} else {
			cfg.App.SecretKey = key
			if err := writeSecretKeyToConfig("config/config.yaml", key); err != nil {
				slog.Warn("SecretKey 写入 config.yaml 失败，本次运行有效但重启后失效", "error", err)
			} else {
				slog.Info("已自动生成 SecretKey 并写入 config/config.yaml")
			}
		}
	}

	// --- 内置技能（从 context/skills/ 目录扫描加载） ---
	skillSeedSvc := service.NewSkillSeedService(db)
	if err := skillSeedSvc.Seed(cfg.Agent.ContextRoot); err != nil {
		slog.Warn("内置技能加载失败", "error", err)
	}

	// --- 内置 AI 智能体（从 context/agents/ 目录扫描加载） ---
	agentSeedSvc := service.NewAgentSeedService(db)
	if err := agentSeedSvc.Seed(cfg.Agent.ContextRoot); err != nil {
		slog.Warn("内置智能体加载失败", "error", err)
	}
}

// writeSecretKeyToConfig 将生成的 SecretKey 回写到 config.yaml
func writeSecretKeyToConfig(path, key string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := strings.Replace(string(data), `secret_key: ""`, `secret_key: "`+key+`"`, 1)
	return os.WriteFile(path, []byte(updated), 0644)
}

// initLogger 初始化 slog 日志，返回文件 Handler 供调用方在退出时 Close
func initLogger(cfg config.LogConfig) *applogger.DailyFileHandler {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var consoleHandler slog.Handler
	if cfg.Format == "json" {
		consoleHandler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		consoleHandler = slog.NewTextHandler(os.Stdout, opts)
	}

	// 初始化错误日志文件 Handler（warn 及以上写入按天轮转的文件）
	errDir := cfg.ErrorLogDir
	if errDir == "" {
		errDir = "logs/error"
	}
	if err := os.MkdirAll(errDir, 0755); err != nil {
		slog.SetDefault(slog.New(consoleHandler))
		slog.Warn("无法创建错误日志目录，跳过文件日志", "dir", errDir, "error", err)
		return nil
	}
	fileHandler := applogger.NewDailyFileHandler(errDir, slog.LevelWarn)
	slog.SetDefault(slog.New(applogger.NewMultiHandler(consoleHandler, fileHandler)))
	return fileHandler
}

