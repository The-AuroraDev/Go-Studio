/*
 * tokens.test.ts — 保证 tokens.ts 与 tokens.css 的数值逐项一致。
 * 两份文件任一端被单独修改时，这个测试会失败，从而杜绝设计 token 漂移。
 */

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { describe, expect, it } from 'vitest';

import {
  COLOR_KEYS,
  DARK_COLORS,
  LIGHT_COLORS,
  MOTION,
  RADII,
  SPACING,
  Z_INDEX,
  type ColorKey,
} from './tokens';

const css = readFileSync(fileURLToPath(new URL('./tokens.css', import.meta.url)), 'utf8');

/** 提取 CSS 中某个自定义属性的声明值。 */
function cssValue(property: string): string {
  const pattern = new RegExp(`${property}\\s*:\\s*([^;]+);`);
  const match = pattern.exec(css);
  if (!match?.[1]) {
    throw new Error(`token ${property} is not declared in tokens.css`);
  }
  return match[1].trim();
}

describe('spacing tokens', () => {
  const cases: [keyof typeof SPACING, string][] = [
    ['xs', '--gs-space-1'],
    ['sm', '--gs-space-2'],
    ['md', '--gs-space-3'],
    ['lg', '--gs-space-4'],
    ['xl', '--gs-space-5'],
    ['xxl', '--gs-space-6'],
  ];

  it.each(cases)('SPACING.%s matches %s', (key, property) => {
    expect(cssValue(property)).toBe(SPACING[key]);
  });
});

describe('radius tokens', () => {
  it('SM matches', () => expect(cssValue('--gs-radius-sm')).toBe(RADII.sm));
  it('MD matches', () => expect(cssValue('--gs-radius-md')).toBe(RADII.md));
  it('LG matches', () => expect(cssValue('--gs-radius-lg')).toBe(RADII.lg));
});

describe('motion tokens', () => {
  it('fast duration matches', () => {
    expect(cssValue('--gs-duration-fast')).toBe(`${MOTION.fastMs}ms`);
  });
  it('normal duration matches', () => {
    expect(cssValue('--gs-duration-normal')).toBe(`${MOTION.normalMs}ms`);
  });
  it('easing matches', () => expect(cssValue('--gs-easing')).toBe(MOTION.easing));
});

describe('z-index tokens', () => {
  it.each(Object.entries(Z_INDEX))('%s matches', (key, value) => {
    expect(cssValue(`--gs-z-${key}`)).toBe(String(value));
  });
});

describe('color tokens', () => {
  /** 取出某个主题块内的声明值，避免同名变量在两个主题间互相覆盖。 */
  function themedValue(property: string, theme: 'dark' | 'light'): string {
    const selector = theme === 'dark' ? 'dark' : 'light';
    const block = new RegExp(`\\[data-theme='${selector}'\\]\\s*\\{([^}]*)\\}`).exec(css);
    const body = block?.[1];
    if (!body) {
      throw new Error(`theme block ${theme} is missing from tokens.css`);
    }
    const match = new RegExp(`${property}\\s*:\\s*([^;]+);`).exec(body);
    if (!match?.[1]) {
      throw new Error(`token ${property} is not declared in ${theme} block`);
    }
    return match[1].trim();
  }

  const keys: ColorKey[] = [...COLOR_KEYS];

  it('covers all 18 color keys', () => {
    expect(keys).toHaveLength(18);
  });

  it.each(keys)('dark --gs-color-%s matches tokens.ts', (key) => {
    expect(themedValue(`--gs-color-${key}`, 'dark')).toBe(DARK_COLORS[key]);
  });

  it.each(keys)('light --gs-color-%s matches tokens.ts', (key) => {
    expect(themedValue(`--gs-color-${key}`, 'light')).toBe(LIGHT_COLORS[key]);
  });
});
