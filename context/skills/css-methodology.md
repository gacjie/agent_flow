---
name: css-methodology
label: CSS 方法论
description: CSS 方法论速查，涵盖 BEM 命名、CSS 变量分层、响应式断点、双主题实现、选择器优先级和常见布局模式
keywords: CSS,BEM,变量,响应式,主题,Flexbox,Grid,选择器,优先级,布局,断点
level: 1
status: 1
---

# CSS 方法论速查

## BEM 命名规范

### BEM 基本概念

BEM = Block（块）+ Element（元素）+ Modifier（修饰符）

```
.block                    -- 独立的功能组件
.block__element           -- 块的组成部分，离开块没有独立意义
.block--modifier          -- 块的外观/状态变体
.block__element--modifier -- 元素的外观/状态变体
```

### 命名规则

| 规则 | 说明 | 示例 |
|------|------|------|
| Block 用单词或连字符 | 描述组件的用途 | `.search-form`、`.nav-bar` |
| Element 用双下划线 | 连接块和元素 | `.search-form__input` |
| Modifier 用双连字符 | 连接块/元素和修饰符 | `.search-form--large` |
| 不嵌套 Element | 元素名不反映 DOM 层级 | `.card__title` 非 `.card__header__title` |

### 完整示例

```html
<!-- Block -->
<form class="search-form search-form--dark">
    <!-- Element -->
    <label class="search-form__label">搜索</label>
    <input class="search-form__input search-form__input--disabled" disabled>
    <button class="search-form__button search-form__button--primary">
        搜索
    </button>
</form>
```

```css
/* Block */
.search-form {
    display: flex;
    align-items: center;
    gap: 8px;
}

/* Modifier（块级） */
.search-form--dark {
    background: #1a1a1a;
}

/* Element */
.search-form__input {
    padding: 8px 12px;
    border: 1px solid var(--border);
    border-radius: 4px;
}

/* Modifier（元素级） */
.search-form__input--disabled {
    opacity: 0.5;
    cursor: not-allowed;
}

.search-form__button {
    padding: 8px 16px;
    border: none;
    border-radius: 4px;
    cursor: pointer;
}

.search-form__button--primary {
    background: var(--color-primary);
    color: white;
}
```

### BEM 常见错误

| 错误 | 正确 |
|------|------|
| `.card .title` | `.card__title` |
| `.card__header__title` | `.card__title` |
| `.card__title.active` | `.card__title--active` |
| `.card-title` | `.card__title`（如果 title 属于 card） |

## CSS 变量分层体系

### 三层变量架构

```css
/* 第 1 层：基础色板（设计令牌）*/
:root {
    /* 颜色原始值 */
    --blue-50: #eff6ff;
    --blue-500: #3b82f6;
    --blue-600: #2563eb;
    --blue-700: #1d4ed8;
    --gray-50: #f9fafb;
    --gray-100: #f3f4f6;
    --gray-200: #e5e7eb;
    --gray-700: #374151;
    --gray-800: #1f2937;
    --gray-900: #111827;
    --red-500: #ef4444;
    --green-500: #22c55e;

    /* 间距基准 */
    --spacing-unit: 4px;

    /* 字号基准 */
    --font-size-sm: 0.875rem;
    --font-size-base: 1rem;
    --font-size-lg: 1.125rem;
    --font-size-xl: 1.25rem;

    /* 圆角 */
    --radius-sm: 4px;
    --radius-md: 8px;
    --radius-lg: 12px;
}

/* 第 2 层：语义变量（引用基础色板）*/
[data-theme="light"] {
    --bg-base: var(--gray-50);
    --bg-surface: #ffffff;
    --bg-hover: var(--gray-100);
    --text-primary: var(--gray-900);
    --text-secondary: var(--gray-700);
    --border: var(--gray-200);
    --color-primary: var(--blue-600);
    --color-primary-hover: var(--blue-700);
    --color-danger: var(--red-500);
    --color-success: var(--green-500);
    --shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

[data-theme="dark"] {
    --bg-base: var(--gray-900);
    --bg-surface: var(--gray-800);
    --bg-hover: var(--gray-700);
    --text-primary: var(--gray-50);
    --text-secondary: var(--gray-200);
    --border: var(--gray-700);
    --color-primary: var(--blue-500);
    --color-primary-hover: var(--blue-600);
    --color-danger: var(--red-500);
    --color-success: var(--green-500);
    --shadow: 0 1px 3px rgba(0, 0, 0, 0.4);
}

/* 第 3 层：组件变量（引用语义变量）*/
.card {
    --card-bg: var(--bg-surface);
    --card-border: var(--border);
    --card-shadow: var(--shadow);
    --card-padding: calc(var(--spacing-unit) * 4);

    background: var(--card-bg);
    border: 1px solid var(--card-border);
    box-shadow: var(--card-shadow);
    padding: var(--card-padding);
}
```

### 变量命名约定

| 前缀 | 含义 | 示例 |
|------|------|------|
| `--bg-` | 背景色 | `--bg-base`、`--bg-surface`、`--bg-hover` |
| `--text-` | 文字色 | `--text-primary`、`--text-secondary` |
| `--color-` | 主题色 | `--color-primary`、`--color-danger` |
| `--border` | 边框 | `--border`、`--border-hover` |
| `--shadow` | 阴影 | `--shadow`、`--shadow-lg` |
| `--radius-` | 圆角 | `--radius-sm`、`--radius-md` |
| `--font-size-` | 字号 | `--font-size-sm`、`--font-size-base` |

## 响应式断点策略

### 推荐断点

| 断点名称 | 宽度 | 目标设备 |
|----------|------|----------|
| `--bp-sm` | 640px | 手机横屏 |
| `--bp-md` | 768px | 平板竖屏 |
| `--bp-lg` | 1024px | 平板横屏/小笔记本 |
| `--bp-xl` | 1280px | 桌面 |
| `--bp-2xl` | 1536px | 大屏桌面 |

### Mobile-First 写法

```css
/* 基础样式（手机优先） */
.container {
    padding: 16px;
    width: 100%;
}

.grid {
    display: grid;
    grid-template-columns: 1fr;
    gap: 16px;
}

/* 平板以上 */
@media (min-width: 768px) {
    .container {
        padding: 24px;
        max-width: 768px;
        margin: 0 auto;
    }

    .grid {
        grid-template-columns: repeat(2, 1fr);
    }
}

/* 桌面以上 */
@media (min-width: 1024px) {
    .container {
        max-width: 1024px;
    }

    .grid {
        grid-template-columns: repeat(3, 1fr);
        gap: 24px;
    }
}

/* 大屏桌面 */
@media (min-width: 1280px) {
    .container {
        max-width: 1200px;
    }

    .grid {
        grid-template-columns: repeat(4, 1fr);
    }
}
```

### 媒体查询组织原则

1. 按组件组织（每个组件的媒体查询紧跟在基础样式后面），而非按断点集中放置
2. 使用 `min-width`（Mobile-First），避免混用 `min-width` 和 `max-width`
3. 断点值使用变量或统一定义，避免散落在各处

## 双主题实现模式

### 基于 data-theme 属性

```html
<html data-theme="light">
```

```css
/* 语义变量定义（见上方变量分层体系） */

/* 组件直接使用语义变量 */
body {
    background: var(--bg-base);
    color: var(--text-primary);
}

.btn-primary {
    background: var(--color-primary);
    color: white;
    border: none;
}

.btn-primary:hover {
    background: var(--color-primary-hover);
}
```

### JavaScript 主题切换

```javascript
// 切换主题
function toggleTheme() {
    const html = document.documentElement;
    const current = html.getAttribute("data-theme");
    const next = current === "light" ? "dark" : "light";
    html.setAttribute("data-theme", next);
    localStorage.setItem("theme", next);
}

// 页面加载时恢复主题
function initTheme() {
    const saved = localStorage.getItem("theme");
    const preferred = window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark" : "light";
    document.documentElement.setAttribute("data-theme", saved || preferred);
}

// 监听系统主题变化
window.matchMedia("(prefers-color-scheme: dark)")
    .addEventListener("change", (e) => {
        if (!localStorage.getItem("theme")) {
            document.documentElement.setAttribute(
                "data-theme", e.matches ? "dark" : "light"
            );
        }
    });
```

### 主题切换注意事项

| 事项 | 说明 |
|------|------|
| 避免闪烁 | 主题初始化脚本放在 `<head>` 中，阻塞渲染前设置 |
| 图片适配 | 使用 `<picture>` + `prefers-color-scheme` 媒体查询 |
| 阴影适配 | 暗色主题需要更深的阴影 |
| 颜色对比度 | 暗色主题确保文字和背景对比度 >= 4.5:1（WCAG AA） |
| 过渡动画 | 添加 `transition: background-color 0.2s, color 0.2s` |

## 选择器优先级规则

### 优先级计算

| 选择器 | (a, b, c) | 示例 |
|--------|-----------|------|
| 内联样式 | (1, 0, 0) | `style="color: red"` |
| ID 选择器 | (0, 1, 0) | `#header` |
| 类/属性/伪类 | (0, 0, 1) | `.active`、`[type="text"]`、`:hover` |
| 元素/伪元素 | (0, 0, 0) + 1 | `div`、`::before` |
| 通配符 | (0, 0, 0) | `*` |

计算规则：a > b > c，同级别相加比较。

### 优先级示例

```css
/* 优先级 (0, 0, 1) -- 1 个类 */
.card { color: blue; }

/* 优先级 (0, 0, 2) -- 2 个类 */
.card.active { color: red; }

/* 优先级 (0, 1, 0) -- 1 个 ID */
#main { color: green; }

/* 优先级 (0, 1, 1) -- 1 个 ID + 1 个类 */
#main .card { color: purple; }

/* 优先级 (0, 0, 2) -- 1 个类 + 1 个伪类 */
.card:hover { color: orange; }
```

### 优先级最佳实践

1. 避免使用 ID 选择器作为样式钩子（优先级过高难以覆盖）
2. 避免使用 `!important`（破坏优先级层叠）
3. 保持选择器简短（最多 3 层嵌套）
4. 使用 BEM 命名保持选择器扁平化
5. 使用 CSS 变量代替 `!important` 实现主题覆盖

## 常见布局模式

### Flexbox 常用布局

```css
/* 水平居中 */
.center-horizontal {
    display: flex;
    justify-content: center;
}

/* 垂直居中 */
.center-vertical {
    display: flex;
    align-items: center;
}

/* 水平+垂直居中 */
.center-both {
    display: flex;
    justify-content: center;
    align-items: center;
}

/* 两端对齐（左右分布） */
.space-between {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

/* 导航栏布局 */
.navbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0 24px;
    height: 60px;
}

/* 底部固定（Sticky Footer） */
.page {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
}
.page__content {
    flex: 1;  /* 占据剩余空间 */
}
.page__footer {
    /* 自动贴底 */
}

/* 等高列 */
.equal-height-row {
    display: flex;
    gap: 16px;
}
.equal-height-row > * {
    flex: 1;  /* 等宽等高 */
}
```

### Grid 常用布局

```css
/* 等宽多列网格 */
.grid-auto {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
    gap: 16px;
}

/* 固定列数网格 */
.grid-3col {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 16px;
}

/* 侧边栏布局 */
.layout-sidebar {
    display: grid;
    grid-template-columns: 250px 1fr;
    gap: 24px;
    min-height: 100vh;
}

/* 双侧边栏布局 */
.layout-double-sidebar {
    display: grid;
    grid-template-columns: 200px 1fr 300px;
    gap: 24px;
}

/* 仪表盘布局（不等宽） */
.dashboard {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    grid-template-rows: auto 1fr;
    gap: 16px;
}
.dashboard__header {
    grid-column: 1 / -1;  /* 横跨所有列 */
}
.dashboard__chart {
    grid-column: span 2;  /* 占 2 列 */
}
.dashboard__sidebar {
    grid-column: span 1;
    grid-row: span 2;    /* 占 2 行 */
}

/* 表单布局（标签 + 输入框） */
.form-grid {
    display: grid;
    grid-template-columns: 120px 1fr;
    gap: 12px 16px;
    align-items: center;
}
```

### Flexbox vs Grid 选择

| 场景 | 推荐 | 原因 |
|------|------|------|
| 一维布局（一行或一列） | Flexbox | 更简单直观 |
| 二维布局（行+列） | Grid | 同时控制行列 |
| 内容驱动布局 | Flexbox | 元素大小由内容决定 |
| 布局驱动内容 | Grid | 先定义网格再放入内容 |
| 导航栏/工具栏 | Flexbox | 水平排列+对齐 |
| 页面整体结构 | Grid | 侧边栏+内容区 |
| 卡片列表 | Grid | 等宽等高自动换行 |
| 垂直居中 | 两者均可 | Flexbox 稍简洁 |
