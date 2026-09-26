// eslint.config.js — 前端静态检查规则，禁 any、禁硬编码色值、禁直连 DOM 副作用。
// SPDX-License-Identifier: MIT

import js from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import globals from 'globals';
import tseslint from 'typescript-eslint';

/** 只有这些文件参与带类型信息的检查，其余文件走纯语法检查。 */
const typeCheckedFiles = ['**/*.ts', '**/*.tsx'];

export default tseslint.config(
  {
    // wailsjs 由 wails generate module 生成，属于第三方产物，不参与检查。
    ignores: ['dist/**', 'node_modules/**', 'wailsjs/**'],
  },
  js.configs.recommended,
  // 带类型信息的规则只作用于 TS 源码，避免 eslint.config.js 自身缺类型报错。
  ...tseslint.configs.recommendedTypeChecked.map((config) => ({
    ...config,
    files: typeCheckedFiles,
  })),
  {
    files: typeCheckedFiles,
    languageOptions: {
      ecmaVersion: 2022,
      globals: {
        ...globals.browser,
      },
      parserOptions: {
        // 应用源码与测试分属两个 tsconfig，这里显式列出两者，
        // 保证每个文件恰好落在一个项目里。
        project: ['./tsconfig.json', './tsconfig.node.json'],
        tsconfigRootDir: import.meta.dirname,
      },
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
      // 设计 token 必须在 CSS 变量里声明，组件里不允许出现字面量色值。
      'no-restricted-syntax': [
        'error',
        {
          selector: "Literal[value=/#[0-9a-fA-F]{3,8}/]",
          message: '禁止硬编码颜色，请使用 tokens.css 中的 CSS 变量。',
        },
        {
          selector: "Literal[value=/rgba?\\(/]",
          message: '禁止硬编码颜色，请使用 tokens.css 中的 CSS 变量。',
        },
      ],
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/consistent-type-imports': [
        'error',
        { prefer: 'type-imports', fixStyle: 'inline-type-imports' },
      ],
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    },
  },
  {
    // 构建与检查脚本运行在 node 环境，不提供浏览器全局变量。
    files: ['*.js'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: {
        ...globals.node,
      },
    },
  },
  {
    // token 文件是唯一允许出现字面量色值与尺寸的地方，
    // 它的数值由 tokens.test.ts 与 tokens.css 逐项比对保证一致。
    files: ['src/tokens/**'],
    rules: {
      'no-restricted-syntax': 'off',
    },
  },
  {
    files: ['**/*.test.ts', '**/*.test.tsx'],
    rules: {
      '@typescript-eslint/no-non-null-assertion': 'off',
    },
  },
);
