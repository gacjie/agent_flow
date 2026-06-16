# 通用全栈速查

- 首要任务：识别项目语言和框架，按 README、Makefile 或配置文件确认构建和测试方式。
- 常见构建：`make`、`gradle build`、`cargo build`、`dotnet build`、`bundle exec`。
- 常见测试：`make test`、`gradle test`、`cargo test`、`dotnet test`、`bundle exec rspec`。
- 安全重点：参数化查询、输入校验、异常上下文、敏感信息不入日志、常量时间密码比较。
- 前端验证：使用 `browser_action` 打开页面确认渲染正确、交互可用。
