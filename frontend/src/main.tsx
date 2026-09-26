/*
 * main.tsx — 前端入口：引入设计 token 与全局样式后挂载根组件。
 */

import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { App } from '@/App';
import '@/app/global.css';
import '@/tokens/tokens.css';

const container = document.querySelector<HTMLDivElement>('#root');
if (!container) {
  throw new Error('root container #root is missing from index.html');
}

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
