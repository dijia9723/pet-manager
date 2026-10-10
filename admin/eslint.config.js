import js from '@eslint/js'
import globals from 'globals'
import tseslint from 'typescript-eslint'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import prettier from 'eslint-config-prettier'

// ESLint 9 flat config
export default tseslint.config(
  {
    ignores: [
      'dist',
      'node_modules',
      'dev-dist',
      '**/*.d.ts',
      'Pets-Live2D',
      'public/live2d/vendor',
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2022,
      globals: {
        ...globals.browser,
      },
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      // 组件文件只导出组件（fast refresh 前提）；常量伴随导出放行，规则仅警告不阻断
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
      // 中后台业务中 any 作为逃生舱保留
      '@typescript-eslint/no-explicit-any': 'off',
    },
  },
  {
    // 入口 / 路由装配文件允许混合导出（非组件 + 组件），不参与 fast refresh 约束
    files: ['src/main.tsx', 'src/router/index.tsx', 'src/router/guard.tsx'],
    rules: {
      'react-refresh/only-export-components': 'off',
    },
  },
  {
    // Node 侧配置文件
    files: ['vite.config.ts', 'eslint.config.js'],
    languageOptions: {
      globals: {
        ...globals.node,
      },
    },
  },
  // 放在最后：关闭与 Prettier 冲突的格式化规则
  prettier,
)
