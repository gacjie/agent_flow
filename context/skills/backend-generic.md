---
name: backend-generic
label: 通用后端开发技能
description: 未覆盖语言或无法判断技术栈时的后端开发识别和验证策略
keywords: Backend,Java,Rust,Ruby,C#,Kotlin,Swift,通用后端,验证
level: 1
status: 1
sort: 24
---

# 通用后端开发技能

## 技术栈识别

| 线索 | 判断 |
|------|------|
| `pom.xml`、`build.gradle`、`build.gradle.kts` | Java/Kotlin |
| `Cargo.toml` | Rust |
| `Gemfile` | Ruby |
| `.sln`、`.csproj`、`Directory.Build.props` | .NET/C# |
| `Package.swift` | Swift |

## 实施要点

- 先找项目 README、构建脚本、CI 配置和同类实现，不凭语言常识强推结构。
- 按项目已有层级实现：公共方法 → 数据访问 → 服务逻辑 → 接口/命令入口。
- 输入校验、权限、事务、错误响应和日志必须跟随项目已有模式。
- 无法识别框架时，只修改目标文件附近可确定的最小范围。

## 验证策略

| 层次 | 做法 |
|------|------|
| 语法/编译 | 使用项目构建脚本、语言标准编译器或 CI 中命令 |
| 启动 | 使用 README、Makefile、脚本或主入口 |
| 功能 | 请求端点、执行命令或运行最小用例 |
| 测试 | 使用项目已有测试脚本，只修复本次变更引入失败 |

无法完整验证时输出已验证层次、无法验证原因和建议后续。
