# 编排速查

## 阶段产物

| 阶段 | 子阶段 | 责任人 | 关键产物 |
|---|---|---|---|
| 1. 需求分析 | — | olivia | `specs/requirement.md` |
| 2. 设计与规划 | 2-A 架构规范 | archer | `specs/tech-spec.md`、`AGENTS.md` |
| 2. 设计与规划 | 2-B 系统设计 | ethan（分步） | `specs/design.md`、`tasks/design-*` → `tasks/api-contract-*` |
| 2. 设计与规划 | 2-B2 前端风格 | sophia（有前端时） | `specs/ui-patterns.md`、`tasks/ui-patterns-*` |
| 2. 设计与规划 | 2-C 任务规划 | noah | 任务树 |
| 3. 开发实施 | — | gavin/lucas/patrick/nathan/derek/emma | 代码变更 + 任务状态（一个 phase 一次 call_agent） |
| 4. 验证与审查 | 4-A 测试验证 | liam | `tasks/test-report-*` + `tasks/test-report-e2e-*`（有前端时） |
| 4. 验证与审查 | 4-B 代码审查 | alex | `tasks/review-round-*` |
| 5. 完成汇报 | — | owen（条件调用 mia） | 直接回复用户 |

## 路由规则

- `[Backend:Go]` -> gavin
- `[Backend:Python]` -> lucas
- `[Backend:PHP]` -> patrick
- `[Backend:Node]` -> nathan
- `[Backend]` -> derek（通用全栈，处理无专用智能体的技术栈）
- `[Frontend]` -> emma（优先）；derek 可作为前端 fallback
- `[Test:*]` -> liam
- `[Doc]` -> mia

## 动态裁剪要点

- 无前端：跳过 sophia（UI patterns）、跳过 liam E2E 验证
- 已有项目：archer 更新而非重写 AGENTS.md
- 小型单模块：ethan design + api-contract 可合为一文件；noah 可能只 1-2 个 phase
- 纯 CLI/脚本：开发可能只有一个 phase 一次 call_agent
- 文档更新：新增模块/改 API/改部署 → 调用 mia；纯重构/修复 → 不调用
