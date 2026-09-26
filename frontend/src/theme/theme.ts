/*
 * theme.ts — 主题解析：把配置里的三态取值消解成实际配色。
 * 纯函数，不触碰 DOM，便于单元测试。
 */

import { THEME_NAMES, type ResolvedTheme, type ThemeName } from '@/tokens/tokens';

/** 系统偏好探测函数，由 ThemeProvider 注入以便测试替换。 */
export type PrefersDark = () => boolean;

/** 判断一个字符串是否为受支持的主题取值。 */
export function isThemeName(value: unknown): value is ThemeName {
  return typeof value === 'string' && (THEME_NAMES as readonly string[]).includes(value);
}

/** 非法取值一律回退到 system，保证界面永远可用。 */
export function normalizeTheme(value: unknown): ThemeName {
  return isThemeName(value) ? value : 'system';
}

/** 把三态主题消解成实际配色，system 交给系统偏好决定。 */
export function resolveTheme(theme: ThemeName, systemPrefersDark: PrefersDark): ResolvedTheme {
  if (theme === 'system') {
    return systemPrefersDark() ? 'dark' : 'light';
  }
  return theme;
}

/** 判断系统是否偏好暗色。运行环境一定提供 matchMedia，缺失时按亮色处理。 */
export function systemPrefersDark(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return false;
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}
