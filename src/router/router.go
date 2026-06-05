package router

import (
	"io/fs"
	"net/http"

	"agent_flow/src/controller"
	"agent_flow/src/middleware"
	"agent_flow/src/service"
	"agent_flow/src/tool"
	"agent_flow/src/view"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// New 创建并配置路由器
func New(db *gorm.DB, engine *view.Engine, permService *service.PermissionService, configService *service.SysConfigService, toolRegistry *tool.Registry, toolService *service.ToolService, workingMgr *service.WorkingDBManager) http.Handler {
	r := chi.NewRouter()

	// 全局中间件
	r.Use(middleware.Security)
	r.Use(middleware.Recovery)
	r.Use(middleware.Logger)

	// 初始化服务
	fileSyncService := service.NewFileSyncService(db)
	authService := service.NewAuthService(db)
	adminService := service.NewAdminService(db)
	captchaService := service.NewCaptchaService()
	modelService := service.NewLLMProviderService(db)
	modelService.SysConfigService = configService
	skillService := service.NewSkillService(db, fileSyncService)
	agentService := service.NewAgentService(db, fileSyncService)
	projectService := service.NewProjectService(db)
	workspaceService := service.NewWorkspaceService(db, workingMgr)
	// Phase 4: 对话系统服务
	chatService := service.NewChatService(db, workingMgr)
	chatRunner := service.NewChatRunner(chatService, modelService, toolRegistry)
	// RunnerManager：后台任务管理器（独立于 HTTP 连接）
	runnerMgr := service.NewRunnerManager()
	chatRunner.RunnerMgr = runnerMgr
	chatRunner.AgentService = agentService
	// Phase 6: 任务系统服务（工作区级，操作 working.db）
	taskService := service.NewTaskService(workingMgr)
	// Phase 7: 上下文整理 + 记忆
	tidyUpService := service.NewTidyUpService(chatService, taskService, modelService)
	chatRunner.TaskService = taskService
	chatRunner.TidyUpService = tidyUpService
	// Phase 8: 上下文索引
	indexerService := service.NewIndexerService(db)
	contextMatcher := service.NewContextMatcher(indexerService)
	chatRunner.ContextMatcher = contextMatcher
	// 系统提示词管理
	promptService := service.NewPromptService()
	chatRunner.PromptService = promptService
	tidyUpService.PromptService = promptService
	// 视觉代理服务
	visionService := service.NewVisionService(modelService, configService)
	visionService.PromptService = promptService
	chatRunner.VisionService = visionService
	// 提示词增强服务
	promptEnhanceService := service.NewPromptEnhanceService(modelService, configService, promptService)

	// 注册 Agent 管理工具（供智能体通过 tool_call 调用）
	tool.RegisterAgentTools(toolRegistry, agentService, chatRunner)
	// 注册任务管理工具（供智能体通过 tool_call 操作工作区任务）
	tool.RegisterTaskTools(toolRegistry, taskService)
	// 注册图片生成工具（需要 ModelGetter + ConfigGetter 依赖）
	toolRegistry.Register(&tool.ImageGenTool{
		ConfigGetter: configService,
		ModelGetter:  modelService,
	})

	// 静态文件服务
	staticFS, _ := fs.Sub(view.StaticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// 初始化控制器
	base := controller.Base{View: engine}
	authCtrl := &controller.Auth{Base: base, AuthService: authService, CaptchaService: captchaService}
	profileCtrl := &controller.Profile{Base: base, AdminService: adminService}
	dashboardCtrl := &controller.AdminDashboard{
		Base:             base,
		AdminService:     adminService,
		PermService:      permService,
		ProjectService:   projectService,
		AgentService:     agentService,
		SkillService:     skillService,
		WorkspaceService: workspaceService,
		ModelService:     modelService,
		ToolService:      toolService,
	}
	memberCtrl := &controller.Member{Base: base, Service: adminService, PermService: permService}
	roleCtrl := &controller.RoleCtrl{Base: base, PermService: permService}
	configCtrl := &controller.SysConfigCtrl{Base: base, ConfigService: configService, ModelService: modelService}
	providerCtrl := &controller.LLMProviderCtrl{Base: base, Service: modelService}
	skillCtrl := &controller.SkillCtrl{Base: base, Service: skillService}
	toolCtrl := &controller.SystemToolCtrl{Base: base, ToolService: toolService}
	mcpToolCtrl := &controller.MCPToolCtrl{Base: base, ToolService: toolService}
	agentCtrl := &controller.AgentCtrl{Base: base, Service: agentService, ModelService: modelService, SkillService: skillService, ToolService: toolService}
	promptCtrl := &controller.PromptCtrl{Base: base, PromptService: promptService}
	projectCtrl := &controller.ProjectCtrl{Base: base, Service: projectService}
	workspaceCtrl := &controller.WorkspaceCtrl{Base: base, Service: workspaceService, ProjectService: projectService, AgentService: agentService}
	taskCtrl := &controller.TaskCtrl{Base: base, TaskService: taskService, WorkspaceService: workspaceService}
	workbenchCtrl := &controller.WorkbenchCtrl{
		Base:                 base,
		ChatService:          chatService,
		ChatRunner:           chatRunner,
		WorkspaceService:     workspaceService,
		AgentService:         agentService,
		ProjectService:       projectService,
		ModelService:         modelService,
		RunnerManager:        runnerMgr,
		TaskService:          taskService,
		PromptEnhanceService: promptEnhanceService,
	}

	// ========== APP API 路由（豁免 CSRF）==========
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/login", authCtrl.APILogin)
		r.Post("/logout", authCtrl.APILogout)
	})

	// ========== Web 页面路由 ==========
	r.Group(func(r chi.Router) {
		r.Use(middleware.CORS())
		r.Use(middleware.CSRF)
		r.Use(middleware.InjectAdmin(authService, permService))

		// 公开页面
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/admin", http.StatusFound)
		})
		r.Get("/auth/login", authCtrl.LoginForm)
		r.Post("/auth/login", authCtrl.Login)
		r.Get("/auth/captcha", authCtrl.CaptchaRefresh)
		r.Post("/auth/logout", authCtrl.Logout)

		// 需要登录
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(authService, permService))

			r.Route("/admin", func(r chi.Router) {
				r.With(middleware.RequirePermission("admin.dashboard")).Get("/", dashboardCtrl.Dashboard)

				r.Get("/profile", profileCtrl.Edit)
				r.Post("/profile", profileCtrl.Update)
				r.Get("/password", profileCtrl.PasswordForm)
				r.Post("/password", profileCtrl.PasswordUpdate)

				r.Route("/members", func(r chi.Router) {
					r.With(middleware.RequirePermission("admin_mgmt.view")).Get("/", memberCtrl.List)
					r.With(middleware.RequirePermission("admin_mgmt.create")).Get("/new", memberCtrl.CreateForm)
					r.With(middleware.RequirePermission("admin_mgmt.create")).Post("/", memberCtrl.Create)
					r.With(middleware.RequirePermission("admin_mgmt.edit")).Get("/{id}/edit", memberCtrl.EditForm)
					r.With(middleware.RequirePermission("admin_mgmt.edit")).Post("/{id}", memberCtrl.Update)
					r.With(middleware.RequirePermission("admin_mgmt.delete")).Delete("/{id}", memberCtrl.Delete)
				})

				r.Route("/roles", func(r chi.Router) {
					r.With(middleware.RequirePermission("role.view")).Get("/", roleCtrl.List)
					r.With(middleware.RequirePermission("role.edit")).Get("/{id}", roleCtrl.EditForm)
					r.With(middleware.RequirePermission("role.edit")).Post("/{id}", roleCtrl.Update)
				})

				r.With(middleware.RequirePermission("system.config")).Get("/config", configCtrl.Edit)
				r.With(middleware.RequirePermission("system.config")).Post("/config", configCtrl.Update)

				// 模型管理路由
				r.Route("/models", func(r chi.Router) {
					r.With(middleware.RequirePermission("model.view")).Get("/", providerCtrl.List)
					r.With(middleware.RequirePermission("model.create")).Get("/new", providerCtrl.CreateForm)
					r.With(middleware.RequirePermission("model.create")).Post("/", providerCtrl.Create)
					r.With(middleware.RequirePermission("model.view")).Post("/export", providerCtrl.Export)
					r.With(middleware.RequirePermission("model.create")).Post("/import", providerCtrl.Import)
					r.With(middleware.RequirePermission("model.create")).Post("/fetch-upstream", providerCtrl.FetchModels)
					r.With(middleware.RequirePermission("model.edit")).Get("/{id}/edit", providerCtrl.EditForm)
					r.With(middleware.RequirePermission("model.edit")).Post("/{id}", providerCtrl.Update)
					r.With(middleware.RequirePermission("model.delete")).Delete("/{id}", providerCtrl.Delete)
					r.With(middleware.RequirePermission("model.edit")).Post("/{id}/toggle-auto", providerCtrl.ToggleAuto)
					r.With(middleware.RequirePermission("model.view")).Get("/{id}/test", providerCtrl.TestConnect)
					r.With(middleware.RequirePermission("model.edit")).Post("/{id}/fetch-upstream", providerCtrl.FetchModels)
				})

				r.Route("/skills", func(r chi.Router) {
					r.With(middleware.RequirePermission("skill.view")).Get("/", skillCtrl.List)
					r.With(middleware.RequirePermission("skill.create")).Get("/new", skillCtrl.CreateForm)
					r.With(middleware.RequirePermission("skill.create")).Post("/", skillCtrl.Create)
					r.With(middleware.RequirePermission("skill.edit")).Get("/{id}/edit", skillCtrl.EditForm)
					r.With(middleware.RequirePermission("skill.edit")).Post("/{id}", skillCtrl.Update)
					r.With(middleware.RequirePermission("skill.delete")).Delete("/{id}", skillCtrl.Delete)
				})

				// 系统工具（原 /tools 改为 /system-tools）
				r.Route("/system-tools", func(r chi.Router) {
					r.With(middleware.RequirePermission("tool_mgmt.view")).Get("/", toolCtrl.List)
					r.With(middleware.RequirePermission("tool_mgmt.edit")).Post("/{id}/toggle", toolCtrl.Toggle)
				})
				// MCP 工具（全 CRUD）
				r.Route("/mcp-tools", func(r chi.Router) {
					r.With(middleware.RequirePermission("mcp_tool.view")).Get("/", mcpToolCtrl.List)
					r.With(middleware.RequirePermission("mcp_tool.view")).Get("/statuses", mcpToolCtrl.Statuses)
					r.With(middleware.RequirePermission("mcp_tool.create")).Post("/import", mcpToolCtrl.Import)
					r.With(middleware.RequirePermission("mcp_tool.view")).Post("/export", mcpToolCtrl.Export)
					r.With(middleware.RequirePermission("mcp_tool.edit")).Get("/{id}/edit", mcpToolCtrl.EditForm)
					r.With(middleware.RequirePermission("mcp_tool.edit")).Post("/{id}", mcpToolCtrl.Update)
					r.With(middleware.RequirePermission("mcp_tool.delete")).Delete("/{id}", mcpToolCtrl.Delete)
					r.With(middleware.RequirePermission("mcp_tool.edit")).Post("/{id}/toggle", mcpToolCtrl.Toggle)
					r.With(middleware.RequirePermission("mcp_tool.edit")).Post("/{id}/start", mcpToolCtrl.Start)
					r.With(middleware.RequirePermission("mcp_tool.edit")).Post("/{id}/stop", mcpToolCtrl.Stop)
				})
				// 系统提示词管理
				r.Route("/prompts", func(r chi.Router) {
					r.With(middleware.RequirePermission("prompt.view")).Get("/", promptCtrl.List)
					r.With(middleware.RequirePermission("prompt.edit")).Get("/{name}/edit", promptCtrl.Edit)
					r.With(middleware.RequirePermission("prompt.edit")).Post("/{name}", promptCtrl.Update)
					r.With(middleware.RequirePermission("prompt.edit")).Post("/{name}/reset", promptCtrl.Reset)
				})

				r.Route("/agents", func(r chi.Router) {
					r.With(middleware.RequirePermission("agent.view")).Get("/", agentCtrl.List)
					r.With(middleware.RequirePermission("agent.create")).Get("/new", agentCtrl.CreateForm)
					r.With(middleware.RequirePermission("agent.create")).Post("/", agentCtrl.Create)
					r.With(middleware.RequirePermission("agent.edit")).Get("/{id}/edit", agentCtrl.EditForm)
					r.With(middleware.RequirePermission("agent.edit")).Post("/{id}", agentCtrl.Update)
					r.With(middleware.RequirePermission("agent.delete")).Delete("/{id}", agentCtrl.Delete)
				})

				r.Route("/projects", func(r chi.Router) {
					r.With(middleware.RequirePermission("project.view")).Get("/", projectCtrl.List)
					r.With(middleware.RequirePermission("project.view")).Get("/browse-dir", projectCtrl.BrowseDir)
					r.With(middleware.RequirePermission("project.create")).Get("/new", projectCtrl.CreateForm)
					r.With(middleware.RequirePermission("project.create")).Post("/", projectCtrl.Create)
					r.With(middleware.RequirePermission("project.edit")).Get("/{id}/edit", projectCtrl.EditForm)
					r.With(middleware.RequirePermission("project.edit")).Post("/{id}", projectCtrl.Update)
					r.With(middleware.RequirePermission("project.delete")).Delete("/{id}", projectCtrl.Delete)

					r.Route("/{projectID}/workspaces", func(r chi.Router) {
						r.With(middleware.RequirePermission("project.view")).Get("/", workspaceCtrl.List)
						r.With(middleware.RequirePermission("project.create")).Get("/new", workspaceCtrl.CreateForm)
						r.With(middleware.RequirePermission("project.create")).Post("/", workspaceCtrl.Create)
						r.With(middleware.RequirePermission("project.edit")).Get("/{id}/edit", workspaceCtrl.EditForm)
						r.With(middleware.RequirePermission("project.edit")).Post("/{id}", workspaceCtrl.Update)
						r.With(middleware.RequirePermission("project.delete")).Delete("/{id}", workspaceCtrl.Delete)
					})
				})

				// Phase 6: 任务展示（嵌套在工作区下，只读）
				r.Route("/projects/{projectID}/workspaces/{workspaceID}/tasks", func(r chi.Router) {
					r.With(middleware.RequirePermission("project.view")).Get("/", taskCtrl.List)
				})

				// 工作台路由
				r.Route("/workbench", func(r chi.Router) {
					r.With(middleware.RequirePermission("workbench.view")).Get("/", workbenchCtrl.Page)
					r.With(middleware.RequirePermission("workbench.view")).Get("/conversations", workbenchCtrl.ListConversations)
					r.With(middleware.RequirePermission("workbench.create")).Post("/conversations", workbenchCtrl.CreateConversation)
					r.With(middleware.RequirePermission("workbench.view")).Get("/conversations/{id}/messages", workbenchCtrl.GetMessages)
					r.With(middleware.RequirePermission("workbench.create")).Post("/conversations/{id}/send", workbenchCtrl.SendMessage)
					r.With(middleware.RequirePermission("workbench.create")).Post("/conversations/{id}/stop", workbenchCtrl.StopConversation)
					r.With(middleware.RequirePermission("workbench.create")).Delete("/conversations/{id}", workbenchCtrl.DeleteConversation)
					r.With(middleware.RequirePermission("workbench.create")).Post("/workspaces", workbenchCtrl.CreateWorkspace)
					r.With(middleware.RequirePermission("workbench.view")).Get("/workspaces", workbenchCtrl.ListWorkspaces)
					r.With(middleware.RequirePermission("workbench.view")).Get("/tasks", workbenchCtrl.ListTasks)
					r.With(middleware.RequirePermission("workbench.view")).Get("/docs", workbenchCtrl.ListDocs)
					r.With(middleware.RequirePermission("workbench.view")).Get("/docs/content", workbenchCtrl.ReadDoc)
					r.With(middleware.RequirePermission("workbench.view")).Get("/project-docs", workbenchCtrl.ListProjectDocs)
					r.With(middleware.RequirePermission("workbench.view")).Get("/project-docs/content", workbenchCtrl.ReadProjectDoc)
					r.With(middleware.RequirePermission("workbench.view")).Get("/project-files", workbenchCtrl.ListProjectFiles)
					r.With(middleware.RequirePermission("workbench.view")).Get("/project-files/content", workbenchCtrl.ReadProjectFile)
					r.With(middleware.RequirePermission("workbench.create")).Post("/docs/save", workbenchCtrl.SaveDoc)
					r.With(middleware.RequirePermission("workbench.create")).Post("/project-docs/save", workbenchCtrl.SaveProjectDoc)
					r.With(middleware.RequirePermission("workbench.create")).Post("/project-files/save", workbenchCtrl.SaveProjectFile)
					r.With(middleware.RequirePermission("workbench.create")).Post("/upload", workbenchCtrl.UploadFile)
					r.With(middleware.RequirePermission("workbench.view")).Get("/uploads", workbenchCtrl.ServeUpload)
					r.With(middleware.RequirePermission("workbench.view")).Get("/uploads/list", workbenchCtrl.ListUploads)
					r.With(middleware.RequirePermission("workbench.create")).Post("/uploads/save", workbenchCtrl.SaveUpload)
					r.With(middleware.RequirePermission("workbench.create")).Post("/enhance-prompt", workbenchCtrl.EnhancePrompt)
				})
			})
		})
	})

	// 404
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		engine.RenderError(w, http.StatusNotFound, "页面不存在")
	})

	return r
}
