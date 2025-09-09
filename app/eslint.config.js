import { FlatCompat } from '@eslint/eslintrc';
import nextPlugin from '@next/eslint-plugin-next';
import reactPlugin from 'eslint-plugin-react';
import hooksPlugin from 'eslint-plugin-react-hooks';
import typescriptEslint from 'typescript-eslint';

const compat = new FlatCompat({
  baseDirectory: import.meta.dirname,
});

export default [
  {
    // Ignore build outputs and deps
    ignores: ['.next/**', 'out/**', 'node_modules/**'],
  },
  ...compat.config({
    extends: [
      'next/core-web-vitals',
      'plugin:@typescript-eslint/recommended',
      'prettier',
    ],
  }),
  {
    files: ['**/*.{js,mjs,cjs,ts,jsx,tsx}'],
    plugins: {
      '@next/next': nextPlugin,
      'react': reactPlugin,
      'react-hooks': hooksPlugin,
      '@typescript-eslint': typescriptEslint.plugin,
    },
    rules: {
      ...nextPlugin.configs.recommended.rules,
      ...reactPlugin.configs.recommended.rules,
      ...hooksPlugin.configs.recommended.rules,
      // React 17+ and Next.js use the new JSX transform; no need to import React in scope
      'react/react-in-jsx-scope': 'off',
    },
  },
];
