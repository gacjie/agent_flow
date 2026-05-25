---
name: code-review-checklist
label: 代码审查清单
description: 代码审查清单速查，涵盖 OWASP Top 10 快速检查、常见性能反模式、代码异味识别和安全头配置
keywords: 审查,OWASP,安全,性能,代码异味,N+1,SQL注入,XSS,安全头,检查清单
level: 1
status: 1
---

# 代码审查清单速查

## OWASP Top 10 快速检查表

### A01 注入

| 检查项 | 通过标准 | 严重程度 |
|--------|----------|----------|
| SQL 查询是否使用参数化 | `WHERE id = ?` 或 ORM 方法，非字符串拼接 | 严重 |
| 命令执行是否安全 | 使用 subprocess 列表参数，非 shell=True + 字符串 | 严重 |
| 模板是否自动转义 | 未使用 `|safe`、`template.HTML`、`{% autoescape false %}` | 严重 |
| LDAP 查询是否参数化 | 过滤器参数经过转义 | 严重 |
| 文件路径是否校验 | 用户输入的路径经过规范化和白名单检查，防止路径遍历 | 严重 |

快速搜索关键词：
```
# SQL 注入风险
f"SELECT.*{
"SELECT.*" +
.format(.*SELECT
.execute(f"

# 命令注入风险
os.system(
subprocess.*shell=True
eval(
exec(
```

### A02 认证缺陷

| 检查项 | 通过标准 | 严重程度 |
|--------|----------|----------|
| 密码存储 | bcrypt/argon2 哈希，非 MD5/SHA1/明文 | 严重 |
| 密码强度 | 至少要求 8 位 + 大小写 + 数字 | 警告 |
| Session Token | 使用 crypto/rand 或 secrets 模块生成 | 严重 |
| Session 过期 | 设置了合理的过期时间（如 24 小时） | 警告 |
| 登录限流 | 多次失败后锁定或延迟 | 警告 |
| 登出处理 | 服务端销毁 Session，非仅删除客户端 Cookie | 警告 |

### A03 敏感数据暴露

| 检查项 | 通过标准 | 严重程度 |
|--------|----------|----------|
| 密码 JSON 排除 | `json:"-"` 或序列化时排除密码字段 | 严重 |
| 日志无敏感信息 | 日志中不打印密码、Token、身份证号 | 严重 |
| 无硬编码密钥 | API Key / Secret 通过环境变量或配置文件注入 | 严重 |
| HTTPS 传输 | 生产环境强制 HTTPS | 警告 |
| 错误信息脱敏 | 生产环境错误响应不暴露堆栈和内部路径 | 警告 |

快速搜索关键词：
```
password.*=.*"
secret.*=.*"
api_key.*=.*"
token.*=.*"
log.*(password
log.*(token
```

### A07 XSS

| 检查项 | 通过标准 | 严重程度 |
|--------|----------|----------|
| HTML 输出转义 | 使用模板引擎自动转义 | 严重 |
| innerHTML 使用 | 不处理用户可控数据，或已转义 | 严重 |
| document.write 使用 | 不存在或不处理用户数据 | 严重 |
| eval 使用 | 不存在或不处理用户数据 | 严重 |
| URL 参数回显 | 经过转义或使用 textContent | 警告 |
| CSP 配置 | 设置了 Content-Security-Policy 头 | 建议 |

### A05 安全配置

| 检查项 | 通过标准 | 严重程度 |
|--------|----------|----------|
| 调试模式 | 生产配置中 debug=false | 警告 |
| 默认密码 | 无硬编码默认密码，或强制首次登录修改 | 警告 |
| CORS 配置 | 不使用 `*` 作为允许源 | 警告 |
| 目录列表 | Web 服务器禁止目录浏览 | 建议 |
| 错误页面 | 自定义错误页面，不暴露框架版本 | 建议 |

## 常见性能反模式清单

### 数据库相关

| 反模式 | 表现 | 修复方向 |
|--------|------|----------|
| N+1 查询 | 循环内逐条查询关联数据 | 预加载（Preload/joinedload） |
| SELECT * | 查询所有列但只用几列 | 指定需要的列 |
| 缺少索引 | WHERE/ORDER BY 的列无索引 | 添加索引 |
| 未分页 | 查询无 LIMIT，返回全量数据 | 添加分页参数 |
| 循环内事务 | 每次循环都 commit | 批量操作后统一 commit |
| COUNT 后 SELECT | 先 COUNT 再 SELECT 同条件 | 一次查询返回总数和数据 |

识别模式：
```
# N+1 查询（循环内查询）
for item in items:
    related = db.query(Related).filter(Related.item_id == item.id)

# Go 版本
for _, item := range items {
    db.Where("item_id = ?", item.ID).Find(&related)
}
```

### 计算相关

| 反模式 | 表现 | 修复方向 |
|--------|------|----------|
| 循环内重复计算 | 不变的值在循环内反复计算 | 提取到循环外 |
| 不必要的序列化 | 数据多次 JSON 编解码 | 减少转换次数 |
| 字符串拼接 | 循环内用 + 拼接字符串 | 使用 StringBuilder/join |
| 未缓存计算结果 | 相同输入重复计算 | 添加缓存（LRU） |

### 资源管理

| 反模式 | 表现 | 修复方向 |
|--------|------|----------|
| 未关闭连接 | 数据库/HTTP 连接未 close | 使用 defer/with/finally |
| 无限增长缓存 | Map/Dict 只增不删 | 设置大小上限 + 淘汰策略 |
| goroutine 泄漏 | 启动协程但无退出条件 | 添加 context 取消机制 |
| 大文件一次读取 | 读取整个文件到内存 | 使用流式读取（bufio） |

## 代码异味识别要点

### 函数层面

| 异味 | 判定标准 | 建议 |
|------|----------|------|
| 过长函数 | >50 行（不含注释/空行） | 提取子函数 |
| 过多参数 | >5 个参数 | 封装为参数对象/结构体 |
| 过深嵌套 | >3 层 if/for | early return / 提取函数 |
| 布尔参数 | 函数行为由布尔参数决定 | 拆分为两个函数 |
| 过长返回值 | 返回 >3 个值 | 封装为结构体 |

### 类/模块层面

| 异味 | 判定标准 | 建议 |
|------|----------|------|
| 上帝类 | 单个类/文件 >500 行 | 拆分职责 |
| 重复代码 | 两处以上相似度 >80% | 提取公共函数 |
| 魔法数字 | 代码中直接使用未命名常量 | 定义命名常量 |
| 过度注释 | 注释解释"做什么"而非"为什么" | 改善命名使代码自解释 |
| 死代码 | 被注释掉的代码块 | 删除（版本控制有历史） |

### 架构层面

| 异味 | 判定标准 | 建议 |
|------|----------|------|
| 循环依赖 | A 导入 B，B 导入 A | 提取接口或第三个模块 |
| 层级穿透 | Controller 直接操作数据库 | 通过 Service 层 |
| 配置硬编码 | 将配置写死在代码中 | 提取到配置文件 |
| 日志缺失 | 关键操作无日志 | 添加操作日志 |

## 安全头配置检查

### 必须设置的安全头

| 响应头 | 推荐值 | 作用 |
|--------|--------|------|
| `X-Content-Type-Options` | `nosniff` | 阻止浏览器 MIME 嗅探 |
| `X-Frame-Options` | `DENY` 或 `SAMEORIGIN` | 防止点击劫持 |
| `X-XSS-Protection` | `1; mode=block` | 启用浏览器 XSS 过滤 |
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` | 强制 HTTPS |
| `Content-Security-Policy` | 见下方 | 限制资源加载来源 |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | 控制 Referer 泄露 |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` | 禁用不需要的浏览器功能 |

### CSP 最小安全策略

```
Content-Security-Policy:
    default-src 'self';
    script-src 'self';
    style-src 'self' 'unsafe-inline';
    img-src 'self' data:;
    font-src 'self';
    frame-ancestors 'none';
    base-uri 'self';
    form-action 'self'
```

### Go 中间件设置示例

```go
func SecurityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
        next.ServeHTTP(w, r)
    })
}
```

### Python 中间件设置示例

```python
# Flask
@app.after_request
def set_security_headers(response):
    response.headers["X-Content-Type-Options"] = "nosniff"
    response.headers["X-Frame-Options"] = "DENY"
    response.headers["X-XSS-Protection"] = "1; mode=block"
    response.headers["Referrer-Policy"] = "strict-origin-when-cross-origin"
    response.headers["Content-Security-Policy"] = (
        "default-src 'self'; script-src 'self'; "
        "style-src 'self' 'unsafe-inline'; "
        "img-src 'self' data:; frame-ancestors 'none'"
    )
    return response
```
