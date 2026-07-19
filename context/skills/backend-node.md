---
name: backend-node
label: Node.js 后端开发技能
description: Node.js/TypeScript 后端项目识别、实现和验证速查
keywords: Node.js,TypeScript,Express,Nest,Fastify,Koa,npm,yarn,pnpm,jest,vitest
level: 1
status: 1
sort: 23
---

# Node.js 后端开发技能

## 技术栈识别

| 线索 | 判断 |
|------|------|
| `package.json` | Node.js 项目 |
| `tsconfig.json` | TypeScript |
| `package-lock.json`、`yarn.lock`、`pnpm-lock.yaml` | 包管理器优先级 |
| `src/main.ts`、`@nestjs` | NestJS |
| `express`、`fastify`、`koa` | HTTP 框架线索 |

## 实施要点

- 先确认 JS/TS、模块格式（CJS/ESM）、框架、路由注册、ORM/数据访问和校验方式。
- 新文件跟随所在模块语言、导出风格、命名规范和错误处理中间件模式。
- 不确定包管理器时先看 lock 文件和 package.json scripts 字段。
- 认证权限变更同时检查 guard/middleware、service、route/controller 和数据范围过滤。
- 禁止 `child_process.exec()`/`eval()`/`new Function()` 接受用户输入，用 `execFile` + 参数数组。
- JSON 解析/对象合并前过滤 `__proto__`/`constructor`/`prototype` 键，防原型链污染。
- 正则表达式避免嵌套量词和灾难性回溯，大输入或不可控模式使用 `re2` 安全库。
- 事件循环中禁止同步 I/O（`readFileSync`/`writeFileSync`），生产代码必须全部异步。
- Stream 处理必须正确处理背压：监听 `drain` 事件或使用 `stream.pipeline()`。
- 生产部署必须设置 `NODE_ENV=production`，框架据此启用视图缓存和优化。

## 验证命令

| 层次 | 优先命令 |
|------|---------|
| 编译/语法 | TypeScript 用 `npx tsc --noEmit`；JS 用 `node --check <file>` |
| 依赖漏洞 | `npm audit` / `yarn audit` / `pnpm audit` |
| Lint | `npx eslint .` 或项目配置的 lint 脚本 |
| 性能诊断 | `npx clinic doctor -- node server.js`（事件循环诊断） |
| 启动 | `npm/yarn/pnpm` scripts 中的 start/dev 脚本 |
| 接口 | HTTP 请求、supertest 或项目测试客户端 |
| 测试 | `npm test`、`npx jest`、`npx vitest` 或项目脚本 |

只修复本次变更引入的失败；历史失败记录到总结。

## 框架与项目模式

### Express 安全与性能配置

```javascript
// 安全头（一行代码 13 个安全响应头）
app.use(helmet());

// CORS 严格配置（禁止 * 通配符）
app.use(cors({ origin: ['https://example.com'], credentials: true }));

// 限流（按用户/API Key，非仅 IP）
app.use(rateLimit({ windowMs: 15*60*1000, max: 100 }));

// 请求体大小限制（防内存耗尽）
app.use(express.json({ limit: '1mb' }));
```

- 错误处理中间件放在所有路由之后：`app.use((err, req, res, next) => {...})`
- 404 处理放在路由之后、错误中间件之前

### NestJS 关键模式

- `ValidationPipe({ whitelist: true, transform: true })` — 全局输入验证 + 自动剥离未声明字段
- `Guard` — 认证/授权守卫（JWT / Session / API Key / Role-based）
- `Interceptor` — 日志/缓存/响应序列化/超时/异常格式化拦截
- 模块化依赖注入容器 — 天然支持单元测试 mock 和模块隔离
- `ThrottlerGuard` — 内置限流守卫

### Fastify 优势

- JSON Schema 验证请求/响应（比 Express 快 2-3 倍序列化性能）
- 插件封装（每个插件有独立的装饰器作用域，避免全局污染）
- 内置结构化日志（pino，JSON 格式，生产友好）
- Hook 生命周期清晰：onRequest → preParsing → preValidation → preHandler → handler

### Node.js 版本策略

- Node.js 23 已 EOL（2025.06），不再接收安全补丁
- 生产环境应使用 Node.js 26（当前 LTS，支持到 2028-04）
- 版本规则：偶数版本号 = LTS 长期支持，奇数版本号 = Current 短期支持
- 升级路径：检查 `engines` 字段 → 运行测试 → 验证原生模块兼容性

### 常见项目结构模式

```
Express:  src/routes/ + src/controllers/ + src/services/ + src/models/ + src/middlewares/
NestJS:   src/modules/feature/(controller|service|module|dto|entity).ts
Fastify:  src/plugins/ + src/routes/ + src/schemas/ + src/services/
```

### 模块系统注意

- CJS (`require`/`module.exports`) vs ESM (`import`/`export`) 不可混用
- `package.json` 中 `"type": "module"` 决定默认模块格式
- TypeScript `esModuleInterop: true` 桥接 CJS/ESM 差异
- 动态 import `await import()` 两种模块系统通用

### 工具链推荐

- `npm audit` / `snyk` — 依赖已知漏洞扫描
- `eslint` + `@typescript-eslint` — 代码质量 + 安全规则
- Clinic.js — 事件循环延迟 / 内存泄漏 / I/O 瓶颈三合一诊断
- `node --inspect` + Chrome DevTools — CPU profiling / 堆快照分析
- `0x` — 火焰图生成（定位 CPU 热点函数）

## 安全风险

Node.js/JavaScript 特有的安全漏洞：

| 风险 | 说明 | 防护 |
|------|------|------|
| 原型链污染 | 通过 `__proto__`/`constructor.prototype` 修改 Object.prototype→影响所有对象→RCE | `Object.create(null)` 无原型对象 / `Object.freeze(Object.prototype)` / 合并前过滤危险键 |
| ReDoS 正则 DoS | 恶意输入触发正则灾难性回溯→阻塞整个事件循环→所有请求超时（单线程） | 安全正则库 `re2`（线性时间保证）/ 输入长度限制 / 避免 `(a+)+` 嵌套量词 |
| 命令注入 | `child_process.exec(userInput)` / `eval()` / `new Function(userInput)` 执行任意代码 | 禁止 exec 拼接用户输入、使用 `execFile('cmd', [arg1, arg2])` 参数数组 |
| 事件循环阻塞 DoS | 同步 I/O / 大 JSON 解析 / CPU 密集计算阻塞单线程→全部并发请求排队 | 全部使用 async API / Worker Threads 处理密集计算 / `setImmediate` 分片 |
| 缺少安全头 | Express 默认不设置任何安全响应头（无 CSP/HSTS/X-Frame-Options） | `helmet()` 中间件一行代码自动添加 13 个安全响应头 |
| SSRF | 服务端 HTTP 请求未验证目标地址→访问内部服务/云元数据（169.254.169.254） | URL 白名单 / 禁止内网 IP 段 / 禁止 `file://` 等非 HTTP 协议 |
| 路径穿越 | 不安全路径拼接中 `../` 逃逸基目录→读写服务器任意文件 | `path.resolve()` 后验证结果路径以预期基目录开头 |
| 依赖链攻击 | 深层传递依赖漏洞影响面极广（CVE-2024-21538 cross-spawn 影响数万项目） | `npm audit` 定期扫描 / lockfile 锁定版本 / 最小化依赖树深度 |
| JSON __proto__ 注入 | JSON 反序列化 `{"__proto__": {"isAdmin": true}}` 污染对象原型链 | JSON Schema 严格验证输入结构 / 安全对象合并库 / 过滤 `__proto__` 键 |
| CVE-2024-52798 | path-to-regexp/Express 路由 ReDoS，影响所有使用动态路由的 Express 应用 | 升级 path-to-regexp 8.x+ / Express 5.x / 路由参数正则长度限制 |
| Buffer 内存泄露 | CVE-2025-55131：`Buffer.alloc` 特定条件泄露未初始化堆内存（含历史 token/密码） | 升级 Node.js 26+ / 显式验证 Buffer 内容已正确零初始化 |
| HTTP/2 崩溃 | CVE-2025-59465：畸形 HEADERS 帧触发断言失败→进程直接崩溃（服务不可用） | 升级 Node.js + 配置 `http2.maxHeaderListSize` / `maxSessionMemory` 限制 |

## 性能陷阱

Node.js 特有的性能问题和优化方案：

| 问题 | 表现 | 优化 |
|------|------|------|
| 事件循环阻塞 | 所有并发请求排队等待、延迟从毫秒级突增到秒级 | 异步 API 替代同步版本（`fs.readFile` 替代 `readFileSync`），任何代码路径禁止同步 I/O |
| CPU 密集在主线程 | 单次密集计算（加密/压缩/解析）阻塞所有其他并发请求处理 | `Worker Threads` 分担计算（2-4 个起步按 CPU 核数扩展）；或 `child_process.fork` 隔离 |
| 无 Clustering | 单进程只能利用单核 CPU 性能，服务器其他核心空闲浪费 | `cluster` 模块 fork 多进程 / PM2 cluster mode / K8s Pod 水平扩展 |
| 内存泄漏 | 堆内存持续增长→GC 频率增加→最终 OOM kill 重启 | 检查闭包持有大对象 / `EventEmitter.setMaxListeners` / `WeakRef` / Chrome DevTools 堆快照对比 |
| Stream 背压忽略 | 快速生产者淹没慢速消费者→可写流内部缓冲区无限膨胀→内存溢出 | 检查 `write()` 返回值为 false 时暂停 / 监听 `drain` 事件 / 使用 `stream.pipeline()` |
| NODE_ENV 未设 production | Express 不缓存视图模板 / 保留详细错误堆栈 / 依赖库保留调试代码路径 | 生产环境必须 `NODE_ENV=production`，Docker/PM2 配置中显式设置环境变量 |
