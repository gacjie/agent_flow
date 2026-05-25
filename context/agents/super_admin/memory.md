# 超级管理智能体 — 专属记忆

## 系统架构知识

**AgentFlow 关键目录路径**：

| 目录 | 用途 |
|------|------|
| `context/agents/{name}/` | 智能体上下文文件（role/rule/skill/workflow/memory） |
| `context/skills/` | 可复用技能库（*.md 文件） |
| `working/{uuid}/` | 工作区目录（每个工作区独立，存放工作记录文档） |
| `config/config.yaml` | 系统配置文件 |

**智能体文件类型说明**：

| 文件 | 作用 | 加载控制 |
|------|------|---------|
| `agent.yaml` | 智能体配置（名称/模型/工具权限） | 启动时自动同步到数据库 |
| `role.md` | 角色定义（身份/职责/方法/规范） | `auto_load_files: ["role"]` |
| `rule.md` | 强制规则与约束 | `auto_load_files: ["rule"]` |
| `skill.md` | 技能列表（预加载常用技能） | `auto_load_files: ["skill"]` |
| `workflow.md` | 工作流程步骤 | `auto_load_files: ["workflow"]` |
| `memory.md` | 智能体专属持久记忆（跨工作区） | `auto_load_files: ["memory"]` |

## 工具使用速查

**write_work_docs vs write_files**：
- `write_work_docs` → 工作区文档（需求/设计/任务/报告），写入 `working/{uuid}/`
- `write_files` → 项目代码文件（.go/.py/.js/config），写入项目目录

**update_memories**：
- 写入当前智能体的 `context/agents/{name}/memory.md`
- 跨工作区持久生效（与工作区无关）
- 使用 old_memory/new_memory 差异模式：只传 new_memory=追加，两者都传=修改，只传 old_memory=删除
- 支持 updates 数组批量操作多条记忆

## 常见操作模式

（本节由智能体在运行过程中自动更新）
