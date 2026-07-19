---
name: frontend-performance
label: 前端性能优化技能
description: 前端性能指标(Core Web Vitals 2024)、优化模式和内存管理清单，供设计师规划前端性能要求
keywords: 性能,LCP,INP,CLS,懒加载,虚拟滚动,内存泄漏,关键渲染路径,代码分割
level: 2
status: 1
sort: 21
---

# 前端性能优化技能

> 前端性能指标、优化模式和内存管理清单。设计师读取后按项目裁剪产出前端性能约定——过滤项目不涉及的性能面，将适用项转化为具体方案。开发者按需读取识别前端性能陷阱。

## 一、性能风险触发项表

| 触发领域 | 具体触发项 |
|---------|-----------|
| 渲染性能 | DOM 节点过多(>1500)、频繁重排/重绘、长任务阻塞(>50ms)、无 CSS containment |
| 网络加载 | 首屏依赖 JS 动态加载资源、无代码分割、未压缩大文件、渲染阻塞 CSS/JS |
| 内存管理 | 事件监听器未清理、定时器未清除、分离 DOM 引用、闭包保持大对象、无界缓存 |
| 交互响应 | 输入无防抖/节流、计算密集在主线程、同步 DOM 读写交错 |
| 大列表/数据 | >200 项列表无虚拟化、图片未懒加载、全量数据一次加载 |

## 二、Core Web Vitals 2024-2026

### 2024 关键变化

INP（Interaction to Next Paint）于 2024.3.12 正式替代 FID 成为核心指标。约 600,000 个网站从通过变为未通过。

### 2026 状态

阈值未变，但 INP 测量方法论收紧。2026.5 全球通过率：LCP 68.6%、CLS 81.3%、INP 86.6%，三项全过仅 55.9%。

### 指标阈值

| 指标 | 良好 | 需改善 | 差 | 含义 |
|------|------|--------|---|------|
| LCP | <2.5s | 2.5-4s | >4s | 最大可见元素加载时间 |
| INP | <200ms | 200-500ms | >500ms | 所有交互响应延迟（替代 FID） |
| CLS | <0.1 | 0.1-0.25 | >0.25 | 布局意外移动程度 |
| TTFB | <200ms | 200-500ms | >500ms | 诊断指标（2026 升级关注） |

### LCP 优化

- `fetchpriority="high"` 提升 LCP 图片优先级
- 确保 LCP 图片 URL 在 HTML 可发现（避免 JS 动态加载，35% 的 LCP 图片不可发现）
- 使用 WebP/AVIF 现代格式（AVIF 比 JPEG 小 50%）
- `<link rel="preload">` 预加载关键资源
- SSR/SSG 首屏内容（避免客户端渲染延迟）

### INP 优化

- 减少 DOM 节点数量（DOM 过大是 INP 差的首要原因）
- 拆分长任务（>50ms），使用 `scheduler.yield()` / `requestIdleCallback`
- 对输入事件防抖/节流
- 使用 Long Animation Frames (LoAF) API 诊断慢帧
- 优先处理 UI 线程工作，非关键计算延后

### CLS 优化

- 图片/视频指定明确宽高（73% 移动页 LCP 元素是图片）
- 预留广告/动态内容空间
- 避免在已有内容上方插入元素
- CSS `contain` 属性限制重排范围

## 三、关键渲染路径

### 渲染流程

```
HTML → DOM / CSS → CSSOM / JS 执行(可能阻塞) → Render Tree → Layout → Paint
```

**关键问题**：CSS 是渲染阻塞资源——浏览器处理完所有 CSS 之前不渲染任何内容。

### 优化策略

- **内联关键 CSS**：提取首屏所需最小 CSS 集合内联到 `<style>` 中
- **移除未用 CSS**：所有样式都会被解析，即使未被使用
- **非首屏 CSS 延迟加载**：`<link media="print" onload="this.media='all'">`
- **Media 查询分离**：`<link media="print">` 不阻塞渲染
- **预加载关键资源**：`<link rel="preload" as="style">`

### 核心三策略

1. 最小化关键资源数量（延迟/异步/消除非关键资源）
2. 减少关键路径长度（减少请求数）
3. 减少关键字节数（压缩文件体积）

## 四、JavaScript 内存管理

### 六大泄漏源

| 泄漏源 | 触发条件 | 防护 |
|--------|---------|------|
| 全局变量 | 大数据存全局不清理 | let/const 声明 + 及时释放 |
| 事件监听器 | DOM 移除但监听未清理 | removeEventListener 配合 DOM 移除 |
| 定时器 | setInterval 未清除 | clearInterval / clearTimeout |
| 闭包 | 保持对父作用域大对象引用 | 解除引用 / WeakRef |
| 分离 DOM | DOM 节点移除但 JS 仍引用 | 移除后清除变量引用 |
| 无界缓存 | 无限存 API 响应/计算值 | LRU 缓存 / 容量上限 |

### 框架清理模式

- **React**：useEffect 返回清理函数（取消订阅/移除监听/清除定时器）
- **Vue**：onUnmounted 钩子清理副作用
- **通用**：Web Workers 卸载计算密集任务到独立线程
- **检测**：Chrome DevTools Memory Profiler 堆快照 + Allocation Timeline

## 五、大列表与虚拟滚动

### 虚拟滚动原理

只渲染可视区域 + 缓冲区的元素（10000 项只渲染约 30 个 DOM 节点）。通过 scrollTop 计算可见索引，padding 元素模拟完整高度。内存恒定，性能不受列表长度影响。

### 适用判断

- >200 项且滚动卡顿 → 使用虚拟滚动
- <100 项 → 不需要（增加不必要复杂度）

### 注意事项

- Ctrl+F 搜索失效（未渲染的项无法被浏览器搜索）
- 可访问性需额外处理（屏幕阅读器支持）
- SEO 不可索引隐藏内容

### 三种方案对比

| 方案 | 特点 | 适用场景 |
|------|------|---------|
| 分页 | 可预测、可书签、页码导航 | 搜索结果、管理后台列表 |
| 无限滚动 | 滚动触发加载、沉浸式 | 社交媒体信息流 |
| 虚拟滚动 | 只渲染可见项、大数据量 | 大数据表格、日志查看器 |

### 库推荐

- React：react-window / TanStack Virtual
- Vue：vue-virtual-scroller
- Angular：CDK Virtual Scroll（v7+）

## 六、2025-2026 前端新能力

| 能力 | 说明 |
|------|------|
| Speculation Rules API | 浏览器级预渲染，实现即时导航（取代 prefetch） |
| View Transitions API | 原生页面动画零 JavaScript，跨页面过渡 |
| React Compiler v1.0 | 2025.12 发布，自动 memoization，减少 30-40% 不必要重渲染 |
| Signals 响应式 | Angular/Svelte 5 原生支持，精准 DOM 更新接近原生 JS 性能 |
| AVIF 全球支持 94.9% | 比 JPEG 小 50%，比 WebP 小 20-30%，优先使用 |
| Rust 构建工具主流化 | Turbopack/Rspack/Rolldown 替代 Webpack，构建速度 10-100x |
| 服务端组件 + 流式 SSR | Next.js App Router + RSC 减少 68% 包体积 |

## 七、性能检查清单

**设计阶段**：
- [ ] 是否有首屏加载策略（关键 CSS 内联/预加载/SSR）
- [ ] 大列表是否有虚拟化方案
- [ ] 图片是否有懒加载和格式选择（WebP/AVIF）
- [ ] CSS/JS 是否有分割方案

**开发阶段**：
- [ ] 图片宽高是否指定（防 CLS）
- [ ] 事件监听器是否在组件卸载时清理
- [ ] 定时器是否在组件卸载时清除
- [ ] 长任务（>50ms）是否拆分
