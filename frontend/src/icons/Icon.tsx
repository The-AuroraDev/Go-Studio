/*
 * Icon.tsx — 单色行内图标组件：16px、currentColor、随主题自动适配。
 */

import type { CSSProperties } from 'react';

import { getIconSource } from '@/icons/registry';
import styles from './Icon.module.css';

/** 图标语义色。 */
type IconTone = 'primary' | 'secondary' | 'muted' | 'accent' | 'success' | 'warning' | 'error';

interface IconProps {
  /** 图标名，对应 assets/icons 下的文件名去掉 .svg。 */
  name: string;
  /** 像素尺寸，默认 16px；抽屉标题可用 20px，欢迎页可用 24px。 */
  size?: number;
  /** 语义色，默认继承文字颜色。 */
  tone?: IconTone;
  /** 无障碍标签。省略时视为纯装饰，图标对读屏隐藏。 */
  label?: string;
}

/** 语义色到 CSS Module 类名的映射。 */
function toneClass(tone: IconTone): string {
  switch (tone) {
    case 'primary':
      return styles.tonePrimary;
    case 'secondary':
      return styles.toneSecondary;
    case 'muted':
      return styles.toneMuted;
    case 'accent':
      return styles.toneAccent;
    case 'success':
      return styles.toneSuccess;
    case 'warning':
      return styles.toneWarning;
    case 'error':
      return styles.toneError;
  }
}

/**
 * 渲染行内 SVG。图标来源是构建期打包进来的仓库内静态资源，不是用户输入，
 * 因此这里使用 innerHTML 注入是安全的；换成 <img> 会让图标无法继承 currentColor。
 */
export function Icon({ name, size = 16, tone = 'primary', label }: IconProps) {
  const source = getIconSource(name);
  if (!source) {
    return null;
  }

  const style: CSSProperties = {
    '--gs-icon-size': `${size}px`,
  } as CSSProperties;

  return (
    <span
      className={`${styles.icon} ${toneClass(tone)}`}
      style={style}
      role={label ? 'img' : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
      dangerouslySetInnerHTML={{ __html: source }}
    />
  );
}
