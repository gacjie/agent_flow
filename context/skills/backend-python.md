---
name: backend-python
label: Python 后端开发技能
description: Python 后端项目识别、实现和验证速查
keywords: Python,Django,Flask,FastAPI,pyproject,requirements,pytest
level: 1
status: 1
sort: 21
---

# Python 后端开发技能

## 技术栈识别

| 线索 | 判断 |
|------|------|
| `pyproject.toml`、`requirements.txt`、`Pipfile` | Python 依赖管理 |
| `manage.py` | Django |
| `app.py`、`wsgi.py` | Flask 或 WSGI 项目 |
| `main.py` + FastAPI import | FastAPI |
| `tests/`、`pytest.ini` | pytest 测试 |

## 实施要点

- 先确认框架、路由注册、依赖注入、ORM/仓储模式和异常处理。
- 优先使用项目已有虚拟环境、服务层、schema/serializer 和校验封装。
- 对 `None`、空集合、非法类型、超长输入和重复提交有明确处理。
- 认证权限变更同时检查依赖、装饰器、中间件或 permission class。
- 禁止对外部输入使用 `eval()`/`exec()`/`pickle.loads()`，无安全沙箱可依赖。
- async 函数中严禁调用同步阻塞 I/O（文件/网络/数据库），会阻塞事件循环致所有协程饿死。
- Django QuerySet 使用 `select_related`（外键 JOIN）/`prefetch_related`（多对多批量）预加载。
- 文件路径操作使用 `pathlib` 或 `werkzeug.utils.safe_join`，禁止直接拼接用户输入。
- 命令执行使用 `subprocess` 列表参数形式 `['cmd', arg1, arg2]`，禁止 `shell=True`。
- YAML 解析始终使用 `yaml.safe_load()`，禁止 `yaml.load()` 未指定安全 Loader。

## 验证命令

| 层次 | 优先命令 |
|------|---------|
| 语法 | `python -m py_compile <file>` 或导入目标模块 |
| 安全扫描 | `bandit -r .`（检测常见安全反模式） |
| 类型检查 | `mypy --strict <module>` |
| 部署检查 | `python manage.py check --deploy`（Django 安全配置检查） |
| 启动 | README、框架命令或入口模块 |
| 接口 | 项目已有测试客户端、HTTP 请求或框架测试工具 |
| 测试 | `pytest`、`python -m pytest` 或项目脚本 |

存在 `.venv` 时优先使用项目虚拟环境。

## 框架与项目模式

### Django 安全配置

`python manage.py check --deploy` 可一键检查以下配置项：

```python
# settings/production.py 必须项
DEBUG = False                          # 关闭调试信息暴露
ALLOWED_HOSTS = ['example.com']        # 明确允许域名
SESSION_COOKIE_SECURE = True           # Cookie 仅 HTTPS 传输
CSRF_COOKIE_SECURE = True              # CSRF Cookie 仅 HTTPS
SECURE_HSTS_SECONDS = 31536000         # 强制 HTTPS（一年）
SECURE_SSL_REDIRECT = True             # HTTP 自动跳转 HTTPS
SECURE_BROWSER_XSS_FILTER = True       # XSS 过滤头
```

中间件顺序：SecurityMiddleware → SessionMiddleware → CsrfViewMiddleware → AuthenticationMiddleware → MessageMiddleware

### FastAPI 关键模式

- 依赖注入 `Depends()` + Pydantic 模型自动验证请求体（类型 + 约束 + 嵌套）
- async 路由用于 I/O 密集场景（数据库查询/HTTP 调用/文件读写）
- CPU 密集操作使用 `run_in_executor` 或 `BackgroundTasks` 避免阻塞事件循环
- `HTTPException(status_code=, detail=)` 统一错误响应格式
- `Security` 依赖实现 OAuth2/JWT 认证

### Flask 安全要点

- `flask-talisman` — 安全头（CSP/HSTS/X-Frame-Options）
- `flask-limiter` — 请求限流保护
- `flask-wtf` — 表单 CSRF 保护
- `flask-login` — Session 管理（remember_me 安全配置）

### Python 3.14 新特性

安全相关：
- t-strings (PEP 750) — 模板字符串支持安全 SQL/HTML 构建，从语法层面防注入
- Sigstore 签名替代 PGP — 包发布完整性验证更可靠
- 改进的 SSL 默认配置 — TLS 1.3 优先

性能相关：
- Free-Threaded 模式 (PEP 779) — 移除 GIL，真正多线程并行（CPU 密集不再受限单核）
- 增量 GC — 减少垃圾回收的 STW 暂停对请求延迟的影响
- JIT 编译器实验支持 — copy-and-patch 策略提升热路径性能

### 常见项目结构模式

```
Django:    project/settings/ + apps/app_name/models|views|urls|serializers
FastAPI:   app/main.py + app/routers/ + app/models/ + app/schemas/ + app/deps/
Flask:     app/__init__.py(create_app) + app/blueprints/ + app/models/
```

### 依赖管理

- `pyproject.toml` (PEP 621) — 现代标准，推荐新项目使用
- `requirements.txt` — 传统方式，`pip freeze > requirements.txt` 锁定版本
- `poetry` / `pdm` — 带 lock 文件的完整依赖管理
- 生产部署：固定版本号（`==`），开发可用范围（`>=1.0,<2.0`）

### 工具链推荐

按推荐顺序：
- `bandit` — 安全漏洞模式扫描
- `mypy` / `pyright` — 静态类型检查
- `ruff` — lint + format 统一工具（替代 flake8+black+isort，速度快 100 倍）
- `pytest` + `coverage` — 测试 + 覆盖率
- `pip-audit` — 依赖已知漏洞扫描

## 安全风险

Python 语言和框架特有的安全漏洞：

| 风险 | 说明 | 防护 |
|------|------|------|
| Pickle 反序列化 RCE | `pickle.loads()` 对不可信数据可通过 `__reduce__` 执行任意代码 | 使用 JSON/MessagePack 替代，Pickle 无法安全沙箱化 |
| YAML 反序列化 | `yaml.load()` 默认 Loader 可实例化任意 Python 对象→RCE | 始终使用 `yaml.safe_load()`，禁止裸 `yaml.load()` |
| eval/exec 代码执行 | 对用户输入 `eval()`/`exec()` 可执行任意 Python 代码 | 彻底禁止对任何外部输入使用 eval/exec，无例外 |
| os.system 命令注入 | `subprocess(shell=True)` 或 `os.system()` 拼接用户输入→shell 命令注入 | `shlex.quote()` 转义 + subprocess 列表参数 `['cmd', arg]` |
| SSRF | `requests.get(url)` / urllib 未验证目标（支持 file:// gopher:// 等协议） | 验证并限制出站 URL 协议和目标地址，禁止访问内网 IP 段 |
| 路径穿越 | `os.path.join('/base', '../etc/passwd')` 结果为 `/etc/passwd`（忽略基路径） | `werkzeug.utils.safe_join` / `secure_filename` / pathlib 前缀验证 |
| Jinja2 SSTI | 用户控制字符串传入 `Template(user_input).render()`→服务端模板注入 | 用户输入只作为模板变量传递，永不作为模板内容本身 |
| Django ORM 注入 | `extra(where=[f"...{input}..."])` / `RawSQL` / `raw()` 未正确参数化 | 使用 ORM 查询集方法或 `raw(sql, [params])` 列表参数化 |
| Django CSRF 禁用 | 滥用 `@csrf_exempt` 装饰器覆盖大量视图→表单伪造 | 最小化豁免范围，纯 API 视图用 Token 认证替代 CSRF |
| Django \|safe 滥用 | 模板中对用户输入使用 `{{ var\|safe }}` 禁用自动转义→XSS | 只对经后端确认安全的内容使用，用户输入永不标记 safe |
| Django DEBUG=True | 生产环境暴露所有变量值、SQL 查询历史、完整堆栈和配置密钥 | 生产必须 `DEBUG=False`，通过 `check --deploy` 自动验证 |
| 漏洞链式利用 | 路径穿越→读取配置→发现内部端点→任意文件写+LFI=RCE（多漏洞组合） | 纵深防御原则：每层独立验证，不依赖单一防线 |

## 性能陷阱

Python 语言和框架特有的性能问题：

| 问题 | 表现 | 优化 |
|------|------|------|
| GIL 限制 | CPU 密集型任务受限单核，多线程无法利用多核并行 | `multiprocessing` 多进程 / C 扩展 / Python 3.14 Free-Threaded 模式真并行 |
| asyncio 阻塞 | 事件循环被同步调用卡死，所有协程饿死无法响应 | async 函数中严禁同步 I/O，阻塞操作用 `loop.run_in_executor(None, func)` |
| Django QuerySet 惰性 | 看似一行代码实际触发数十次 SQL（循环内访问外键属性） | `select_related`（外键 JOIN）/ `prefetch_related`（多对多批量）/ `only()`/`defer()` |
| SQLAlchemy 连接池 | 连接耗尽所有请求排队超时 / 连接老化随机断开 | 配置 `pool_size=10` / `pool_recycle=3600` / `pool_pre_ping=True` 验证连接存活 |
| 盲目优化 | 优化了不是真正瓶颈的代码，浪费时间且无实际收益 | `cProfile` 全局剖析 / `line_profiler` 逐行定位→确认瓶颈再优化 |
| 重复计算 | 相同输入反复计算相同结果，浪费 CPU 和响应时间 | `functools.lru_cache` 本地缓存 / Redis 分布式缓存 / 结果持久化 |
