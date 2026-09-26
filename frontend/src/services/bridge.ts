/*
 * bridge.ts — Wails 绑定调用的唯一入口。
 * 组件不允许直接 import wailsjs，必须经过这里以便统一处理错误与后续审计。
 */

import { Config, LogFilePath, Ping, PingVersion, SetTheme } from '../../wailsjs/go/app/App';

/** 后端自检回包前缀，用于确认链路真实可用。 */
const PONG_PREFIX = 'pong';

/** 前后端自检：把消息送到 Go 后端并拿回回包。 */
export async function ping(message: string): Promise<string> {
  return Ping(message);
}

/** 判定自检回包是否合法。 */
export function isPong(reply: string): boolean {
  return reply.startsWith(PONG_PREFIX);
}

/** 后端自检协议版本。 */
export async function pingVersion(): Promise<number> {
  return PingVersion();
}

/** 读取后端当前生效的完整配置。 */
export async function fetchConfig(): Promise<{ theme: string; logLevel: string }> {
  const loaded = await Config();
  return { theme: loaded.theme, logLevel: loaded.logLevel };
}

/** 持久化主题取值，非法取值由后端拒绝并返回错误。 */
export async function saveTheme(theme: string): Promise<void> {
  await SetTheme(theme);
}

/** 读取当前日志文件路径，供设置界面展示。 */
export async function fetchLogFilePath(): Promise<string> {
  return LogFilePath();
}
