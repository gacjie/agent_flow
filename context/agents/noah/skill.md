# 任务规划速查

## 标签

- `[Backend]`：项目只有一个后端技术栈或编排师可唯一判断。
- `[Backend:Go]` / `[Backend:Python]`：多后端或需明确路由。
- `[Frontend]`：页面、样式、浏览器交互。
- `[Test:Unit]`、`[Test:API]`、`[Test:E2E]`、`[Test:Integration]`：按实际测试对象选择。
- `[Doc]`：项目文档、用户说明、API 文档。

## 优先级

- `P0`：数据模型、基础配置、公共能力、被其他任务依赖的接口。
- `P1`：核心业务流程和主要页面。
- `P2`：测试、文档、增强、边界体验。

## task_doc 关联

- 模型、服务：优先关联 `tasks/design-{module}.md`。
- 页面、模板：优先关联 `tasks/ui-patterns-{module}.md`（若存在），否则关联 `tasks/design-{module}.md`。
- API 前后端任务和接口测试：优先关联 `tasks/api-contract-{module}.md`。
- 修复任务：关联对应测试报告或审查报告。

## 自检句式

结束前确认：任务已写入、父子关系正确、标签完整、无跨栈叶子任务、关键功能有覆盖、测试任务已规划。
