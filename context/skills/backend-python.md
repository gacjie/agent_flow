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

## 验证命令

| 层次 | 优先命令 |
|------|---------|
| 语法 | `python -m py_compile <file>` 或导入目标模块 |
| 启动 | README、框架命令或入口模块 |
| 接口 | 项目已有测试客户端、HTTP 请求或框架测试工具 |
| 测试 | `pytest`、`python -m pytest` 或项目脚本 |

存在 `.venv` 时优先使用项目虚拟环境。
