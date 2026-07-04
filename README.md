# AgentFlow

[中文文档](README_zh_cn.md)

AgentFlow is an out-of-the-box AI agent workflow system for running multi-agent workspaces, streaming conversations, project-aware tools, task tracking, and model/MCP management from a web console or mobile client.

## Functional Overview

### Login And Account

- Login with administrator credentials and captcha verification.
- The default administrator can be created from the configured `admin_user` and `admin_password`; first login can require password change.
- Manage personal profile information and password from the account menu.
- Use role-based permissions to control access to the dashboard, workbench, models, agents, skills, MCP tools, projects, members, roles, prompts, system tools, and system settings.

### Model Management

- Add and manage models using these protocols:
  - OpenAI-compatible chat completions
  - OpenAI Responses
  - Anthropic Claude
  - Google Gemini
- Configure display name, local model ID, upstream model ID, base URL, API key, token limits, and reasoning effort.
- Mark model capabilities with `tools`, `vision`, and `image_gen`.
- Enable models for automatic fallback/switching.
- Test model connectivity from the model list.
- Fetch upstream model lists when the upstream provider supports listing models.
- Import and export model configurations as JSON.

### Workbench

- Select, create, and delete workspaces from the workbench.
- Create, open, delete, and stop conversations.
- Stream assistant responses with SSE.
- Reconnect to a running conversation and replay available streamed events.
- Switch the active agent for a conversation.
- View assistant text, reasoning content, tool calls, and tool results.
- Use main conversations and sub-conversations created by agent delegation.
- Enhance a drafted prompt before sending it.
- Stop a running task without deleting the conversation history.

### Attachments And Files

- Upload supported attachments including images, text files, Markdown files, and PDFs.
- Preview uploaded image attachments and remove pending attachments before sending.
- Browse uploaded files and browser screenshots for the current workspace.
- Read and save editable uploaded text content.
- Browse project files for workspaces linked to a project path.
- Read existing project files and save edits to existing files.
- Read and save workspace documents under `specs/` and `tasks/`.
- Read and save project documents such as `AGENTS.md`, `README.md`, and Markdown files under `docs/`.

### Projects And Workspaces

- Create projects with a name, description, and bound project path.
- Create workspaces under projects or directly from the workbench.
- Bind a default agent to a workspace.
- Keep workspace data isolated by workspace UUID.
- View workspace task lists and task progress.
- Delete a workspace and its workspace files when no conversation is running.

### Agents And Skills

- Use built-in agents loaded from the configured context directory.
- Create and edit custom agents from the UI.
- Configure each agent with:
  - name, title, description, keywords, icon, and status
  - model selection or automatic model mode
  - role, rule, workflow, skill, and memory files
  - auto-loaded context file types
  - allowed tools
  - linked skills
- Manage reusable skills with label, description, keywords, level, content, status, and sort order.
- Agent memory is stored per agent and can be updated by agent tools or context tidy-up.

### Tool Capabilities

AgentFlow exposes built-in tools by capability category instead of a fixed tool count:

| Category | Capabilities |
| --- | --- |
| File operations | read, write, delete, list, search, analyze, and diagnose project files |
| Commands | run shell commands in the project work directory with timeout and dangerous-command checks |
| Browser automation | navigate, click, type, scroll, evaluate JavaScript, clear cache, and take screenshots |
| Memory | append, modify, or delete agent memory entries |
| Workspace docs | write, read, delete, list, and analyze workspace documents |
| Context files | read, write, delete, list, search, and analyze context files |
| Tasks | list, create/update/subdivide, and delete workspace tasks |
| Agents | create/update agents, list agents, and call another agent as a sub-agent |
| Web search | search with configurable engine fallback |
| Image generation | generate one or more images using the configured image generation model |
| MCP | use dynamically registered tools exposed by enabled MCP servers |

### MCP Management

- Import MCP servers from standard `mcpServers` JSON.
- Export selected MCP server configurations as `mcpServers` JSON.
- Configure command, arguments, environment variables, timeout, label, category, and version.
- Start and stop MCP servers from the UI.
- View server status, discovered tool count, and tool names.
- Enabled MCP servers are started on application startup and their tools are registered automatically.

### System Tools And Prompts

- View built-in system tools synchronized from the tool registry.
- Enable or disable built-in tools from the system tools page.
- View and edit system prompt templates.
- Reset prompt templates to their default content when available.

### System Settings

- Configure site settings and security-related settings from the system configuration page.
- Important configuration keys include:
  - `session_max_age` for browser session lifetime
  - `admin_user` and `admin_password` for the initial administrator account
  - `llm.timeout`, `llm.max_retries`, and `llm.stream_idle_timeout` for model calls
  - `ai.vision_model`, `ai.image_gen_model`, and `ai.prompt_enhance_model` for AI service routing
  - `search.engine`, `search.searxng_url`, and `search.jina_api_key` for web search
  - pagination defaults for list pages

### Mobile Client Support

- Login through the API with a bearer token.
- Store a separate token per server.
- Manage multiple AgentFlow servers and switch between them.
- Browse workspaces, conversations, and tasks.
- Use SSE streaming conversations with reconnection support.
- Display text, reasoning content, tool calls, tool results, attachments, and code blocks.
