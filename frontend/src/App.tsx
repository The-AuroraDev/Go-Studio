/*
 * App.tsx — 应用根组件：主题提供者包裹默认外壳。
 */

import { AppShell } from '@/app/AppShell';
import { ThemeProvider } from '@/theme/ThemeProvider';

export function App() {
  return (
    <ThemeProvider>
      <AppShell />
    </ThemeProvider>
  );
}
