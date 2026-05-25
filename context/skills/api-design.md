---
name: api-design
label: API 设计规范
description: API 设计规范速查，涵盖 RESTful URL 设计、HTTP 方法语义、状态码、分页、错误响应和版本控制
keywords: API,REST,RESTful,HTTP,状态码,分页,错误处理,版本控制,URL设计,HATEOAS
level: 1
status: 1
---

# API 设计规范速查

## RESTful URL 设计

### URL 命名规则

| 规则 | 正确 | 错误 |
|------|------|------|
| 使用名词复数 | `/api/users` | `/api/user`、`/api/getUsers` |
| 使用连字符分隔 | `/api/user-profiles` | `/api/user_profiles`、`/api/userProfiles` |
| 使用小写字母 | `/api/users` | `/api/Users` |
| 资源嵌套不超过 2 层 | `/api/users/123/orders` | `/api/users/123/orders/456/items/789` |
| 不包含动词 | `POST /api/users` | `/api/createUser` |
| 不包含文件扩展名 | `/api/users/123` | `/api/users/123.json` |

### 资源层级设计

```
# 一级资源（顶级实体）
GET    /api/users
POST   /api/users
GET    /api/users/{id}
PUT    /api/users/{id}
DELETE /api/users/{id}

# 二级资源（从属关系）
GET    /api/users/{id}/orders
POST   /api/users/{id}/orders
GET    /api/users/{id}/orders/{order_id}

# 操作型端点（无法用 CRUD 表达的动作）
POST   /api/users/{id}/activate
POST   /api/users/{id}/reset-password
POST   /api/orders/{id}/cancel
```

### 过滤、排序和搜索

```
# 过滤：使用查询参数
GET /api/users?status=active&role=admin

# 排序：sort 参数，- 前缀表示降序
GET /api/users?sort=created_at        # 升序
GET /api/users?sort=-created_at       # 降序
GET /api/users?sort=-created_at,name  # 多字段排序

# 搜索：keyword 或 q 参数
GET /api/users?q=张三

# 字段选择：fields 参数
GET /api/users?fields=id,name,email
```

## HTTP 方法语义

| 方法 | 语义 | 幂等 | 安全 | 请求体 | 典型用途 |
|------|------|------|------|--------|----------|
| GET | 读取资源 | 是 | 是 | 无 | 获取列表或详情 |
| POST | 创建资源 | 否 | 否 | 有 | 创建新记录 |
| PUT | 全量替换 | 是 | 否 | 有 | 更新整个资源 |
| PATCH | 部分更新 | 否 | 否 | 有 | 更新资源的部分字段 |
| DELETE | 删除资源 | 是 | 否 | 无 | 删除记录 |
| HEAD | 获取响应头 | 是 | 是 | 无 | 检查资源是否存在 |
| OPTIONS | 获取支持的方法 | 是 | 是 | 无 | CORS 预检请求 |

**幂等性说明**：多次执行同一请求，结果与执行一次相同。
- GET /users/1 多次调用返回相同结果（幂等）
- DELETE /users/1 第一次删除成功，后续调用返回 404（仍算幂等，最终状态一致）
- POST /users 每次调用创建一个新用户（非幂等）

**PUT vs PATCH 区别**：
```json
// PUT：必须提供完整资源（缺少的字段会被置空）
PUT /api/users/1
{"name": "张三", "email": "zhang@example.com", "phone": "13800001234"}

// PATCH：只提供需要修改的字段
PATCH /api/users/1
{"phone": "13900001234"}
```

## 状态码使用

### 成功状态码

| 状态码 | 含义 | 使用场景 |
|--------|------|----------|
| 200 OK | 请求成功 | GET 获取成功、PUT/PATCH 更新成功 |
| 201 Created | 资源已创建 | POST 创建成功（响应头包含 Location） |
| 204 No Content | 成功但无响应体 | DELETE 删除成功 |

### 客户端错误状态码

| 状态码 | 含义 | 使用场景 |
|--------|------|----------|
| 400 Bad Request | 请求参数错误 | 参数缺失、格式错误、校验失败 |
| 401 Unauthorized | 未认证 | 未提供 Token 或 Token 已过期 |
| 403 Forbidden | 无权限 | 已认证但无此操作的权限 |
| 404 Not Found | 资源不存在 | 请求的 ID 不存在 |
| 405 Method Not Allowed | 方法不允许 | 对只读资源发送 DELETE |
| 409 Conflict | 资源冲突 | 唯一键重复（邮箱已注册） |
| 413 Payload Too Large | 请求体过大 | 上传文件超限 |
| 422 Unprocessable Entity | 语义错误 | JSON 格式正确但业务逻辑不允许 |
| 429 Too Many Requests | 请求过频 | 触发限流 |

### 服务器错误状态码

| 状态码 | 含义 | 使用场景 |
|--------|------|----------|
| 500 Internal Server Error | 服务器错误 | 未预期的异常 |
| 502 Bad Gateway | 网关错误 | 上游服务不可用 |
| 503 Service Unavailable | 服务不可用 | 维护中或过载 |
| 504 Gateway Timeout | 网关超时 | 上游服务响应超时 |

### 状态码选择决策树

```
请求是否成功？
├── 是 → 是否创建了新资源？
│   ├── 是 → 201 Created
│   └── 否 → 是否有响应体？
│       ├── 是 → 200 OK
│       └── 否 → 204 No Content
└── 否 → 是否是客户端问题？
    ├── 是 → 是否已认证？
    │   ├── 否 → 401 Unauthorized
    │   └── 是 → 是否有权限？
    │       ├── 否 → 403 Forbidden
    │       └── 是 → 资源是否存在？
    │           ├── 否 → 404 Not Found
    │           └── 是 → 参数是否正确？
    │               ├── 否 → 400 Bad Request
    │               └── 是 → 409/422（业务冲突/语义错误）
    └── 否 → 500 Internal Server Error
```

## 分页参数约定

### 偏移量分页（Offset-based）

```
# 请求
GET /api/users?page=2&per_page=20

# 响应
{
    "data": [...],
    "pagination": {
        "page": 2,
        "per_page": 20,
        "total": 156,
        "total_pages": 8
    }
}
```

参数约定：
| 参数 | 类型 | 默认值 | 约束 |
|------|------|--------|------|
| `page` | int | 1 | >= 1 |
| `per_page` | int | 20 | 1-100 |

### 游标分页（Cursor-based）

适用于大数据集和实时数据流：

```
# 请求（首页）
GET /api/messages?limit=20

# 响应
{
    "data": [...],
    "pagination": {
        "next_cursor": "eyJpZCI6MTAwfQ==",
        "has_more": true
    }
}

# 请求（下一页）
GET /api/messages?cursor=eyJpZCI6MTAwfQ==&limit=20
```

### 分页选择建议

| 场景 | 推荐方式 | 原因 |
|------|----------|------|
| 后台管理列表 | 偏移量分页 | 需要跳页、显示总数 |
| 消息流/时间线 | 游标分页 | 数据实时变化，偏移量不稳定 |
| 数据量 < 10万 | 偏移量分页 | 简单直观，性能可接受 |
| 数据量 > 10万 | 游标分页 | OFFSET 大值时性能差 |

## 错误响应格式

### 统一错误格式

```json
{
    "code": 400,
    "message": "参数校验失败",
    "errors": [
        {
            "field": "email",
            "message": "邮箱格式不正确"
        },
        {
            "field": "password",
            "message": "密码长度不能少于 8 位"
        }
    ]
}
```

### 各场景错误响应示例

```json
// 401 未认证
{
    "code": 401,
    "message": "认证令牌已过期，请重新登录"
}

// 403 无权限
{
    "code": 403,
    "message": "当前角色无此操作权限"
}

// 404 资源不存在
{
    "code": 404,
    "message": "用户不存在"
}

// 409 资源冲突
{
    "code": 409,
    "message": "该邮箱已被注册"
}

// 500 服务器错误（生产环境不暴露细节）
{
    "code": 500,
    "message": "服务器内部错误，请稍后重试"
}
```

### 错误响应设计原则

1. **统一结构**：所有错误响应使用相同的 JSON 结构
2. **面向用户**：message 使用用户可理解的语言，非技术术语
3. **可操作**：告诉用户如何解决问题（"请重新登录"、"请检查邮箱格式"）
4. **安全**：生产环境不暴露堆栈、SQL 语句、文件路径
5. **字段级错误**：表单校验失败时返回每个字段的具体错误

## 版本控制策略

### 三种版本控制方式

| 方式 | 示例 | 优点 | 缺点 |
|------|------|------|------|
| URL 路径 | `/api/v1/users` | 直观，易缓存 | URL 变更，客户端需修改 |
| 请求头 | `Accept: application/vnd.api.v1+json` | URL 不变 | 不直观，调试不便 |
| 查询参数 | `/api/users?version=1` | 简单，可选 | 语义不清，缓存困难 |

推荐使用 URL 路径版本控制：

```
# v1 版本
GET /api/v1/users
POST /api/v1/users

# v2 版本（新增字段或变更行为）
GET /api/v2/users
POST /api/v2/users
```

### 版本迁移策略

| 阶段 | 操作 | 时长建议 |
|------|------|----------|
| 发布 v2 | v1 和 v2 同时可用 | - |
| 弃用通知 | v1 响应头添加 `Deprecation: true` | 至少 3 个月 |
| 停用 v1 | v1 返回 410 Gone + 迁移指引 | - |

### 向后兼容的变更（不需要新版本）

- 添加新的可选参数
- 添加新的响应字段
- 添加新的 API 端点
- 添加新的 HTTP 方法支持

### 需要新版本的变更

- 删除或重命名现有参数
- 修改参数的类型或含义
- 修改响应结构（删除字段、改变嵌套层级）
- 修改 API 路径
- 修改业务语义
