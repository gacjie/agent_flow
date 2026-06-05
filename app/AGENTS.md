# AGENTS.md — AgentFlow 移动端应用

## 项目概况

AgentFlow 移动客户端，基于 UniApp (Vue 3) 开发，用于连接 AgentFlow 后端服务进行 AI 对话交互。

- 框架：UniApp + Vue 3 Composition API
- 状态管理：纯 ref（无 Vuex/Pinia）
- 目标平台：Android APP、H5（iOS 可扩展）
- 开发工具：HBuilderX
- App ID：`__UNI__7A7CC3D`
- H5 开发端口：8090

## 目录结构

```
app/
├── main.js                 # 入口（createSSRApp）
├── App.vue                 # 根组件（全局样式 + 生命周期）
├── manifest.json           # 应用配置（平台/权限/版本）
├── pages.json              # 路由 + 全局导航样式
├── uni.scss                # 全局 SCSS 变量
├── package.json            # 依赖声明
├── index.html              # HTML 入口
├── uni.promisify.adaptor.js  # Promise 适配器
├── pages/
│   ├── index/index.vue     # 服务器列表（首页）
│   ├── login/login.vue     # 登录页
│   ├── workbench/workbench.vue  # 工作台（工作区列表）
│   ├── chat/chat.vue       # 对话页（SSE 流式）
│   └── tasks/tasks.vue     # 任务列表
├── utils/
│   ├── api.js              # API 请求封装 + SSE 实现
│   └── storage.js          # 本地存储管理
└── unpackage/              # 构建产物（自动生成）
```

## 页面路由

| 路径 | 导航栏标题 | 说明 |
|------|-----------|------|
| `pages/index/index` | 服务器列表 | 首页，管理多服务器连接 |
| `pages/login/login` | 登录 | 用户名密码认证 |
| `pages/workbench/workbench` | 工作台 | 工作区列表 + 新建 |
| `pages/chat/chat` | （自定义导航） | 会话列表 + 对话界面 |
| `pages/tasks/tasks` | 任务 | 任务状态展示 |

全局导航样式：白色文字、蓝色背景 `#2b5ce6`、页面背景 `#f5f7fa`

## 核心数据模型

```javascript
// 服务器
{ id: '时间戳', name: '名称', baseURL: 'http://host:port', token: 'Bearer Token', adminName: '管理员名' }

// 工作区
{ id: 数字, name: '标识', label: '显示名', description: '描述', status: 1|2|3, Agent: { title: '智能体名' } }

// 会话
{ id: 数字, title: '标题', total_tokens: 数字, status: 1|2|3 }

// 消息
{ role: 'user'|'assistant', content: '文本', tool_calls: [...] }

// 任务
{ id: 数字, title: '标题', phase: 数字, phase_label: '阶段名', status: 0|1|2|3|4 }
```

状态码含义：
- 工作区/会话 status：1=进行中, 2=已完成, 3=已暂停/已取消
- 任务 status：0=待处理(灰), 1=进行中(蓝), 2=完成(绿), 3=失败(红), 4=跳过(灰)

## API 端点

所有请求通过 `utils/api.js` 统一封装，Bearer Token 认证。

```
POST /api/auth/login              # 登录（无需 Token）
POST /api/auth/logout             # 登出

GET  /admin/workbench/workspaces       # 工作区列表（分页）
GET  /admin/workbench/workspaces-all   # 全部工作区
POST /admin/workbench/workspaces       # 创建工作区

GET  /admin/workbench/conversations?workspace_id=X          # 会话列表
POST /admin/workbench/conversations                         # 创建会话
GET  /admin/workbench/conversations/{id}/messages?workspace_id=X  # 获取消息
POST /admin/workbench/conversations/{id}/send               # 发送消息（SSE 流）
POST /admin/workbench/conversations/{id}/stop?workspace_id=X     # 停止会话

GET  /admin/workbench/tasks?workspace_id=X                  # 任务列表
```

## SSE 流式通信

对话页通过 SSE（Server-Sent Events）实现流式消息接收，核心逻辑在 `utils/api.js` 的 `sendMessageSSE` 函数。

协议格式：
```
data: {"seq":1,"type":"content","content":"你好"}
data: {"seq":2,"type":"content","content":"世界"}
data: {"seq":3,"type":"done"}
data: [DONE]
```

事件类型：
- `content` — 流式文本片段，追加到 streamingText
- `done` / `error` — 结束标记
- `[DONE]` — SSE 流终止信号

多平台实现：
```javascript
// APP 原生环境 — plus.net.XMLHttpRequest
// #ifdef APP-PLUS
const xhr = new plus.net.XMLHttpRequest()
xhr.onreadystatechange = () => {
  if (xhr.readyState >= 3) {
    const chunk = xhr.responseText.slice(offset)
    offset = xhr.responseText.length
    parseSSEChunk(chunk, onEvent, onDone)
  }
}
// #endif

// H5 浏览器环境 — 标准 XMLHttpRequest
// #ifdef H5
const xhr = new XMLHttpRequest()
xhr.onprogress = () => {
  const chunk = xhr.responseText.slice(off)
  off = xhr.responseText.length
  parseSSEChunk(chunk, onEvent, onDone)
}
// #endif
```

重连机制：SSE 支持 `last_seq` 参数，断线重连时从上次序号继续接收。

## 本地存储设计

```javascript
// 存储键
'af_servers'        — 服务器列表 Array
'af_current_server' — 当前选中服务器 Object

// API
getServers() / saveServers(list)
getCurrentServer() / setCurrentServer(obj)
addServer(server)        // 自动生成 id = Date.now().toString()
updateServer(id, updates)  // 合并更新，同步 current
removeServer(id)         // 删除并清理 current
```

## 用户操作流程

```
启动 → 服务器列表（index）
  ├─ 添加服务器（modal 表单）
  └─ 点击服务器
       ├─ 无 Token → 登录页（login）→ 认证成功 → 工作台
       └─ 有 Token → 工作台（workbench）
                      ├─ 查看工作区列表
                      ├─ 新建工作区（modal 表单）
                      └─ 点击工作区 → 对话页（chat）
                           ├─ 会话列表视图
                           │   ├─ 新建对话
                           │   └─ 点击对话 → 对话视图
                           └─ 对话视图
                               ├─ 发送消息（SSE 流式接收回复）
                               ├─ 停止对话
                               └─ 返回 → 会话列表
```

## 样式规范

全局颜色变量（uni.scss）：
```scss
$uni-color-primary: #007aff;   // 主色：蓝
$uni-color-success: #4cd964;   // 成功：绿
$uni-color-warning: #f0ad4e;   // 警告：橙
$uni-color-error: #dd524d;     // 错误：红
```

页面通用样式约定：
- 字体：`-apple-system, 'PingFang SC', sans-serif`
- 背景色：`#efeff4`（iOS 风格浅灰）
- 列表分割线：`::after` 伪元素 + `scaleY(0.5)` 实现 0.5px
- 圆角卡片：`border-radius: 16rpx ~ 20rpx`
- 弹窗模态：fixed 定位遮罩 + 居中 modal-box
- 按钮圆角：`border-radius: 50rpx`（全圆角按钮）或 `10rpx`（卡片内按钮）
- 状态徽标：`font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 20rpx`

## 编码约定

1. 使用 Vue 3 `<script setup>` 语法
2. 生命周期使用 `@dcloudio/uni-app` 提供的 `onLoad`、`onShow`、`onUnload`
3. 页面间传参通过 URL query：`uni.navigateTo({ url: '/pages/xx?key=value' })`
4. 表单输入统一使用 `:value` + `@input` 双向绑定（非 v-model）
5. 异步请求统一 `async/await` + `.catch()` 兜底
6. 条件编译：`// #ifdef APP-PLUS` / `// #ifdef H5` 区分平台代码
7. 样式使用 `rpx` 响应式单位，`scoped` 隔离

## 可复用组件模式

### 列表项组件模式
```vue
<view class="list-block">
  <view v-for="item in list" :key="item.id" class="list-item" @click="onTap(item)">
    <view class="item-body">
      <text class="item-name">{{ item.name }}</text>
      <text class="item-desc">{{ item.desc }}</text>
    </view>
    <text class="arrow">›</text>
  </view>
</view>
```

### 弹窗模态框模式
```vue
<view v-if="showModal" class="modal-wrap">
  <view class="modal-mask" @click="showModal = false"></view>
  <view class="modal-box">
    <text class="modal-title">标题</text>
    <view class="form-field">
      <input :value="form.field" @input="form.field = $event.detail.value"
             placeholder="提示" class="field-input" cursor-spacing="20" />
    </view>
    <view class="modal-btns">
      <view class="btn-cancel" @click="showModal = false">取消</view>
      <view class="btn-primary" @click="submit">确定</view>
    </view>
  </view>
</view>
```

### 空状态模式
```vue
<view v-if="loading" class="empty"><text>加载中...</text></view>
<view v-else-if="list.length === 0" class="empty"><text>暂无数据</text></view>
```

## 扩展开发参考

### 表单验证工具（推荐引入）

```javascript
// utils/validator.js — 表单验证器
const rules = {
  string: (v, p) => typeof v === 'string' && v.length >= (p.min || 0) && v.length <= (p.max || 9999),
  notnull: (v) => v !== null && v !== undefined && v !== '',
  email: (v) => /^[\w.-]+@[\w.-]+\.\w+$/.test(v),
  phoneno: (v) => /^1[3-9]\d{9}$/.test(v),
  between: (v, p) => Number(v) >= p.min && Number(v) <= p.max,
  same: (v, p) => v === p.target,
  regex: (v, p) => new RegExp(p.pattern).test(v),
}

export function validate(data, ruleList) {
  for (const r of ruleList) {
    const val = data[r.name]
    const checker = rules[r.type || 'notnull']
    if (!checker || !checker(val, r)) {
      return { ok: false, msg: r.msg || `${r.name} 校验失败` }
    }
  }
  return { ok: true }
}

// 使用示例
const result = validate(form, [
  { name: 'username', type: 'notnull', msg: '请输入用户名' },
  { name: 'password', type: 'string', min: 6, msg: '密码至少6位' },
  { name: 'email', type: 'email', msg: '邮箱格式不正确' },
])
if (!result.ok) uni.showToast({ title: result.msg, icon: 'none' })
```

### 权限请求封装（APP 平台）

```javascript
// utils/permission.js — 平台权限处理
export function requestPermission(permissionName) {
  // #ifdef APP-PLUS
  return new Promise((resolve, reject) => {
    if (uni.getSystemInfoSync().platform === 'ios') {
      // iOS 通过 info.plist 声明，运行时自动弹窗
      resolve(true)
    } else {
      // Android 动态权限申请
      plus.android.requestPermissions(
        [`android.permission.${permissionName}`],
        (e) => {
          if (e.granted.length > 0) resolve(true)
          else reject(new Error('权限被拒绝'))
        },
        (e) => reject(e)
      )
    }
  })
  // #endif
  // #ifdef H5
  return Promise.resolve(true)
  // #endif
}

// 打开系统设置（权限被永久拒绝时引导用户手动开启）
export function gotoSettings() {
  // #ifdef APP-PLUS
  if (uni.getSystemInfoSync().platform === 'ios') {
    plus.runtime.openURL('app-settings:')
  } else {
    const Intent = plus.android.importClass('android.content.Intent')
    const Settings = plus.android.importClass('android.provider.Settings')
    const Uri = plus.android.importClass('android.net.Uri')
    const main = plus.android.runtimeMainActivity()
    const intent = new Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS)
    intent.setData(Uri.fromParts('package', main.getPackageName(), null))
    main.startActivity(intent)
  }
  // #endif
}
```

### 时间/数据格式化工具

```javascript
// utils/format.js
export function formatTime(seconds) {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  return [h, m, s].map(v => String(v).padStart(2, '0')).join(':')
}

export function timeAgo(dateStr) {
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return '刚刚'
  if (mins < 60) return `${mins}分钟前`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}小时前`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days}天前`
  return new Date(dateStr).toLocaleDateString()
}

export function truncate(str, len = 50) {
  return str && str.length > len ? str.slice(0, len) + '...' : str
}
```

### Pinia 状态管理（适合扩展后引入）

```javascript
// store/user.js — 用户状态（替代当前的 storage 直接操作）
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useUserStore = defineStore('user', () => {
  const servers = ref([])
  const currentServer = ref(null)
  const isLoggedIn = computed(() => !!currentServer.value?.token)

  function loadFromStorage() {
    servers.value = uni.getStorageSync('af_servers') || []
    currentServer.value = uni.getStorageSync('af_current_server') || null
  }

  function addServer(server) {
    server.id = Date.now().toString()
    servers.value.push(server)
    uni.setStorageSync('af_servers', servers.value)
  }

  function setToken(serverId, token, adminName) {
    const s = servers.value.find(s => s.id === serverId)
    if (s) { s.token = token; s.adminName = adminName }
    uni.setStorageSync('af_servers', servers.value)
    currentServer.value = s
    uni.setStorageSync('af_current_server', s)
  }

  function logout() {
    if (currentServer.value) currentServer.value.token = null
    uni.setStorageSync('af_servers', servers.value)
    uni.setStorageSync('af_current_server', currentServer.value)
  }

  return { servers, currentServer, isLoggedIn, loadFromStorage, addServer, setToken, logout }
})
```

### 网络请求拦截器模式

```javascript
// utils/http.js — 统一请求拦截（Token 过期自动跳登录）
function request(options) {
  const server = getCurrentServer()
  if (!server) return Promise.reject(new Error('未选择服务器'))

  return new Promise((resolve, reject) => {
    uni.request({
      url: server.baseURL + options.path,
      method: options.method || 'GET',
      data: options.data,
      header: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${server.token}`
      },
      success: (res) => {
        if (res.statusCode === 401) {
          // Token 过期，清除登录态并跳转
          server.token = null
          setCurrentServer(server)
          uni.redirectTo({ url: '/pages/login/login?serverId=' + server.id })
          reject(new Error('登录已过期'))
          return
        }
        if (res.statusCode >= 400) {
          reject(res.data)
          return
        }
        resolve(res.data)
      },
      fail: (err) => reject(err)
    })
  })
}
```

### 下拉刷新 + 上拉加载模式

```vue
<script setup>
import { ref } from 'vue'
import { onPullDownRefresh, onReachBottom } from '@dcloudio/uni-app'

const list = ref([])
const page = ref(1)
const hasMore = ref(true)
const loading = ref(false)

async function loadData(reset = false) {
  if (loading.value) return
  if (reset) { page.value = 1; hasMore.value = true }
  if (!hasMore.value) return
  loading.value = true
  try {
    const res = await request('GET', `/api/items?page=${page.value}&size=20`)
    const items = res.data || []
    if (reset) list.value = items
    else list.value.push(...items)
    hasMore.value = items.length >= 20
    page.value++
  } finally {
    loading.value = false
  }
}

onPullDownRefresh(async () => {
  await loadData(true)
  uni.stopPullDownRefresh()
})

onReachBottom(() => { loadData() })
</script>
```

对应 pages.json 需要开启下拉刷新：
```json
{ "path": "pages/list/list", "style": { "enablePullDownRefresh": true } }
```

### 自定义导航栏模式

```vue
<!-- pages.json 中设置 "navigationStyle": "custom" -->
<template>
  <view class="page">
    <view class="nav-bar" :style="{ paddingTop: statusBarHeight + 'px' }">
      <text class="nav-back" @click="goBack">‹</text>
      <text class="nav-title">{{ title }}</text>
      <text class="nav-action" @click="onAction">操作</text>
    </view>
    <view class="content" :style="{ marginTop: navHeight + 'px' }">
      <!-- 页面内容 -->
    </view>
  </view>
</template>

<script setup>
const sysInfo = uni.getSystemInfoSync()
const statusBarHeight = sysInfo.statusBarHeight
const navHeight = statusBarHeight + 44  // 44px 导航栏高度
</script>

<style scoped>
.nav-bar {
  position: fixed; top: 0; left: 0; right: 0; z-index: 100;
  display: flex; align-items: center; height: 44px;
  padding: 0 20rpx; background: #007aff;
}
</style>
```

## 待开发功能（参考方向）

- [ ] 会话内 Markdown 渲染（当前纯文本显示）
- [ ] 工具调用详情展开/折叠
- [ ] 任务状态实时更新（WebSocket 或轮询）
- [ ] 暗色主题支持
- [ ] 消息长按复制/分享
- [ ] 网络状态检测 + 离线提示
- [ ] 图片/文件消息支持
- [ ] 工作区搜索/筛选
- [ ] 推送通知（任务完成/消息提醒）
- [ ] 多语言支持
