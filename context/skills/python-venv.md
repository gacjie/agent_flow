---
name: python-venv
label: Python 虚拟环境管理
description: Python 项目虚拟环境标准操作，统一使用 .venv/ 目录，涵盖创建、安装依赖、激活和常用命令
keywords: Python,venv,虚拟环境,pip,requirements,pytest,依赖管理,.venv
level: 1
status: 1
---

# Python 虚拟环境管理规范

## 虚拟环境目录约定

所有 Python 项目统一使用 **`.venv/`** 作为虚拟环境目录（隐藏目录，不提交 Git）。

## 初始化流程

### 步骤 1：检查是否已存在

```bash
# 检查 .venv 目录是否存在
test -d .venv && echo "已存在" || echo "需要创建"
```

### 步骤 2：若不存在则创建

```bash
python -m venv .venv
```

### 步骤 3：查找并安装依赖

按以下顺序查找 requirements 文件：

```bash
# 优先顺序：requirements.txt → pip/requirements.txt → requirements-dev.txt

# 方案 A：根目录有 requirements.txt
.venv/bin/pip install -r requirements.txt

# 方案 B：pip/ 子目录
.venv/bin/pip install -r pip/requirements.txt

# 方案 C：同时存在开发依赖
.venv/bin/pip install -r requirements.txt -r requirements-dev.txt 2>/dev/null || true
```

### 完整一键初始化命令

```bash
# Linux / Mac / Git Bash（Windows 下 bash 环境）
test -d .venv || python -m venv .venv && \
  .venv/bin/pip install -r requirements.txt 2>/dev/null || \
  .venv/bin/pip install -r pip/requirements.txt 2>/dev/null || true
```

## 执行 Python 命令规范

**始终使用 `.venv/bin/python`，不使用系统 `python`：**

```bash
# 运行脚本
.venv/bin/python app.py

# 运行模块
.venv/bin/python -m pytest tests/ -v

# 检查导入
.venv/bin/python -c "from myapp import app; print('OK')"

# 安装新包
.venv/bin/pip install <package>
```

## 跨平台说明

| 平台 | Python 路径 | pip 路径 |
|------|------------|---------|
| Linux / Mac | `.venv/bin/python` | `.venv/bin/pip` |
| Windows CMD | `.venv\Scripts\python.exe` | `.venv\Scripts\pip.exe` |
| Windows bash (Git Bash) | `.venv/Scripts/python` | `.venv/Scripts/pip` |

> 系统为 Windows 时，bash 环境下用 `.venv/Scripts/python`；PowerShell/CMD 用 `.venv\Scripts\python.exe`

## pytest 标准执行命令

```bash
# 基础运行
.venv/bin/python -m pytest tests/ -v --tb=long

# 快速失败（-x 遇第一个失败即停止）
.venv/bin/python -m pytest tests/ -v --tb=short -x

# 覆盖率报告
.venv/bin/python -m pytest tests/ --cov=myapp --cov-report=term-missing

# 按关键词过滤
.venv/bin/python -m pytest tests/ -k "test_login"

# 安装测试依赖（若未在 requirements.txt 中）
.venv/bin/pip install pytest pytest-cov
```

## 常见问题排查

| 问题 | 原因 | 解决 |
|------|------|------|
| `ModuleNotFoundError` | 依赖未安装 | `pip install -r requirements.txt` |
| `python: command not found` | 未使用 venv 路径 | 改用 `.venv/bin/python` |
| `pip` 版本过旧 | 旧版 pip | `.venv/bin/pip install --upgrade pip` |
| ImportError 已安装的包 | 安装到了系统 Python | 确认使用 `.venv/bin/pip install` |
