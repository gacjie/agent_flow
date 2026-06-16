# Node.js 后端速查

- 先识别项目命令：README、package.json scripts、tsconfig.json、jest.config、.env.example。
- 常用验证：`npx tsc --noEmit`（TS 编译）、`node --check`（JS 语法）、`npm test`、`npm start`。
- 安全重点：参数化查询、输入校验、Promise rejection 处理、敏感信息不入日志、`crypto.timingSafeEqual` 验密。
- Node 默认：项目存在 lock 文件时用对应包管理器（npm/yarn/pnpm）；TypeScript 项目优先 `npx tsc` 验证。
