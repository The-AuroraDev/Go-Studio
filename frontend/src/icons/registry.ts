/*
 * registry.ts — 图标注册表：构建期把 assets/icons 下的 SVG 全部内联进产物。
 * 运行时不做任何网络请求，图标缺失只影响单个图标而不影响界面启动。
 */

/**
 * 图标源文件目录，位于仓库根的 assets/icons。
 * go-old.svg 是 Go 原始素材，仅作为 go.svg 与 go-brand.svg 的溯源依据，不参与注册。
 */
const iconModules = import.meta.glob<string>(
  ['../../../assets/icons/*.svg', '!../../../assets/icons/go-old.svg'],
  {
    query: '?raw',
    import: 'default',
    eager: true,
  },
);

/** 从文件路径还原图标名：assets/icons/port-forward.svg -> port-forward */
function toIconName(filePath: string): string {
  const fileName = filePath.split('/').pop() ?? filePath;
  return fileName.replace(/\.svg$/, '');
}

const sources = new Map<string, string>(
  Object.entries(iconModules).map(([filePath, source]) => [toIconName(filePath), source]),
);

/** 全部可用图标名，按字典序排列，供校验脚本与测试使用。 */
export const iconNames: readonly string[] = [...sources.keys()].sort();

/** 图标是否已注册。 */
export function hasIcon(name: string): boolean {
  return sources.has(name);
}

/** 读取图标 SVG 源码，未注册时返回 undefined。 */
export function getIconSource(name: string): string | undefined {
  return sources.get(name);
}
