# 编排速查

## 阶段产物

| 阶段 | 责任人 | 关键产物 |
|---|---|---|
| 需求 | olivia | `specs/requirement.md` |
| 规范 | archer | `specs/tech-spec.md`、`AGENTS.md` |
| 设计 | ethan（分步） | `specs/design.md`、`tasks/design-*` → `tasks/api-contract-*` → `specs/ui-patterns.md`、`tasks/ui-patterns-*` |
| 任务 | noah | 任务树 |
| 开发 | gavin/lucas/emma | 代码变更 + 任务状态（一个 phase 一次 call_agent） |
| 测试 | liam | `tasks/test-report-*` + `tasks/test-report-e2e-*`（有前端时必须） |
| 文档 | mia | `docs/`、`AGENTS.md` 文档清单 |
| 审查 | alex | `tasks/review-round-*` |

## 路由规则

- `[Backend:Go]` -> gavin
- `[Backend:Python]` -> lucas
- `[Backend]` -> 按项目唯一后端技术栈选择
- `[Frontend]` -> emma
- `[Test:*]` -> liam
- `[Doc]` -> mia
