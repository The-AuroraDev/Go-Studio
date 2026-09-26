/*
 * theme.test.ts — 主题三态解析的单元测试。
 */

import { describe, expect, it } from 'vitest';

import { isThemeName, normalizeTheme, resolveTheme } from './theme';

describe('isThemeName', () => {
  it('accepts the three supported values', () => {
    expect(isThemeName('dark')).toBe(true);
    expect(isThemeName('light')).toBe(true);
    expect(isThemeName('system')).toBe(true);
  });

  it('rejects anything else', () => {
    expect(isThemeName('Dark')).toBe(false);
    expect(isThemeName('solarized')).toBe(false);
    expect(isThemeName(undefined)).toBe(false);
    expect(isThemeName(42)).toBe(false);
  });
});

describe('normalizeTheme', () => {
  it('keeps valid values untouched', () => {
    expect(normalizeTheme('light')).toBe('light');
  });

  it('falls back to system for invalid values', () => {
    expect(normalizeTheme('')).toBe('system');
    expect(normalizeTheme(null)).toBe('system');
    expect(normalizeTheme({})).toBe('system');
  });
});

describe('resolveTheme', () => {
  it('passes explicit themes through', () => {
    expect(resolveTheme('dark', () => false)).toBe('dark');
    expect(resolveTheme('light', () => true)).toBe('light');
  });

  it('resolves system via the injected preference probe', () => {
    expect(resolveTheme('system', () => true)).toBe('dark');
    expect(resolveTheme('system', () => false)).toBe('light');
  });

  it('does not consult the probe for explicit themes', () => {
    let called = false;
    resolveTheme('dark', () => {
      called = true;
      return false;
    });
    expect(called).toBe(false);
  });
});
