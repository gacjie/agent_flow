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
- 始终使用 `===` 严格比较，禁止 `==` 松散比较（Type Juggling 漏洞核心源头）。
- 数据库操作使用 PDO 预处理语句或 ORM 参数化查询，禁止字符串拼接 SQL。
- 用户输入涉及文件操作时需 `realpath()` + `strpos()` 验证路径在允许目录内。
- 禁止 `eval()`/`system()`/`exec()`/`passthru()`/`shell_exec()` 直接处理用户输入。
- Model 必须显式声明 `$fillable`（白名单）或 `$guarded`（黑名单），防 Mass Assignment。
- 生产环境必须启用 OPcache 并执行 `config:cache` / `route:cache` 预编译。

## 验证命令

| 层次 | 优先命令 |
|------|---------|
| 语法 | `php -l <file>` |
| 路由检查 | `php artisan route:list`（Laravel） |
| 依赖漏洞 | `composer audit` |
| 静态分析 | `vendor/bin/phpstan analyse`（PHPStan） |
| 框架加载 | Composer scripts、`php artisan`、`bin/console` |
| 接口 | 项目路由测试、HTTP 请求或框架测试工具 |
| 测试 | `vendor/bin/phpunit`、`vendor/bin/pest` |

无法执行时记录缺失扩展、vendor 目录或环境配置。

## 框架与项目模式

### Laravel 关键配置

中间件栈顺序：
```
EncryptCookies → AddQueuedCookiesToResponse → StartSession
→ VerifyCsrfToken → SubstituteBindings → Auth → Throttle → CSP
```

授权系统：
- Gate 定义全局策略：`Gate::define('update-post', function($user, $post) {...})`
- Policy 关联模型：`php artisan make:policy PostPolicy --model=Post`
- Controller 中使用：`$this->authorize('update', $post)`

Eloquent 安全使用：
- `$fillable` 白名单限制可批量赋值字段（防 Mass Assignment）
- `with(['relation'])` 预加载关联数据（防 N+1 查询）
- `whereHas()` 安全的关联条件过滤
- `$casts` 类型转换确保数据类型正确

队列系统：
- 耗时任务推入 Queue 异步处理（Redis/SQS/Database 驱动）
- `ThrottleRequests` 中间件默认限流 60 次/分钟
- 失败任务自动重试 + 死信队列

### Symfony 关键配置

- Security 组件：Authenticator 认证 + Voter 细粒度授权投票
- Doctrine ORM：Repository 模式 + DQL 参数化查询 + 迁移管理
- 表单组件：自动 CSRF token 注入 + 类型安全验证
- Messenger 组件：异步消息处理（类似 Laravel Queue）

### ThinkPHP 关键配置

- 中间件机制 + 验证器（Validate 类）独立校验请求参数
- 模型搜索器（`searchXxxAttr`）实现安全的动态条件查询
- 路由中间件分组绑定 + API 版本管理

### 生产部署必须项

```ini
# php.ini 生产关键配置
display_errors = Off
expose_php = Off
opcache.enable = 1
opcache.validate_timestamps = 0
opcache.memory_consumption = 256
opcache.max_accelerated_files = 65536
realpath_cache_size = 4096k
session.cookie_httponly = 1
session.cookie_secure = 1
session.use_strict_mode = 1
```

部署清单：
- PHP-FPM + Nginx 反向代理（禁止 `php -S` 内置服务器用于生产）
- `composer install --no-dev --classmap-authoritative`（去除开发依赖 + 静态映射）
- Laravel 缓存：`config:cache` + `route:cache` + `event:cache` + `view:cache`
- PHP 版本：至少 8.3+（8.1 已 EOL、8.2 将 2026.12 EOL），推荐 8.5
- 错误日志写文件而非显示，`log_errors = On` + `error_log = /path/to/php-error.log`

### WordPress 额外安全要点

- 禁用文件编辑：`define('DISALLOW_FILE_EDIT', true)`
- 限制登录尝试：Limit Login Attempts 插件或自定义
- 及时更新核心/插件/主题版本
- 数据库前缀修改（非默认 `wp_`）

## 安全风险

PHP 语言和框架特有的安全漏洞：

| 风险 | 说明 | 防护 |
|------|------|------|
| SQL 注入 | 用户输入直接拼接到 SQL 字符串 `"WHERE id = $id"` | PDO 预处理 `$stmt->execute([$id])` / ORM 参数化查询 |
| 类型混淆 | `==` 松散比较：`"0e123" == "0e456"` 为 true，magic hash 绕过密码认证 | 始终 `===` 严格比较 + `hash_equals()` 时间安全比较哈希 |
| 危险函数 RCE | eval/system/exec/shell_exec/passthru/popen/反引号可执行系统命令 | 彻底消除 eval；必须 exec 时强制 `escapeshellarg()` + 命令白名单 |
| 文件包含 LFI/RFI | include/require 接受用户可控路径→读任意本地文件或加载远程代码 | 白名单限制可包含文件路径，禁止动态 include 用户输入 |
| 反序列化对象注入 | `unserialize()` 触发 __wakeup/__destruct 魔术方法→任意代码执行链 | 使用 `json_decode()` 替代，禁止对用户输入调用 unserialize |
| extract() 变量覆盖 | `extract($_POST)` 将用户数组导入符号表→覆盖 `$isAdmin` 等关键变量 | 禁止 `extract()` 处理任何外部输入，使用明确赋值 |
| assert() 代码执行 | `assert("$user_string")` 将字符串作为 PHP 代码执行（PHP 7 以下） | 禁用 assert 或确保不接受字符串参数，升级到 PHP 8+ |
| Session 不安全 | PHP 默认接受客户端提交的任意 session ID→Session Fixation 攻击 | 登录后 `session_regenerate_id(true)` + HttpOnly + Secure + SameSite |
| CVE-2024-4577 | Windows+Apache+PHP-CGI 参数注入（CVSS 9.8），48h 内被武器化利用 | 升级 PHP 版本、彻底避免 CGI 模式部署，改用 PHP-FPM |
| Mass Assignment | 框架未限制可批量赋值字段→攻击者提交 `is_admin=1` 提权 | 强制声明 `$fillable` 白名单（Laravel）/ 表单白名单验证 |
| CSRF | 表单/AJAX 缺少 token 验证→伪造用户请求执行敏感操作 | Laravel `VerifyCsrfToken` 中间件 / 手动双提交 Cookie token |
| XSS | 未转义用户输入直接输出到 HTML→注入恶意脚本 | `htmlspecialchars(ENT_QUOTES, 'UTF-8')` + Blade `{{ }}` 自动转义 |
| 过时 PHP 版本 | PHP 8.1 (2025.12 EOL) / 8.2 (2026.12 EOL) 不再接收安全补丁 | 至少升级到 PHP 8.3+，推荐 8.5（含最新安全修复） |
| 目录穿越 | 文件操作接受 `../../etc/passwd` 路径→读写服务器任意文件 | `realpath()` 解析符号链接后验证路径前缀在白名单目录内 |
| CVE-2026-14355 | openssl 扩展 AES-WRAP-PAD 模式堆内存损坏→可能代码执行 | 升级 PHP 8.5+（含 openssl 扩展修复） |

## 性能陷阱

PHP 语言和框架特有的性能问题：

| 问题 | 表现 | 优化 |
|------|------|------|
| 无 OPcache | 每次 HTTP 请求重新词法分析+编译全部 PHP 脚本为 opcode | 启用 OPcache：`memory_consumption=256` / `max_accelerated_files=65536` / 生产 `validate_timestamps=0` |
| 无 JIT | PHP 8.x 数学运算/图像处理/加密等 CPU 密集场景性能未充分利用 | `opcache.jit=1255` 启用 tracing JIT（注：纯 I/O 密集型 Web 应用收益有限） |
| Composer autoload 未优化 | 开发模式 PSR-4 自动加载每次请求执行 4000+ 次文件系统 stat 调用 | `composer dump-autoload --classmap-authoritative` 生成完整静态类映射文件 |
| N+1 查询 | 循环内逐条访问关联数据→100 条记录产生 101 次数据库请求 | Eager Loading `with(['relation'])` 批量预加载 / `whereIn` 手动批量 / Redis 缓存热数据 |
| 未缓存 config/routes | 每次请求重新解析数十个 YAML/PHP 配置文件和路由定义文件 | `php artisan config:cache` + `route:cache` + `event:cache`（Laravel） |
| realpath_cache 过小 | 大型框架（Laravel/Symfony）数百个文件的路径解析 IO 开销显著 | 增大 `realpath_cache_size=4096k` + `realpath_cache_ttl=600` |
| 同步执行耗时任务 | 用户 HTTP 请求阻塞等待邮件发送/PDF 生成/文件处理完成 | 推入消息队列异步处理：Laravel Queue / Symfony Messenger / Beanstalkd |
