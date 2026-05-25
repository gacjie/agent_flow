---
name: frontend-security
label: 前端安全编程
description: 前端安全编程速查，涵盖 XSS 防御、CSRF Token 携带、CSP 策略、安全 DOM 操作和敏感数据处理规范
keywords: XSS,CSRF,CSP,安全,前端,DOM,innerHTML,textContent,Cookie,敏感数据
level: 1
status: 1
---

# 前端安全编程速查

## XSS 防御的 5 种方法

### 方法 1：使用 textContent 替代 innerHTML

```javascript
// 危险：innerHTML 会解析 HTML 标签，用户输入可能包含 <script>
element.innerHTML = userInput;  // XSS 风险

// 安全：textContent 只设置纯文本，HTML 标签不会被解析
element.textContent = userInput;  // 安全
```

适用场景：所有需要将用户输入显示到页面的场景。

### 方法 2：HTML 实体转义

当必须插入包含 HTML 结构的内容时，对用户可控部分进行转义：

```javascript
function escapeHTML(str) {
    const div = document.createElement("div");
    div.textContent = str;
    return div.innerHTML;
}

// 使用：将用户输入嵌入 HTML 结构
const safeHTML = `<span class="username">${escapeHTML(userInput)}</span>`;
```

转义对照表：
| 字符 | 转义后 |
|------|--------|
| `&`  | `&amp;` |
| `<`  | `&lt;` |
| `>`  | `&gt;` |
| `"`  | `&quot;` |
| `'`  | `&#x27;` |

### 方法 3：使用 DOM API 构建元素

```javascript
// 危险：拼接 HTML 字符串
container.innerHTML = `<a href="${url}">${name}</a>`;  // url 和 name 都可能包含恶意代码

// 安全：使用 DOM API
const link = document.createElement("a");
link.href = url;          // 浏览器会自动处理 href 的安全性
link.textContent = name;  // 纯文本，不解析 HTML
container.appendChild(link);
```

### 方法 4：URL 参数编码

```javascript
// 危险：直接拼接用户输入到 URL
const url = `/search?q=${userInput}`;  // userInput 可能包含 &、= 等特殊字符

// 安全：使用 encodeURIComponent
const url = `/search?q=${encodeURIComponent(userInput)}`;

// 更安全：使用 URL API
const url = new URL("/search", window.location.origin);
url.searchParams.set("q", userInput);  // 自动编码
```

### 方法 5：CSP 作为最后防线

即使前面的措施有遗漏，CSP 可以阻止内联脚本执行（见下方 CSP 策略配置）。

## CSRF Token 携带模式

### Cookie 双提交模式

```javascript
// 从 Cookie 中读取 CSRF Token
function getCSRFToken() {
    const match = document.cookie.match(/csrf_token=([^;]+)/);
    return match ? match[1] : "";
}

// 方式 1：表单提交时添加隐藏字段
const form = document.querySelector("form");
const input = document.createElement("input");
input.type = "hidden";
input.name = "csrf_token";
input.value = getCSRFToken();
form.appendChild(input);

// 方式 2：AJAX 请求时添加自定义头
fetch("/api/data", {
    method: "POST",
    headers: {
        "Content-Type": "application/json",
        "X-CSRF-Token": getCSRFToken()
    },
    body: JSON.stringify(data)
});

// 方式 3：全局拦截器（统一添加）
const originalFetch = window.fetch;
window.fetch = function(url, options = {}) {
    if (options.method && options.method !== "GET") {
        options.headers = options.headers || {};
        options.headers["X-CSRF-Token"] = getCSRFToken();
    }
    return originalFetch.call(this, url, options);
};
```

### 表单中的 CSRF Token（模板引擎）

```html
<!-- Go html/template -->
<form method="POST" action="/admin/users">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <!-- 表单字段 -->
</form>

<!-- Jinja2 -->
<form method="POST">
    <input type="hidden" name="csrf_token" value="{{ csrf_token() }}">
</form>
```

## CSP 策略配置

### 推荐的 CSP 策略

```
Content-Security-Policy:
    default-src 'self';
    script-src 'self';
    style-src 'self' 'unsafe-inline';
    img-src 'self' data: https:;
    font-src 'self';
    connect-src 'self';
    frame-ancestors 'none';
    base-uri 'self';
    form-action 'self';
```

各指令含义：
| 指令 | 含义 | 推荐值 |
|------|------|--------|
| `default-src` | 默认来源 | `'self'` |
| `script-src` | JavaScript 来源 | `'self'`（禁止内联脚本） |
| `style-src` | CSS 来源 | `'self' 'unsafe-inline'`（允许内联样式） |
| `img-src` | 图片来源 | `'self' data: https:` |
| `connect-src` | AJAX/WebSocket 来源 | `'self'` |
| `frame-ancestors` | 允许嵌入的父页面 | `'none'`（防止点击劫持） |
| `base-uri` | `<base>` 标签限制 | `'self'` |
| `form-action` | 表单提交目标 | `'self'` |

### 逐步启用 CSP

```
# 第 1 步：仅报告不阻止（观察期）
Content-Security-Policy-Report-Only: default-src 'self'; report-uri /csp-report

# 第 2 步：确认无误后切换为强制模式
Content-Security-Policy: default-src 'self'; ...
```

## 安全的 DOM 操作清单

### 安全操作（推荐使用）

| 操作 | 说明 |
|------|------|
| `element.textContent = value` | 设置纯文本，不解析 HTML |
| `element.setAttribute("data-x", value)` | 设置自定义属性 |
| `element.classList.add/remove/toggle` | 操作 CSS 类名 |
| `element.style.property = value` | 设置内联样式 |
| `document.createElement` + `appendChild` | 构建 DOM 树 |
| `element.value = value`（input/textarea） | 设置表单值 |

### 危险操作（需要特别注意）

| 操作 | 风险 | 安全替代 |
|------|------|----------|
| `element.innerHTML = value` | XSS：解析 HTML 标签和脚本 | `textContent` 或 `createElement` |
| `document.write(value)` | XSS：向文档写入任意 HTML | 使用 DOM API |
| `eval(value)` | 代码注入：执行任意 JavaScript | `JSON.parse` 或其他安全解析 |
| `new Function(value)` | 代码注入：同 eval | 避免动态构造函数 |
| `setTimeout(stringValue, ms)` | 代码注入：字符串参数会被 eval | 传入函数引用 |
| `element.insertAdjacentHTML(pos, value)` | XSS：同 innerHTML | 构建 DOM 元素后 insert |
| `location.href = value` | 开放重定向 / javascript: 协议 | 白名单校验 URL |

### URL 安全检查

```javascript
// 防止 javascript: 协议的 XSS
function isSafeURL(url) {
    try {
        const parsed = new URL(url, window.location.origin);
        return ["http:", "https:"].includes(parsed.protocol);
    } catch {
        return false;
    }
}

// 使用
if (isSafeURL(userProvidedURL)) {
    link.href = userProvidedURL;
}
```

## 敏感数据前端处理规范

### 规则清单

1. **密码**：不在前端存储（localStorage/sessionStorage/Cookie），表单提交后立即清除输入框值
2. **Token**：存储在 HttpOnly + Secure + SameSite=Strict 的 Cookie 中，不存 localStorage
3. **身份证/手机号**：显示时脱敏（`138****1234`、`3201**********1234`）
4. **API Key**：绝不在前端代码中硬编码，通过后端代理请求
5. **错误信息**：生产环境不展示堆栈跟踪和内部错误细节

### 脱敏函数

```javascript
// 手机号脱敏
function maskPhone(phone) {
    return phone.replace(/(\d{3})\d{4}(\d{4})/, "$1****$2");
}

// 邮箱脱敏
function maskEmail(email) {
    const [name, domain] = email.split("@");
    const masked = name.length <= 2
        ? name[0] + "*"
        : name[0] + "*".repeat(name.length - 2) + name.slice(-1);
    return `${masked}@${domain}`;
}

// 身份证脱敏
function maskIDCard(id) {
    return id.replace(/(\d{4})\d{10}(\d{4})/, "$1**********$2");
}
```

### 安全的 Cookie 设置

```javascript
// 后端设置安全 Cookie（前端不应设置敏感 Cookie）
// Set-Cookie: session_token=xxx; HttpOnly; Secure; SameSite=Strict; Path=/; Max-Age=86400

// Cookie 属性说明：
// HttpOnly   - JavaScript 无法读取，防止 XSS 窃取
// Secure     - 仅通过 HTTPS 传输
// SameSite   - Strict: 完全禁止跨站携带 / Lax: 允许导航级跨站
// Path       - 限制 Cookie 作用路径
// Max-Age    - 过期时间（秒），避免永久 Cookie
```
