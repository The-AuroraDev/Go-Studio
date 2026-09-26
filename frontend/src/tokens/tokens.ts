/*
 * tokens.ts — 设计 token 的 TypeScript 视图，供 Monaco、xterm.js 等无法读取
 * CSS 变量的消费方使用。数值与 tokens.css 一一对应，由 tokens.test.ts 保证同步。
 */

import type { config } from '../../wailsjs/go/models';

/** 间距阶梯，基准 4px。 */
export const SPACING = {
  xs: '4px',
  sm: '8px',
  md: '12px',
  lg: '16px',
  xl: '24px',
  xxl: '32px',
} as const;

/** 圆角。避免大圆角是设计红线。 */
export const RADII = {
  sm: '2px',
  md: '4px',
  lg: '6px',
} as const;

/** 动效时长与缓动，只用于浮层进出与状态变化。 */
export const MOTION = {
  fastMs: 100,
  normalMs: 150,
  easing: 'ease-out',
} as const;

/** 层级顺序，浮层之间不可随意调整。 */
export const Z_INDEX = {
  editor: 0,
  statusbar: 10,
  drawer: 20,
  modal: 30,
  palette: 40,
  toast: 50,
} as const;

/** 字号，与 DESIGN.md 的九档保持一致。 */
export const FONT_SIZE = {
  xs: 10,
  sm: 11,
  base: 12,
  md: 13,
  code: 13,
  codeLg: 14,
  statusbar: 11,
  paletteInput: 13,
  paletteRow: 12,
} as const;

/** 字重。 */
export const FONT_WEIGHT = {
  regular: 400,
  medium: 500,
  semibold: 600,
} as const;

/** 组件固定尺寸。 */
export const COMPONENT_SIZE = {
  icon: 16,
  iconButton: 24,
  statusbarHeight: 26,
  paletteInputHeight: 44,
  paletteRowHeight: 30,
  overlayHeaderHeight: 42,
  drawerHeaderHeight: 38,
  buttonHeight: 26,
  inputHeight: 30,
  listRowHeight: 26,
  treeRowHeight: 23,
} as const;

/** 语义色键名，顺序与 tokens.css 中的声明一致。 */
export const COLOR_KEYS = [
  'bg-base',
  'bg-surface',
  'bg-elevated',
  'border-subtle',
  'border-strong',
  'text-primary',
  'text-secondary',
  'text-muted',
  'accent',
  'accent-hover',
  'selection',
  'highlight',
  'success',
  'warning',
  'error',
  'info',
  'text-disabled',
  'bg-disabled',
] as const;

export type ColorKey = (typeof COLOR_KEYS)[number];

/** 暗色主题取值，数值来自 DESIGN.md 第 6 节。 */
export const DARK_COLORS: Readonly<Record<ColorKey, string>> = {
  'bg-base': '#0e0f11',
  'bg-surface': '#15171a',
  'bg-elevated': '#1b1e22',
  'border-subtle': '#262a2f',
  'border-strong': '#3a4047',
  'text-primary': '#e6e8eb',
  'text-secondary': '#a8b0b8',
  'text-muted': '#6f7882',
  accent: '#4c8bf5',
  'accent-hover': '#6ba1ff',
  selection: '#1f3a5f',
  highlight: '#2a2f36',
  success: '#3fb950',
  warning: '#d29922',
  error: '#f85149',
  info: '#58a6ff',
  'text-disabled': '#4a5158',
  'bg-disabled': '#1a1d21',
};

/** 亮色主题取值，数值来自 DESIGN.md 第 6 节。 */
export const LIGHT_COLORS: Readonly<Record<ColorKey, string>> = {
  'bg-base': '#ffffff',
  'bg-surface': '#f6f7f9',
  'bg-elevated': '#ffffff',
  'border-subtle': '#d8dce1',
  'border-strong': '#b6bcc4',
  'text-primary': '#1b1f24',
  'text-secondary': '#57606a',
  'text-muted': '#8b949e',
  accent: '#0969da',
  'accent-hover': '#218bff',
  selection: '#dbeafe',
  highlight: '#eef1f4',
  success: '#1a7f37',
  warning: '#9a6700',
  error: '#cf222e',
  info: '#0969da',
  'text-disabled': '#8c959f',
  'bg-disabled': '#f0f2f4',
};

/** 主题字面量，与 internal/config 的常量一一对应。 */
export const THEME_NAMES = ['dark', 'light', 'system'] as const;
export type ThemeName = (typeof THEME_NAMES)[number];

/** 解析后的实际配色，system 会被消解成 dark 或 light。 */
export type ResolvedTheme = Exclude<ThemeName, 'system'>;

/** 主题对应的颜色表，供 Monaco 与 xterm.js 消费。 */
export function colorsFor(theme: ResolvedTheme): Readonly<Record<ColorKey, string>> {
  return theme === 'light' ? LIGHT_COLORS : DARK_COLORS;
}

/** 后端配置对象的结构化视图，字段与 internal/config.Config 对应。 */
export type StudioConfig = config.Config;
