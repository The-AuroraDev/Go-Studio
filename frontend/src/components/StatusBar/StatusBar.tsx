/*
 * StatusBar.tsx — 底部状态栏：唯一常驻的信息条。
 *
 * 阶段 0 只呈现后端已经真实提供的数据：前后端自检结果、编码与换行符、行列。
 * Git 分支、gopls 状态、诊断数等字段要等对应工具链接入后再显示，
 * 未接入时渲染为空状态而不是编造数值。
 */

import { useEffect, useState } from 'react';

import { isPong, ping } from '@/services/bridge';
import styles from './StatusBar.module.css';

/** 阶段 0 的默认编码与换行符，与 PRD 约定一致。 */
const DEFAULT_ENCODING = 'UTF-8';
const DEFAULT_EOL = 'LF';

/** 后端尚未接入时统一使用的空状态占位符。 */
const PENDING = '—';

interface FieldProps {
  label: string;
  value: string;
  /** 该字段依赖的工具链尚未接入时为 true，用于弱化显示。 */
  pending?: boolean;
  title?: string;
}

function Field({ label, value, pending = false, title }: FieldProps) {
  return (
    <span
      className={`${styles.field} ${pending ? styles.pending : ''}`}
      title={title ?? `${label}: ${value}`}
      data-pending={pending ? 'true' : undefined}
    >
      {value}
    </span>
  );
}

export function StatusBar() {
  const [backend, setBackend] = useState<string>(PENDING);

  useEffect(() => {
    let cancelled = false;
    void ping('go-studio')
      .then((reply) => {
        if (!cancelled) {
          setBackend(isPong(reply) ? reply : PENDING);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setBackend(PENDING);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <footer className={styles.statusbar} role="status" aria-label="Status bar">
      <div className={styles.group}>
        <Field label="File" value={PENDING} pending title="尚未打开文件" />
        <Field label="Git branch" value={PENDING} pending title="Git 集成在阶段 7 接入" />
        <Field label="Go version" value={PENDING} pending title="Go 工具链在阶段 4 接入" />
        <Field label="gopls" value={PENDING} pending title="gopls 在阶段 4 接入" />
        <Field label="Diagnostics" value={PENDING} pending title="诊断在阶段 4 接入" />
      </div>
      <div className={styles.group}>
        <Field label="Backend" value={backend} title="前后端绑定链路自检结果" />
        <Field label="Position" value="1:1" title="行列号" />
        <Field label="Encoding" value={DEFAULT_ENCODING} title="文件编码" />
        <Field label="Line endings" value={DEFAULT_EOL} title="换行符" />
      </div>
    </footer>
  );
}
