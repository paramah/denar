import js from '@eslint/js'
import jsxA11y from 'eslint-plugin-jsx-a11y'
import react from 'eslint-plugin-react'
import reactHooks from 'eslint-plugin-react-hooks'
import globals from 'globals'
import tseslint from 'typescript-eslint'

export default tseslint.config(
  {
    ignores: [
      'dist',
      
      'playwright-report', 'node_modules',
      'test-results',
      'src/api/schema.d.ts',
    ],
  },
  js.configs.recommended,
  tseslint.configs.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      react.configs.flat.recommended,
      react.configs.flat['jsx-runtime'],
      reactHooks.configs.flat.recommended,
      jsxA11y.flatConfigs.recommended,
    ],
    languageOptions: {
      globals: globals.browser,
    },
    settings: {
      react: { version: 'detect' },
    },
    rules: {
      'no-console': 'error',
      'react/no-danger': 'error',
      // Props are typed by TypeScript.
      'react/prop-types': 'off',
    },
  },
  {
    files: ['*.config.{js,ts}', 'e2e/**'],
    languageOptions: {
      globals: globals.node,
    },
  },
)
