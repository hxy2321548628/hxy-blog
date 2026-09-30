import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import tseslint from 'typescript-eslint'

// 使用 ESLint Flat Config：先忽略构建产物，再对所有 TS/TSX 文件启用
// JavaScript、TypeScript 和 React Hooks 的推荐规则。
export default tseslint.config(
  // dist 是 Vite 生成物，不应作为源码重复检查。
  { ignores: ['dist'] },
  {
    files: ['**/*.{ts,tsx}'],
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    languageOptions: {
      // 浏览器全局变量允许 window/document；ES2022 与 tsconfig 的目标保持一致。
      ecmaVersion: 2022,
      globals: globals.browser,
    },
    plugins: {
      'react-hooks': reactHooks,
    },
    rules: reactHooks.configs.flat.recommended.rules,
  },
)
