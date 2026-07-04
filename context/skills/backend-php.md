---
name: backend-php
label: PHP 后端开发技能
description: PHP 后端项目识别、实现和验证速查
keywords: PHP,Laravel,Symfony,ThinkPHP,WordPress,composer,phpunit,pest
level: 1
status: 1
sort: 22
---

# PHP 后端开发技能

## 技术栈识别

| 线索 | 判断 |
|------|------|
| `composer.json` | PHP 依赖管理 |
| `artisan`、`app/Http` | Laravel |
| `bin/console`、`src/Controller` | Symfony |
| `think`、`app/controller` | ThinkPHP |
| `wp-config.php`、`wp-content` | WordPress |

## 实施要点

- 先搜同类 Controller/Route/Service/Model/FormRequest 或模板实现。
- 遵循项目 ORM、Query Builder、PDO 封装和异常响应模式。
- update/edit 接口必须对照 add/create 校验必填、枚举、默认值和业务规则。
- 权限变更同时检查中间件、策略、FormRequest、路由权限和数据模型。

## 验证命令

| 层次 | 优先命令 |
|------|---------|
| 语法 | `php -l <file>` |
| 框架加载 | Composer scripts、`php artisan`、`bin/console` 或入口脚本 |
| 接口 | 项目路由测试、HTTP 请求或框架测试工具 |
| 测试 | Composer scripts、`vendor/bin/phpunit`、`vendor/bin/pest` |

无法执行时记录缺失扩展、vendor 目录或环境配置。
