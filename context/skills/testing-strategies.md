---
name: testing-strategies
label: 测试策略
description: 测试策略速查，涵盖边界值分析、等价类划分、mock/fixture 模式、测试命名规范和覆盖率目标
keywords: 测试,边界值,等价类,mock,fixture,pytest,覆盖率,参数化,测试设计,测试命名
level: 1
status: 1
---

# 测试策略速查

## 边界值分析表格模板

### 通用边界值分析表

对于每个输入参数，填写以下表格确定测试值：

| 参数名 | 类型 | 有效范围 | min-1 | min | min+1 | 典型值 | max-1 | max | max+1 |
|--------|------|----------|-------|-----|-------|--------|-------|-----|-------|
| age    | int  | 0-150    | -1    | 0   | 1     | 30     | 149   | 150 | 151   |
| name   | str  | 1-50字符 | ""    | "A" | "AB"  | "张三" | 49字符 | 50字符 | 51字符 |
| score  | float| 0.0-100.0| -0.1  | 0.0 | 0.1   | 60.0   | 99.9  | 100.0| 100.1 |
| items  | list | 1-100项  | []    | [x] | [x,y] | 50项   | 99项  | 100项| 101项 |

### 特殊类型的边界值

**字符串**：
| 场景 | 测试值 |
|------|--------|
| 空字符串 | `""` |
| 纯空白 | `"   "` |
| 单字符 | `"a"` |
| 最大长度 | `"a" * max_len` |
| 超长 | `"a" * (max_len + 1)` |
| 特殊字符 | `"<script>alert(1)</script>"` |
| Unicode | `"\u0000"`, `"\uffff"`, 中文/日文/emoji |

**数值**：
| 场景 | 测试值 |
|------|--------|
| 零 | `0` |
| 负数 | `-1` |
| 最小整数 | `-2^31`（int32） |
| 最大整数 | `2^31 - 1`（int32） |
| 浮点精度 | `0.1 + 0.2`（不等于 0.3） |
| 无穷 | `float("inf")` |
| NaN | `float("nan")` |

**日期时间**：
| 场景 | 测试值 |
|------|--------|
| 闰年2月29日 | `2024-02-29` |
| 非闰年2月29日 | `2023-02-29`（应拒绝） |
| 月末 | `2023-01-31` |
| 跨年 | `2023-12-31` -> `2024-01-01` |
| 时区边界 | UTC+14 / UTC-12 |

## 等价类划分方法

### 划分步骤

1. 识别输入参数及其规格说明
2. 为每个参数划分有效等价类和无效等价类
3. 从每个等价类中选取一个代表值
4. 编写测试用例，每个用例尽量只包含一个无效等价类（便于定位失败原因）

### 等价类划分示例

**用户注册邮箱验证**：

| 等价类类别 | 等价类 | 代表值 | 预期结果 |
|-----------|--------|--------|----------|
| 有效 | 标准邮箱 | `user@example.com` | 通过 |
| 有效 | 子域名邮箱 | `user@mail.example.com` | 通过 |
| 有效 | 加号邮箱 | `user+tag@example.com` | 通过 |
| 无效 | 缺少 @ | `userexample.com` | 拒绝 |
| 无效 | 缺少域名 | `user@` | 拒绝 |
| 无效 | 缺少用户名 | `@example.com` | 拒绝 |
| 无效 | 空字符串 | `""` | 拒绝 |
| 无效 | None | `None` | 拒绝 |
| 无效 | 多个 @ | `user@@example.com` | 拒绝 |
| 无效 | 特殊字符 | `user <script>@example.com` | 拒绝 |

**HTTP 状态码分类**：

| 等价类 | 范围 | 代表值 | 含义 |
|--------|------|--------|------|
| 信息 | 100-199 | 100 | 继续 |
| 成功 | 200-299 | 200 | 成功 |
| 重定向 | 300-399 | 301 | 永久重定向 |
| 客户端错误 | 400-499 | 400 | 请求错误 |
| 服务器错误 | 500-599 | 500 | 服务器错误 |
| 无效 | <100 | 99 | 无效状态码 |
| 无效 | >599 | 600 | 无效状态码 |

## mock/fixture 常用模式

### pytest fixture 模式

```python
import pytest

# 基础 fixture（function 作用域，每个测试独立执行）
@pytest.fixture
def sample_user():
    return {"id": 1, "name": "张三", "email": "zhang@example.com"}

# yield fixture（带清理）
@pytest.fixture
def temp_file(tmp_path):
    filepath = tmp_path / "test.txt"
    filepath.write_text("测试内容")
    yield filepath
    # teardown：tmp_path 会自动清理，这里可以做额外清理

# 参数化 fixture
@pytest.fixture(params=[
    {"role": "admin", "can_delete": True},
    {"role": "user", "can_delete": False},
    {"role": "guest", "can_delete": False},
], ids=["admin", "user", "guest"])
def user_with_role(request):
    return request.param

# 数据库 fixture（session 作用域，整个测试会话只创建一次）
@pytest.fixture(scope="session")
def db_engine():
    engine = create_engine("sqlite:///:memory:")
    Base.metadata.create_all(engine)
    yield engine
    engine.dispose()

@pytest.fixture
def db_session(db_engine):
    connection = db_engine.connect()
    transaction = connection.begin()
    session = Session(bind=connection)
    yield session
    session.close()
    transaction.rollback()  # 每个测试后回滚，保证隔离
    connection.close()
```

### mock/patch 模式

```python
from unittest.mock import patch, Mock, MagicMock

# 模式 1：patch 装饰器
@patch("myapp.service.requests.get")
def test_fetch_data(mock_get):
    mock_get.return_value = Mock(
        status_code=200,
        json=Mock(return_value={"data": "test"})
    )
    result = fetch_data("https://api.example.com")
    assert result == {"data": "test"}
    mock_get.assert_called_once_with("https://api.example.com")

# 模式 2：patch 上下文管理器
def test_send_email():
    with patch("myapp.service.smtp_client") as mock_smtp:
        mock_smtp.send.return_value = True
        result = send_notification("user@example.com", "测试")
        assert result is True
        mock_smtp.send.assert_called_once()

# 模式 3：side_effect 模拟异常
@patch("myapp.service.db.query")
def test_db_error(mock_query):
    mock_query.side_effect = ConnectionError("数据库连接失败")
    with pytest.raises(ServiceError):
        get_user(1)

# 模式 4：side_effect 模拟多次调用返回不同值
@patch("myapp.service.random.randint")
def test_retry(mock_randint):
    mock_randint.side_effect = [1, 2, 3]  # 第 1/2/3 次调用分别返回
    assert generate_id() == 1
    assert generate_id() == 2
```

## 测试命名规范

### 命名格式

```
test_{被测功能}_{场景}_{预期结果}
```

### 命名示例

| 测试名 | 说明 |
|--------|------|
| `test_login_valid_credentials_returns_token` | 登录-合法凭证-返回Token |
| `test_login_wrong_password_returns_401` | 登录-错误密码-返回401 |
| `test_login_empty_username_raises_validation_error` | 登录-空用户名-抛校验异常 |
| `test_create_user_duplicate_email_returns_409` | 创建用户-重复邮箱-返回409 |
| `test_paginate_page_zero_defaults_to_one` | 分页-页码0-默认第1页 |
| `test_upload_file_exceeds_limit_returns_400` | 上传-超大小限制-返回400 |
| `test_delete_user_as_guest_returns_403` | 删除用户-游客身份-返回403 |

### 命名原则

- 使用英文下划线命名，不用驼峰
- 被测功能用动词开头（login/create/delete/update/get）
- 场景描述具体条件（empty/invalid/duplicate/expired）
- 预期结果用动词（returns/raises/creates/deletes）
- 避免含糊名称（test_user_1、test_case_a）

## 覆盖率目标设定

### 分层覆盖率目标

| 层级 | 最低目标 | 理想目标 | 重点关注 |
|------|----------|----------|----------|
| 核心业务逻辑 | 90% | 95% | 支付/认证/权限/数据计算 |
| API 端点 | 80% | 90% | 参数校验/权限检查/错误处理 |
| 工具函数 | 80% | 90% | 边界条件/类型处理 |
| 配置/初始化 | 50% | 70% | 默认值/环境变量 |
| UI 组件 | 60% | 80% | 交互逻辑/状态管理 |

### 覆盖率类型

| 类型 | 说明 | 优先级 |
|------|------|--------|
| 行覆盖率 | 每行代码是否被执行 | 基础指标 |
| 分支覆盖率 | 每个 if/else 分支是否被执行 | 重要指标 |
| 条件覆盖率 | 复合条件中每个子条件的真/假是否都被测试 | 高级指标 |
| 路径覆盖率 | 所有可能的执行路径是否被测试 | 参考指标 |

### pytest 覆盖率命令

```bash
# 安装
pip install pytest-cov

# 运行并生成覆盖率报告
pytest --cov=myapp --cov-report=term-missing tests/

# 生成 HTML 报告
pytest --cov=myapp --cov-report=html tests/

# 设置最低覆盖率（低于此值则测试失败）
pytest --cov=myapp --cov-fail-under=80 tests/
```
