// vite.config.ts — 前端构建与开发服务器配置，由 Wails dev/build 调用。
// SPDX-License-Identifier: MIT

import { fileURLToPath, URL } from 'node:url';

import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

/** 仓库根目录，图标等共享资源位于此处。 */
const repoRoot = fileURLToPath(new URL('..', import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  css: {
    modules: {
      localsConvention: 'camelCaseOnly',
    },
  },
  server: {
    // 端口不能写死成 34115：那是 wails dev server 的默认绑定地址
    // （internal/project/project.go 里 DevServer 缺省为 localhost:34115），
    // 两者抢同一端口会让 wails dev 直接退出、界面全白。
    // 这里用 Vite 自己的默认端口，由 wails 通过 "frontend:dev:serverUrl": "auto"
    // 从 Vite 的启动输出里自动发现地址。
    host: '127.0.0.1',
    fs: {
      // 图标存放在仓库根的 assets/icons，需要显式允许跨目录读取。
      allow: [repoRoot],
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'es2022',
    sourcemap: true,
  },
  test: {
    // 默认按纯逻辑测试跑在 node 环境；需要 DOM 的组件测试用
    // 文件顶部的 @vitest-environment jsdoc 单独声明。
    environment: 'node',
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
  },
});
