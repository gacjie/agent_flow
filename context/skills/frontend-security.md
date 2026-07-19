---
name: frontend-security
label: 前端综合安全技能
description: 前端安全风险触发项、XSS/CSRF/CSP防护和高风险模式清单，供设计师裁剪产出项目前端安全文档
keywords: XSS,CSRF,CSP,安全头,DOM注入,postMessage,第三方脚本,SRI
level: 2
status: 1
sort: 11
---

# 前端综合安全技能

> 前端安全规范全面清单。设计师读取后按项目裁剪产出 docs/frontend-security.md——过滤项目不涉及的安全面，将适用项转化为具体技术方案。开发/测试/审查智能体优先使用项目文档，无项目文档时读此技能作为降级。

## 风险触发项（前端）

| 风险面 | 触发条件 | 关注点 |
|--------|---------|--------|
| 输出渲染 | 页面、模板、富文本、小程序视图 | 输出编码/转义、用户可控 URL、错误页安全边界、危险 DOM（innerHTML/eval/document.write）、日志和模板变量注入 |
| 前端状态 | 异步数据、表单交互、权限/认证失败、客户端存储 | 加载/空/错误/权限失败状态处理、表单边界（空选中/动态字段/禁用/重复提交）、异步时序和协调 |
| 资源加载 | 第三方脚本/资源、用户可控 URL、外部内容 | 来源限制、同源路径校验、完整性校验（SRI）、CSP 相关考虑、跳转地址白名单 |
| 跨窗口通信 | postMessage、iframe 嵌入、window.open | origin 校验、消息格式验证、最小化暴露接口 |
| 文档一致性 | 前后端字段、状态值、路径、响应结构 | 前后端接口契约对齐、CSS class 定义覆盖、资源引用完整性 |

### 各阶段处理动作

| 阶段 | 触发后动作 |
|------|-----------|
| 需求 | 记录前端安全需求和验收点 |
| 设计 | 转化为前端安全约定和组件规范 |
| 开发 | 实现客户端安全边界和用户可见状态 |
| 测试 | 验证渲染安全、交互边界和资源加载 |
| 审查 | 检查输出编码、状态处理和前后端一致性 |

## 安全检查清单

### 输出与渲染安全

- 用户可控文本使用 `textContent`、模板自动转义或创建节点，不拼入 HTML
- 仅对可信静态模板片段使用 HTML 插入；属性值使用 DOM API 设置
- HTML 转义函数必须覆盖 `< > & ' "`，或使用 DOM API 设置属性值
- 第三方资源、跳转地址和外部内容展示限制来源并避免开放跳转
- 错误页和错误信息不泄露服务端路径、堆栈或内部细节

### DOM 型 XSS 防护

以下 API 可将字符串作为 HTML 或代码执行，必须严格管控：

| 危险 API | 风险 | 安全替代 |
|---------|------|---------|
| `innerHTML` / `outerHTML` | 注入的字符串作为 HTML 解析执行 | `textContent` 或 DOM API 创建节点 |
| `document.write()` | 直接向文档流写入 HTML | DOM API 操作 |
| `eval()` / `Function()` | 将字符串作为 JavaScript 执行 | JSON.parse / 安全的数据处理逻辑 |
| `setTimeout(string)` | 字符串参数等同 eval | 传入函数引用而非字符串 |
| `insertAdjacentHTML()` | 同 innerHTML 风险 | `textContent` + DOM API |
| `location.href = userInput` | 可注入 `javascript:` 协议 | 校验 URL 协议白名单（http/https） |

### CSP 内容安全策略

| 指令 | 作用 | 推荐值 |
|------|------|--------|
| `default-src` | 所有资源默认来源 | `'self'` |
| `script-src` | JavaScript 加载来源 | `'self'`，避免 `'unsafe-inline'` `'unsafe-eval'` |
| `style-src` | CSS 加载来源 | `'self' 'unsafe-inline'`（内联样式需要时） |
| `img-src` | 图片加载来源 | `'self' data:`（按需添加 CDN 域名） |
| `connect-src` | XHR/Fetch/WebSocket 目标 | `'self'`（按需添加 API 域名） |
| `frame-ancestors` | 允许嵌入本页的父页面 | `'none'`（替代 X-Frame-Options） |
| `base-uri` | `<base>` 标签允许的 URL | `'self'`（防止 base 标签劫持） |
| `form-action` | 表单提交目标 | `'self'` |

**部署建议**：先用 `Content-Security-Policy-Report-Only` 观察违规，再切换为强制模式。

### 请求与凭证安全

- 状态变更请求按项目方式携带 CSRF/认证信息
- 客户端校验不替代后端校验，只做体验和早期拦截
- 不覆盖浏览器原生 API（window.confirm/alert/open/fetch 等）
- Cookie 和凭证信息不在 URL 参数或日志中暴露

### 跨窗口通信安全

- `postMessage` 接收方必须校验 `event.origin`，拒绝非预期来源
- 发送方指定目标 origin（第二参数），不使用 `'*'` 通配符
- 消息内容做格式/类型校验，不直接执行接收到的字符串
- iframe 嵌入使用 `sandbox` 属性限制能力（按需开放）

### 第三方脚本与资源安全

- 外部脚本添加 SRI 校验：`<script src="..." integrity="sha384-..." crossorigin="anonymous">`
- CSP 限制脚本来源域名，不使用 `script-src *` 通配符
- 评估第三方脚本的权限范围（能否访问 Cookie/DOM/用户数据）
- 不引入新前端库，除非任务明确要求或项目已使用
- 定期审计第三方依赖版本和已知漏洞

### 客户端状态安全

- fetch/XHR 请求具备超时或取消机制，避免永久等待
- 请求失败、非成功状态、非法 JSON 或业务错误必须有用户可见处理
- 不把 `fetch` resolved 或 HTTP 200 当作业务成功
- 权限或认证失败（401/403）有用户可见失败态，不把错误当空数据
- 交互组件处理键盘焦点、ARIA/语义状态、事件防抖/节流
- JS 异常、资源 404、无效 JSON、网络超时有可诊断或防抖处理

### 资源与加载安全

- 新增页面引用项目基础样式和脚本；新增 CSS/JS 被对应模板引用
- 模板中使用的 CSS class 在项目 CSS 中有定义
- 新增 CSS/JS 文件的引用不产生 404

## 审查高风险模式（前端）

以下模式在审查中应重点关注：

- 前后端字段、状态值、路径或响应结构不一致
- 项目文档与实现的前端规范、CSS class、路由或待确认标记不一致
- 路由、菜单入口、静态资源或启动配置缺失
- 模板 CSS class 名在项目 CSS 中无定义
- 查询参数、表单、JSON 或配置值来源混用导致覆盖
- 动态选择器、字段名未转义
- 用户输入直接进入 innerHTML/eval/document.write 等危险 API
- postMessage 未校验 origin 或消息内容未验证
- 测试只覆盖成功路径、缺少断言或未覆盖权限边界
- 异步数据依赖未协调，回填前未确认元素/选项存在
- 表单缺少空选中、禁用态、重复提交或非法响应处理
