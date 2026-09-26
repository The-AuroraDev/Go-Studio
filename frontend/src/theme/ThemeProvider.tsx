/*
 * ThemeProvider.tsx — 把主题状态同步到 documentElement，并监听系统偏好变化。
 */

import { type ReactNode, useEffect } from 'react';

import { fetchConfig } from '@/services/bridge';
import { useThemeStore } from '@/stores/themeStore';

/** 系统暗色偏好媒体查询，system 主题据此消解。 */
const darkQuery = '(prefers-color-scheme: dark)';

interface ThemeProviderProps {
  children: ReactNode;
}

/**
 * 主题提供者。职责只有三件：把 resolved 写到 data-theme、跟随系统变化、
 * 用后端配置初始化偏好。配色本身全部来自 tokens.css，这里不出现任何色值。
 */
export function ThemeProvider({ children }: ThemeProviderProps) {
  const resolved = useThemeStore((state) => state.resolved);
  const refresh = useThemeStore((state) => state.refresh);
  const hydrate = useThemeStore((state) => state.hydrate);

  useEffect(() => {
    document.documentElement.dataset.theme = resolved;
  }, [resolved]);

  useEffect(() => {
    const query = window.matchMedia(darkQuery);
    const onChange = () => refresh();
    query.addEventListener('change', onChange);
    return () => query.removeEventListener('change', onChange);
  }, [refresh]);

  useEffect(() => {
    let cancelled = false;
    void fetchConfig()
      .then((config) => {
        if (!cancelled) {
          hydrate(config.theme);
        }
      })
      .catch(() => {
        // 后端不可用时保留默认的 system 偏好，界面不会因此无法启动。
      });
    return () => {
      cancelled = true;
    };
  }, [hydrate]);

  return <>{children}</>;
}
