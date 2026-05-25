---
name: agent-creator
label: 智能体设计专家
description: 专业智能体创建指南，涵盖六文件完整设计方法论、auto_load_files/tools选型、提示词工程最佳实践、质量红线与反模式
keywords: 智能体创建,提示词工程,Agent设计,role.md,agent.yaml,auto_load_files,auto_load_tools,prompt engineering
level: 1
status: 1
---

# 智能体设计专家技能

## 技能动态加载

使用 `load_skill` 工具按需加载技能：

```
load_skill(list=true)              # 查看所有可用技能
load_skill(name="agent-creator")   # 加载本技能完整内容
load_skill(name="python-venv")     # 加载 Python 虚拟环境规范
```

> 全局技能预加载通过管理后台 `/admin/agents` → 编辑智能体 → 关联技能 配置，对话开始时自动注入 system prompt。`load_skill` 工具用于在对话中动态补充未预加载的技能。

## 标准 agent.yaml 模板

```yaml
name: {english_name}          # [a-z_]+，如 code_reviewer / python_backend
title: {中文职位名}             # 2-15字，如 "代码审查工程师"
description: {功能描述}         # 80-150字：擅长什么 + 适用场景
keywords: {关键词}              # 逗号分隔，中英双语含同义词
model_id: auto
model_mode: auto
icon: ""
sort: 10
status: 1
auto_load_files:
  - role
  # - rule        # 有强制操作约束时加
  # - skill       # 有大量技术知识预加载时加
  # - workflow    # 有固定多步骤流程时加
  # - memory      # 需要跨会话持久知识时加
auto_load_tools:
  # 文件操作：read_files / write_files / edit_files / list_files / search_files
  # 命令执行：run_command
  # 搜索：web_searches
  # 工作文档：write_work_docs / list_work_docs / read_work_docs
  # 任务管理：task_lists / task_writers / task_deletes
  # 记忆：update_memories
  # 智能体：write_agent / list_agents / call_agent
```

## auto_load_files 选型原则

| 文件类型 | 何时加入 |
|---------|---------|
| `role` | **始终加入**，所有角色必须有 |
| `rule` | 有强制行为约束或安全限制时（代码开发/审查/数据操作类） |
| `skill` | 需要预加载大量技术知识/命令速查/API参考时 |
| `workflow` | 有固定多步骤执行流程、需要检查清单的场景 |
| `memory` | 需要跨会话持久化知识时（偏好记录/项目约定/常见模式） |

**轻量对话角色**（问答/翻译/写作）只需 `["role"]`。

## auto_load_tools 分类参考

| 角色类型 | 推荐工具组合 |
|---------|------------|
| 纯对话类 | 空（无工具） |
| 代码生成类 | read_files, write_files, edit_files, list_files, search_files, run_command, task_lists, task_writers |
| 代码审查类 | read_files, list_files, search_files, run_command, write_work_docs, task_lists, task_writers |
| 任务规划类 | write_work_docs, list_work_docs, read_work_docs, task_lists, task_writers, task_deletes |
| 需求分析类 | read_files, search_files, write_work_docs, web_searches, task_lists |
| 编排类 | call_agent, list_work_docs, read_work_docs, task_lists, task_writers, task_deletes |
| 系统管理类 | all |

> **禁止**默认选 `["all"]`，除非该角色确实需要全部权限。

## 六文件内容框架

### role.md（100-200行，最重要）

```markdown
# {角色名称}

{一段话定义：你是谁 + 专业背景 + 核心价值}

## 专业身份
{领域背景与核心能力简述}

## 核心能力
### 1. {能力分类}
{具体技能/方法论，不是抽象描述}

## 工作方法/思维方式
{处理任务的具体步骤，含判断标准}

## 工具使用指南（有工具时）
{每个工具的使用场景}

## 输出规范
{格式要求/语言风格/内容结构}

## 约束边界
{明确不做什么，安全限制，拒绝条件}
```

### rule.md（60-80行）

```markdown
## 强制要求（5-8条，正面描述"必须..."）
1. 具体场景 + 具体动作
2. ...

## 质量红线
{最低可接受标准，用于判断是否完成}

## 禁止事项（3-5条，以"禁止..."开头）
- 禁止 ...
- 禁止 ...

## 异常处理
{文档不足/需求模糊/发生冲突时的处理规则}

## 产出契约
{任务完成后必须输出什么，格式如何}
```

### workflow.md（50-80行）

```markdown
## 流程 N：{流程名称}

适用场景：{一句话描述}

步骤 1 → {工具/动作}
         {执行说明，预期结果}

步骤 2 → {决策点}
         如果 A → 走 X 路径
         如果 B → 走 Y 路径

检查清单：
- [ ] {必须验证的条件}
- [ ] {完成标志}
```

### skill.md（30-60行）

内容要求：具体技术知识、代码示例、参数速查。**不能重复 role.md 内容**。

```markdown
# {角色} — 技能速查

## 常用命令/API
{代码示例}

## 参数速查表
{表格}

## 全局技能索引
{关联的 context/skills/ 文件说明}
```

### memory.md（20-50行初始）

预配置该角色领域的实用知识，不是空文件或泛泛介绍：
- 速查表/配置模板/常见模式
- 末尾留 `## 常见创建模式` 供智能体运行时自动更新

## 提示词工程准则

### 好的 role.md 特征

1. **身份明确**：第一句话清楚说明"你是什么 + 擅长什么"
2. **职责具体**：动词开头（"分析代码逻辑"），不用抽象描述（"帮助用户"）
3. **方法可操作**：提供具体步骤而非抽象原则
4. **有边界感**：明确说明什么不做、哪些场景拒绝
5. **格式统一**：规定输出格式，让回复保持一致性

### 常见反模式（避免）

| 反模式 | 问题 |
|--------|------|
| "你是一个很厉害的助手" | 无专业定位，行为不可预测 |
| 职责列了 20 项 | 焦点分散，每项都做不深 |
| 没有禁止事项 | 可能产生不当行为 |
| 没有输出规范 | 回复质量不稳定 |
| auto_load_files 全选 | 浪费上下文，影响响应速度 |
| `name` 用 `helper`/`ai`/`bot` | 语义不清，难以维护 |

## 创建前五要素确认

在设计智能体之前，必须明确以下五要素：

1. **角色用途**：解决什么问题，处理什么类型的任务？
2. **专业领域**：需要哪些领域知识？
3. **工具需求**：需要读写文件/执行命令/联网，还是仅对话？
4. **交互风格**：严谨正式还是轻松友好？简洁还是详尽？
5. **自主程度**：全自主执行还是每步需要确认？

## 创建操作规则

- `name` 格式：只允许 `[a-z_]+`，推荐 `领域_职能`，如 `python_backend`
- 禁止通用名：`helper`/`assistant`/`bot`/`agent`/`ai`
- **创建前必须** `list_agents` 确认 name 不重名
- 文件写入顺序：`role.md → rule.md → skill.md → workflow.md → memory.md → agent.yaml`（agent.yaml 最后写入）
- `write_agent` 必须传 `role_prompt`（= role.md 完整内容），按 name 自动判断创建或更新
