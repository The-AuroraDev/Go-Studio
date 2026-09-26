/*
 * AppShell.test.tsx — 默认界面的渲染冒烟测试：只有编辑器区域与状态栏，
 * 不得出现侧边栏、标签页、工具栏等常驻 chrome。
 *
 * @vitest-environment jsdom
 */

import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AppShell } from './AppShell';
import { useThemeStore } from '@/stores/themeStore';

// 后端桥接在渲染测试中不可用，统一替换为可控桩，避免测试依赖真实 Wails 运行时。
vi.mock('@/services/bridge', () => ({
  fetchConfig: vi.fn().mockResolvedValue({ theme: 'dark', logLevel: 'info' }),
  ping: vi.fn().mockResolvedValue('pong v1: go-studio'),
  isPong: (reply: string) => reply.startsWith('pong'),
  saveTheme: vi.fn().mockResolvedValue(undefined),
}));

afterEach(() => {
  cleanup();
  useThemeStore.setState({ preference: 'system', resolved: 'dark' });
});

describe('AppShell', () => {
  it('renders the editor region and the status bar', () => {
    render(<AppShell />);

    expect(screen.getByRole('main', { name: 'Editor' })).toBeDefined();
    expect(screen.getByRole('status', { name: 'Status bar' })).toBeDefined();
  });

  it('has no persistent sidebar, tab bar or toolbar', () => {
    const { container } = render(<AppShell />);

    expect(container.querySelector('aside')).toBeNull();
    expect(container.querySelector('[role="tablist"]')).toBeNull();
    expect(container.querySelector('[role="toolbar"]')).toBeNull();
    expect(container.querySelector('nav')).toBeNull();
  });

  it('shows the real backend self-check reply in the status bar', async () => {
    render(<AppShell />);

    await waitFor(() => {
      expect(screen.getByTitle('前后端绑定链路自检结果').textContent).toBe('pong v1: go-studio');
    });
  });

  it('marks toolchain fields as pending instead of inventing values', () => {
    render(<AppShell />);

    for (const title of ['尚未打开文件', 'Git 集成在阶段 7 接入', 'gopls 在阶段 4 接入']) {
      const field = screen.getByTitle(title);
      expect(field.textContent).toBe('—');
      expect(field.dataset.pending).toBe('true');
    }
  });
});
