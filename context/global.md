## 文档目录与工具对应

| 文档类型 | 路径前缀 | 写入工具 | 读取工具 |
|---------|---------|---------|---------|
| 工作区规格文档 | `specs/` | `write_contexts` | `read_contexts` |
| 工作区任务文档 | `tasks/` | `write_contexts` | `read_contexts` |
| 上传附件 | `uploads/` | — | `read_contexts` |
| 上下文技能 | `skills/` | `write_contexts` | `read_contexts` |
| 项目文档 | `docs/` | `write_contexts` / `write_files` | `read_contexts` / `read_files` |

所有文档工具（read_contexts / write_contexts / delete_contexts / context_lists / search_contexts / analysis_contexts）通过路径前缀自动路由到正确目录。uploads/ 为只读。
