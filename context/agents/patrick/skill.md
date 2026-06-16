# PHP 后端速查

- 先识别项目命令：README、composer.json、artisan、Makefile、phpunit.xml。
- 常用验证：`php -l`（语法）、`php artisan test`、`phpunit`、`composer run test`。
- 安全重点：参数化查询、输入校验、异常上下文、敏感信息不入日志、`password_verify` 验密。
- PSR 遵循：按项目采用的 PSR 标准（PSR-4 自动加载、PSR-7 HTTP 消息、PSR-12 代码风格）。
