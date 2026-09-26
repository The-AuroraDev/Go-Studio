/*
 * themeStore.ts — 主题状态：唯一真源是后端配置，本地 store 只做缓存与乐观更新。
 */

import { create } from 'zustand';

import { saveTheme } from '@/services/bridge';
import type { ResolvedTheme, ThemeName } from '@/tokens/tokens';
import { normalizeTheme, resolveTheme, systemPrefersDark } from '@/theme/theme';

interface ThemeState {
  /** 用户选择的三态取值。 */
  preference: ThemeName;
  /** 消解后的实际配色，组件只应依赖它。 */
  resolved: ResolvedTheme;
  /** 切换偏好并持久化。返回是否持久化成功，失败时调用方可提示用户。 */
  setPreference: (preference: ThemeName) => Promise<boolean>;
  /** 用后端下发的配置初始化界面。 */
  hydrate: (theme: unknown) => void;
  /** 系统偏好变化时重新消解，仅在 preference 为 system 时生效。 */
  refresh: () => void;
}

/** 把偏好写入后端配置，返回是否持久化成功。 */
async function persistTheme(preference: ThemeName): Promise<boolean> {
  try {
    await saveTheme(preference);
    return true;
  } catch {
    // 持久化失败不应让界面卡在旧主题，这里选择保持本地生效并上报失败。
    return false;
  }
}

export const useThemeStore = create<ThemeState>((set, get) => ({
  preference: 'system',
  resolved: resolveTheme('system', systemPrefersDark),
  setPreference: async (preference) => {
    set({
      preference,
      resolved: resolveTheme(preference, systemPrefersDark),
    });
    return persistTheme(preference);
  },
  hydrate: (theme) => {
    const preference = normalizeTheme(theme);
    set({
      preference,
      resolved: resolveTheme(preference, systemPrefersDark),
    });
  },
  refresh: () => {
    const { preference } = get();
    set({ resolved: resolveTheme(preference, systemPrefersDark) });
  },
}));
