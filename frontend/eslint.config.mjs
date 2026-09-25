import { defineConfig, globalIgnores } from 'eslint/config';
import nextVitals from 'eslint-config-next/core-web-vitals';
import nextTypescript from 'eslint-config-next/typescript';
import typescriptEslint from '@typescript-eslint/eslint-plugin';

export default defineConfig([
  ...nextVitals,
  ...nextTypescript,
  {
    plugins: {
      '@typescript-eslint': typescriptEslint,
    },
    rules: {
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
      '@typescript-eslint/consistent-type-imports': 'warn',
      'react-hooks/incompatible-library': 'warn',
      'react-hooks/purity': 'warn',
      'react-hooks/set-state-in-effect': 'warn',
      'react/no-children-prop': 'warn',
      'react/no-unescaped-entities': 'warn',
      'no-restricted-syntax': [
        'error',
        {
          selector: "TSAsExpression[typeAnnotation.typeName.name='unknown']",
          message: 'Avoid `as unknown as X` — prefer a properly typed generic.',
        },
      ],
    },
  },
  globalIgnores([
    '.next/**',
    'out/**',
    'node_modules/**',
    'dist/**',
    'playwright-report/**',
    'test-results/**',
  ]),
]);
