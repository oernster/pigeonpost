/// <reference types="vitest/config" />
import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: false,
    setupFiles: ['./src/test/setup.ts'],
    coverage: {
      provider: 'v8',
      // Only the pure logic modules carry the 100% gate. This list grows as each pure
      // module is extracted with its test; every listed module must hit 100%. Hooks and
      // components are tested but not gated (a React hook fuses logic with framework
      // plumbing, so a blanket 100% there buys brittle tests, not correctness).
      include: [
        'src/messageText.ts',
        'src/shortcuts.ts',
        'src/print.ts',
        'src/readerFormat.ts',
        'src/composeAddresses.ts',
        'src/accountProviders.ts',
        'src/sidebarDnd.ts',
        'src/calendarModel.ts',
        'src/replyDraft.ts',
      ],
      thresholds: {lines: 100, functions: 100, statements: 100, branches: 100},
    },
  },
})
