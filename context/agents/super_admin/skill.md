# 超级管理智能体 — 核心技能

以下为预加载的常用技能。使用 `context_lists()` 查看所有可用技能，`read_contexts(files=[{"path": "skills/xxx.md"}])` 动态加载指定技能的完整内容。

## 系统状态检查

常用只读命令组合：

```bash
df -h                  # 磁盘使用情况
free -m                # 内存使用情况
uptime                 # 系统运行时间和负载
ps aux | head -20      # 进程列表（前20条）
netstat -tlnp          # 监听端口列表
ls -la /path/to/dir    # 目录详情（含隐藏文件）
find /path -name "*.log" -mtime -1  # 查找最近1天修改的日志
```

## 智能体生命周期管理

**创建智能体参数清单**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `name` | 必填 | `[a-z_]+`，领域_职能格式，如 `code_reviewer` |
| `title` | 必填 | 中文显示名称，如 `代码审查工程师` |
| `role_prompt` | 必填 | role.md 完整内容（Markdown） |
| `description` | 可选 | 80-150字功能描述 |
| `keywords` | 可选 | 逗号分隔，中英双语 |
| `model_id` | 可选 | 留空使用系统默认模型 |

**auto_load_tools 常用组合速查**：

| 角色类型 | 工具 |
|---------|------|
| 代码开发类 | read_files, write_files, list_files, search_files, analysis_files, run_command, web_searches, write_work_docs, delete_work_docs, list_work_docs, read_work_docs, task_lists, task_writers |
| 代码审查类 | read_files, list_files, search_files, analysis_files, run_command, web_searches, write_work_docs, delete_work_docs, list_work_docs, read_work_docs, task_lists, task_writers, browser_action |
| 任务规划类 | read_files, write_files, list_files, write_work_docs, delete_work_docs, list_work_docs, read_work_docs, analysis_work_docs, task_lists, task_writers, task_deletes |
| 编排类 | call_agent, read_files, write_files, list_files, list_work_docs, read_work_docs, analysis_work_docs, write_work_docs, delete_work_docs, task_lists, task_writers |
| 系统管理类 | all |

**禁止通用 name**：`helper`/`assistant`/`bot`/`agent`/`ai`

**更新智能体**：只传需要修改的字段，例如只更新 title：
```json
{"id": 5, "title": "新标题"}
```

## 智能体文件写入路径规范

| 文件 | 路径 | 说明 |
|------|------|------|
| role.md | `context/agents/{name}/role.md` | 最重要，100-200行 |
| rule.md | `context/agents/{name}/rule.md` | 60-80行，含禁止事项 |
| skill.md | `context/agents/{name}/skill.md` | 技术知识速查 |
| workflow.md | `context/agents/{name}/workflow.md` | 每流程末有检查清单 |
| memory.md | `context/agents/{name}/memory.md` | 跨会话持久知识 |
| agent.yaml | `context/agents/{name}/agent.yaml` | **最后写入** |

> 全局共享技能放在 `context/skills/` 目录，由管理后台关联到智能体。

## 文件搜索与定位

三步定位工作流：
1. `list_files(dirs=[{"path": ".", "recursive": true}])` — 获取目录全貌
2. `search_files(queries=[{"pattern": "关键词", "path": "src/"}])` — 确定目标文件
3. `read_files(files=[{"path": "src/xxx.go", "offset": 1, "limit": 50}])` — 查看具体内容

**搜索技巧**：
- `file_pattern: "*.go"` — 限定文件类型
- `pattern: "func.*Handler"` — 支持简单正则
- `path: "src/service"` — 限定搜索目录缩小范围

## 记忆管理

`update_memories` 使用 old_memory/new_memory 差异模式，支持单条和批量操作：

| 操作 | 参数 | 说明 |
|------|------|------|
| 追加 | 只传 `new_memory` | 在记忆文件末尾追加新内容 |
| 修改 | `old_memory` + `new_memory` | 精确匹配旧内容并替换为新内容 |
| 删除 | 只传 `old_memory` | 精确匹配并删除该段内容 |
| 批量 | `updates` 数组 | 数组中每项含 old_memory/new_memory，一次执行多条操作 |

## 工作区文档规范

`write_work_docs` 使用 `path` 参数指定文档路径（相对于工作区目录）：

| path | 用途 | 加载方式 |
|------|------|----------|
| `specs/requirement.md` | 需求说明文档 | 自动加载到上下文 |
| `specs/design.md` | 系统设计文档 | 自动加载到上下文 |
| `specs/tech-spec.md` | 技术规范文档 | 自动加载到上下文 |
| `tasks/backend-api.md` | 任务文档 | 由任务节点 task_doc 字段引用，按需加载 |

