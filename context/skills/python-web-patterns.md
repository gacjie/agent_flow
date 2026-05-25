---
name: python-web-patterns
label: Python Web 开发模式
description: Python Web 开发常用模式速查，涵盖路由定义、ORM CRUD、表单验证、认证中间件、错误处理、分页查询和文件上传
keywords: Python,Web,Flask,FastAPI,Django,路由,ORM,CRUD,验证,中间件,分页,文件上传
level: 1
status: 1
---

# Python Web 开发常用模式速查

## 路由定义模式

### Flask 风格

```python
from flask import Flask, request, jsonify

app = Flask(__name__)

# 基础路由
@app.route("/api/users", methods=["GET"])
def list_users():
    page = request.args.get("page", 1, type=int)
    return jsonify({"users": [], "page": page})

# 路径参数
@app.route("/api/users/<int:user_id>", methods=["GET"])
def get_user(user_id):
    return jsonify({"id": user_id})

# 蓝图分组
from flask import Blueprint
api = Blueprint("api", __name__, url_prefix="/api")

@api.route("/items", methods=["POST"])
def create_item():
    data = request.get_json()
    return jsonify(data), 201
```

### FastAPI 风格

```python
from fastapi import FastAPI, Path, Query, HTTPException

app = FastAPI()

# 类型注解自动校验
@app.get("/api/users/{user_id}")
async def get_user(
    user_id: int = Path(..., ge=1, description="用户ID"),
    fields: str = Query(None, description="返回字段")
):
    return {"id": user_id}

# 请求体模型
from pydantic import BaseModel

class UserCreate(BaseModel):
    username: str
    email: str
    password: str

@app.post("/api/users", status_code=201)
async def create_user(user: UserCreate):
    return {"id": 1, "username": user.username}
```

## ORM CRUD 模式

### SQLAlchemy 基础 CRUD

```python
from sqlalchemy.orm import Session

# Create
def create_user(db: Session, username: str, email: str) -> User:
    user = User(username=username, email=email)
    db.add(user)
    db.commit()
    db.refresh(user)
    return user

# Read（单条 + 列表）
def get_user(db: Session, user_id: int) -> User | None:
    return db.query(User).filter(User.id == user_id).first()

def list_users(db: Session, skip: int = 0, limit: int = 20) -> list[User]:
    return db.query(User).offset(skip).limit(limit).all()

# Update
def update_user(db: Session, user_id: int, **kwargs) -> User | None:
    user = db.query(User).filter(User.id == user_id).first()
    if not user:
        return None
    for key, value in kwargs.items():
        setattr(user, key, value)
    db.commit()
    db.refresh(user)
    return user

# Delete（软删除）
def delete_user(db: Session, user_id: int) -> bool:
    user = db.query(User).filter(User.id == user_id).first()
    if not user:
        return False
    user.deleted_at = datetime.utcnow()
    db.commit()
    return True
```

### 防 N+1 查询

```python
# 错误：循环内查询（N+1）
users = db.query(User).all()
for user in users:
    orders = db.query(Order).filter(Order.user_id == user.id).all()  # N 次查询

# 正确：预加载关联
from sqlalchemy.orm import joinedload
users = db.query(User).options(joinedload(User.orders)).all()  # 1 次查询
```

## 表单验证模式

### Pydantic 验证

```python
from pydantic import BaseModel, validator, Field
import re

class UserCreate(BaseModel):
    username: str = Field(..., min_length=3, max_length=50)
    email: str = Field(..., max_length=200)
    password: str = Field(..., min_length=8)

    @validator("email")
    def validate_email(cls, v):
        pattern = r"^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$"
        if not re.match(pattern, v):
            raise ValueError("邮箱格式不正确")
        return v

    @validator("password")
    def validate_password(cls, v):
        if not re.search(r"[A-Z]", v):
            raise ValueError("密码必须包含大写字母")
        if not re.search(r"[a-z]", v):
            raise ValueError("密码必须包含小写字母")
        if not re.search(r"\d", v):
            raise ValueError("密码必须包含数字")
        return v
```

### WTForms 验证（Flask）

```python
from wtforms import Form, StringField, PasswordField, validators

class LoginForm(Form):
    username = StringField("用户名", [
        validators.DataRequired(message="用户名不能为空"),
        validators.Length(min=3, max=50, message="用户名长度 3-50 字符")
    ])
    password = PasswordField("密码", [
        validators.DataRequired(message="密码不能为空"),
        validators.Length(min=8, message="密码至少 8 位")
    ])
```

## 认证中间件模式

### JWT 认证中间件

```python
from functools import wraps
from flask import request, g, jsonify
import jwt

def auth_required(f):
    @wraps(f)
    def decorated(*args, **kwargs):
        token = request.headers.get("Authorization", "").replace("Bearer ", "")
        if not token:
            return jsonify({"error": "缺少认证令牌"}), 401
        try:
            payload = jwt.decode(token, SECRET_KEY, algorithms=["HS256"])
            g.current_user_id = payload["user_id"]
        except jwt.ExpiredSignatureError:
            return jsonify({"error": "令牌已过期"}), 401
        except jwt.InvalidTokenError:
            return jsonify({"error": "无效令牌"}), 401
        return f(*args, **kwargs)
    return decorated

# 权限检查装饰器
def require_permission(permission: str):
    def decorator(f):
        @wraps(f)
        def decorated(*args, **kwargs):
            if permission not in g.permissions:
                return jsonify({"error": "权限不足"}), 403
            return f(*args, **kwargs)
        return decorated
    return decorator
```

### FastAPI 依赖注入认证

```python
from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPBearer

security = HTTPBearer()

async def get_current_user(credentials = Depends(security)):
    try:
        payload = jwt.decode(credentials.credentials, SECRET_KEY, algorithms=["HS256"])
        user = await get_user(payload["user_id"])
        if not user:
            raise HTTPException(status_code=401, detail="用户不存在")
        return user
    except jwt.InvalidTokenError:
        raise HTTPException(status_code=401, detail="无效令牌")

@app.get("/api/profile")
async def profile(user = Depends(get_current_user)):
    return {"username": user.username}
```

## 错误处理模式

### 统一异常处理

```python
# 自定义异常类
class AppError(Exception):
    def __init__(self, status_code: int, message: str, detail: str = None):
        self.status_code = status_code
        self.message = message
        self.detail = detail

class NotFoundError(AppError):
    def __init__(self, resource: str, id: int):
        super().__init__(404, f"{resource} 不存在", f"ID={id}")

class ValidationError(AppError):
    def __init__(self, message: str):
        super().__init__(400, message)

# Flask 全局异常处理器
@app.errorhandler(AppError)
def handle_app_error(error):
    return jsonify({
        "code": error.status_code,
        "message": error.message,
        "detail": error.detail
    }), error.status_code

@app.errorhandler(500)
def handle_internal_error(error):
    # 生产环境不暴露堆栈
    return jsonify({"code": 500, "message": "服务器内部错误"}), 500
```

## 分页查询模式

```python
# 通用分页函数
def paginate(query, page: int = 1, per_page: int = 20) -> dict:
    page = max(1, page)
    per_page = min(max(1, per_page), 100)  # 限制最大每页数
    total = query.count()
    items = query.offset((page - 1) * per_page).limit(per_page).all()
    return {
        "items": items,
        "total": total,
        "page": page,
        "per_page": per_page,
        "total_pages": (total + per_page - 1) // per_page
    }

# 使用
@app.get("/api/users")
def list_users():
    page = request.args.get("page", 1, type=int)
    per_page = request.args.get("per_page", 20, type=int)
    query = db.query(User).filter(User.deleted_at.is_(None)).order_by(User.id.desc())
    return jsonify(paginate(query, page, per_page))
```

## 文件上传模式

```python
import os
import uuid
from werkzeug.utils import secure_filename

ALLOWED_EXTENSIONS = {"png", "jpg", "jpeg", "gif", "pdf"}
MAX_FILE_SIZE = 10 * 1024 * 1024  # 10MB

def allowed_file(filename: str) -> bool:
    return "." in filename and filename.rsplit(".", 1)[1].lower() in ALLOWED_EXTENSIONS

@app.post("/api/upload")
def upload_file():
    if "file" not in request.files:
        return jsonify({"error": "未选择文件"}), 400

    file = request.files["file"]
    if file.filename == "":
        return jsonify({"error": "文件名为空"}), 400

    if not allowed_file(file.filename):
        return jsonify({"error": f"不支持的文件类型，允许：{ALLOWED_EXTENSIONS}"}), 400

    # 检查文件大小
    file.seek(0, os.SEEK_END)
    size = file.tell()
    file.seek(0)
    if size > MAX_FILE_SIZE:
        return jsonify({"error": "文件超过 10MB 限制"}), 400

    # 安全文件名 + UUID 防冲突
    ext = file.filename.rsplit(".", 1)[1].lower()
    filename = f"{uuid.uuid4().hex}.{ext}"
    filepath = os.path.join(UPLOAD_DIR, filename)
    file.save(filepath)

    return jsonify({"filename": filename, "size": size}), 201
```
