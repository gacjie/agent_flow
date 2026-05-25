# AgentFlow

[English](README.md)

开箱即用的 AI 智能体工作流系统。基于 Go 语言构建，单二进制部署，内嵌前端界面和 SQLite 数据库。

## 特性

- **多智能体编排** — 内置智能体 + 自定义创建，支持角色/规则/工作流/技能/记忆配置
- **实时工作台** — 基于 SSE 的流式对话界面，支持断线重连
- **28 个内置工具** — 文件操作、Shell 命令、浏览器自动化、记忆管理、任务规划等
- **MCP 工具集成** — 通过 Model Context Protocol 扩展外部工具（stdio JSON-RPC）
- **多 LLM 支持** — OpenAI、Anthropic Claude、Google Gemini 及 OpenAI 兼容 API
- **分阶段任务管理** — 将复杂工作拆解为带依赖关系的分阶段任务
- **项目与工作区隔离** — 每个工作区拥有独立数据库和文件目录
- **RBAC 权限控制** — 基于角色的权限系统，34 个权限节点
- **日夜双主题** — 原生 CSS/JS 前端，支持明暗主题切换
- **零依赖部署** — 单二进制文件，无需外部运行时

## 架构

```
┌─────────────────────────────────────────────────────────┐
│                       前端层                             │
│         原生 CSS/JS + html/template + go:embed           │
├─────────────────────────────────────────────────────────┤
│                      HTTP 层                             │
│        Chi v5 路由 + 中间件栈（CSRF、认证、               │
│        CORS、安全头、异常恢复）                           │
├─────────────────────────────────────────────────────────┤
│                      控制器层                            │
│       21 个控制器（CRUD + 工作台 SSE + 文档）            │
├─────────────────────────────────────────────────────────┤
│                      服务层                              │
│  ChatRunner │ RunnerManager │ TidyUp │ TaskPlanner      │
│  Auth │ Agent │ Skill │ Tool │ Workspace │ Indexer      │
├─────────────────────────────────────────────────────────┤
│                     工具系统                             │
│  Registry + Executor + MCP Manager + 28 个内置工具      │
├──────────────────────┬──────────────────────────────────┤
│     LLM 提供商       │           数据层                  │
│  OpenAI │ Anthropic  │  GORM + SQLite（主库）            │
│  Gemini │ Responses  │  每工作区独立 working.db          │
└──────────────────────┴──────────────────────────────────┘
```

**技术栈：**

| 层级 | 技术 |
|------|------|
| 语言 | Go 1.25 |
| 路由 | chi/v5 |
| ORM | GORM（纯 Go SQLite 驱动） |
| 前端 | 原生 CSS/JS、html/template、go:embed |
| 配置 | Viper（YAML + 环境变量） |
| 日志 | slog（标准库） |

## 目录结构

```
agent_flow/
├── main.go                     # 入口：配置→数据库→迁移→种子数据→路由→启动
├── config/
│   └── config.yaml             # 配置文件（server/database/auth/agent/llm）
├── context/
│   ├── agents/                 # 内置智能体定义（agent.yaml + 提示词文件）
│   ├── skills/                 # 内置技能（Markdown 文件）
│   └── prompt/                 # 系统提示词模板
└── src/
    ├── common/                 # 公共工具（响应/错误/加密/校验）
    ├── config/                 # 配置结构体 + 加载器
    ├── database/               # 数据库初始化
    ├── logger/                 # 按天轮转日志处理器
    ├── model/                  # GORM 模型（17 个）
    ├── service/                # 业务逻辑（25 个服务）
    ├── controller/             # HTTP 处理器（21 个控制器）
    ├── tool/                   # 工具系统（28 个内置 + MCP 适配器）
    │   ├── files/              # 文件操作实现
    │   ├── tasks/              # 任务操作辅助
    │   └── browsers/           # 浏览器自动化（chromedp）
    ├── agentctx/               # 上下文构建器（5 层提示词组装）
    ├── provider/               # LLM 客户端（OpenAI/Anthropic/Gemini）
    ├── middleware/             # 认证、CSRF、CORS、安全头、日志、异常恢复
    ├── router/                 # 路由注册
    └── view/
        ├── static/             # CSS + JS 静态资源
        └── template/           # HTML 模板（布局 + 页面）
```

## 功能说明

### 工作台

统一的 AI 交互界面。选择工作区和智能体后，即可与 AI 实时对话，AI 可调用工具自主执行任务。

- SSE 流式推送，200 事件环形缓冲支持断线重连回放
- 后台任务执行独立于 HTTP 连接
- 多订阅者支持（多浏览器标签页）
- 子智能体调用，支持复杂多步工作流

### 智能体系统

- **内置智能体** — 从 `context/agents/` 加载的预配置智能体
- **自定义智能体** — 通过界面创建，配置角色、规则、工作流、技能和记忆
- **加载控制** — 精细控制每个智能体可访问的文件和工具
- **智能体记忆** — 跨工作区持久记忆，基于差异模式更新
- **子智能体调用** — 智能体可将工作委派给其他智能体

### 工具系统

28 个内置工具按类别组织：

| 类别 | 工具 |
|------|------|
| 文件操作 | `read_files`、`write_files`、`delete_files`、`list_files`、`search_files`、`analysis_files`、`diagnose_files` |
| 命令执行 | `run_command` |
| 浏览器 | `browser_action`（chromedp 驱动 headless 浏览器） |
| 记忆 | `update_memories` |
| 网络搜索 | `web_searches` |
| 工作区文档 | `write_work_docs`、`read_work_docs`、`delete_work_docs`、`list_work_docs`、`analysis_work_docs` |
| 上下文 | `read_contexts`、`write_contexts`、`delete_contexts`、`context_lists`、`search_contexts`、`analysis_contexts` |
| 任务 | `task_lists`、`task_writers`、`task_deletes` |
| 智能体 | `write_agent`、`list_agents`、`call_agent` |

**MCP 工具** — 通过 Model Context Protocol（stdio JSON-RPC）扩展外部工具能力。

### 任务管理

- 分阶段任务拆解，支持依赖关系追踪
- 状态流转：待处理 → 进行中 → 完成/失败/跳过
- 任务文档关联工作区 `tasks/` 目录
- 自动检测阶段完成进度

### 上下文系统

5 层分级上下文组装系统提示词：

1. **System** — 全局规则（`context/system.md`）
2. **Project** — 项目级上下文（AGENTS.md / CONTEXT.md / README.md）
3. **Specs** — 工作区规格文档（`workspace/{uuid}/specs/`）
4. **Role** — 智能体专属提示词（role/rule/workflow/skill/memory）
5. **Task Summary** — 当前阶段任务概览

## 快速开始

### 环境要求

- Go 1.25 或更高版本

### 源码运行

```bash
git clone https://github.com/gacjie/agent_flow.git
cd agent_flow
go run main.go
```

浏览器打开 http://localhost:8080，默认账号：`admin` / `admin123`

### 编译二进制

```bash
go build -o agent_flow .
./agent_flow
```

### 交叉编译

```bash
# Linux / macOS / Git Bash
bash build.sh

# Windows CMD
build.bat
```

编译产物输出到 `build/` 目录：

| 文件 | 平台 | 架构 |
|------|------|------|
| `agent_flow_windows_amd64.exe` | Windows | x86_64 |
| `agent_flow_linux_amd64` | Linux | x86_64 |
| `agent_flow_linux_arm64` | Linux | ARM64 |
| `agent_flow_linux_armv7` | Linux | ARMv7 |

## 配置说明

配置文件：`config/config.yaml`

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  mode: "release"          # debug 或 release

database:
  driver: "sqlite"
  dsn: "data.db"

auth:
  session_ttl: "24h"
  admin_username: "admin"
  admin_password: "admin123"

agent:
  context_root: "context"
  max_iterations: 50

llm:
  api_timeout: 120
  max_retries: 2
```

### 环境变量覆盖

所有配置项均可通过 `AF_` 前缀的环境变量覆盖：

```bash
AF_SERVER_PORT=9090 AF_AUTH_ADMIN_PASSWORD=secure123 ./agent_flow
```

## 部署

### 最简部署

1. 将编译好的二进制文件复制到服务器
2. 将 `config/config.yaml` 放到同一目录
3. 运行二进制文件

```bash
./agent_flow_linux_amd64
```

数据库文件（`data.db`）和工作区目录会在首次运行时自动创建。静态资源和模板已嵌入二进制文件。

### Systemd 服务（Linux）

```ini
[Unit]
Description=AgentFlow
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/agent_flow
ExecStart=/opt/agent_flow/agent_flow_linux_amd64
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### LLM 模型配置

首次登录后，进入 **系统管理 → LLM 模型** 添加你的 LLM 提供商：

- **OpenAI 兼容** — 任何遵循 OpenAI Chat Completions 格式的 API
- **Anthropic** — Claude 模型，支持 Prompt Caching
- **Gemini** — Google Gemini 模型
- **OpenAI Responses** — OpenAI Responses API 格式

## 截图

> 截图即将添加。

<!-- ![工作台](screenshots/workbench.png) -->
<!-- ![仪表盘](screenshots/dashboard.png) -->

## 许可证

本项目基于 [Apache License 2.0](LICENSE) 开源。
