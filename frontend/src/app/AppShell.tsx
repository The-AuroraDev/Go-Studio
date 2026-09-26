/*
 * AppShell.tsx — 默认界面的唯一布局：全屏编辑器区域加一条底部状态栏。
 * 这里不放置侧边栏、标签页或工具栏，浮层由阶段 3 的浮层系统接管。
 */

import type { ReactNode } from 'react';

import { StatusBar } from '@/components/StatusBar/StatusBar';
import styles from './AppShell.module.css';

interface AppShellProps {
  /** 编辑器区域内容。阶段 0 为空，阶段 2 接入 Monaco。 */
  editor?: ReactNode;
}

/** 应用外壳，保证状态栏始终可见且不被任何浮层遮挡。 */
export function AppShell({ editor }: AppShellProps) {
  return (
    <div className={styles.shell}>
      <main className={styles.editor} aria-label="Editor">
        {editor}
      </main>
      <StatusBar />
    </div>
  );
}
