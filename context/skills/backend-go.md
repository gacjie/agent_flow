---
name: backend-go
label: Go 后端开发技能
description: Go 后端项目识别、实现和验证速查
keywords: Go,Golang,go.mod,go build,go test,Gin,Chi,GORM
level: 1
status: 1
sort: 20
---

# Go 后端开发技能

## 技术栈识别

| 线索 | 判断 |
|------|------|
| `go.mod` | Go 模块根目录 |
| `cmd/`、`main.go` | 启动入口 |
| `internal/`、`pkg/` | 内部模块和公共包 |
| `*_test.go` | Go 测试 |
| `gin`、`chi`、`echo`、`fiber` | HTTP 框架线索 |

## 实施要点

- 先搜同类 handler/service/repository/model，再按同模式扩展。
- 错误处理遵循项目已有返回值、包装和日志模式。
- 数据写入涉及多表或状态流转时使用项目事务模式。
- 新增权限相关逻辑时同时检查中间件、策略和服务层边界。

## 验证命令

| 层次 | 优先命令 |
|------|---------|
| 编译 | `go build ./...` |
| 静态检查 | `go vet ./...` |
| 启动 | README、Makefile、脚本或目标 `main` 包 |
| 测试 | `go test ./...` |

无法执行时记录具体缺失依赖、环境变量或服务。
