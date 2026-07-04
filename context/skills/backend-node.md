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

- 先确认 JS/TS、模块格式、框架、路由注册、ORM/数据访问和校验方式。
- 新文件跟随所在模块语言、导出风格、命名和错误处理中间件。
- 不确定包管理器时先看 lock 文件和 scripts。
- 认证权限变更同时检查 guard/middleware、service、route/controller 和数据范围。

## 验证命令

| 层次 | 优先命令 |
|------|---------|
| 编译/语法 | TypeScript 用 `npx tsc --noEmit`；JS 用 `node --check <file>` |
| 启动 | `npm/yarn/pnpm` scripts 或入口加载 |
| 接口 | HTTP 请求、supertest 或项目测试客户端 |
| 测试 | `npm test`、`npx jest`、`npx vitest` 或项目脚本 |

只修复本次变更引入的失败；历史失败记录到总结。
