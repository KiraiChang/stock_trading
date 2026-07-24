import { defineConfig } from 'vitest/config'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// 獨立於 vite.config.ts：不帶入 build outDir（../backend/internal/ui/dist）、
// emptyOutDir 與 manualChunks，避免跑測試時誤動 build 產物。只重用 svelte plugin
// 讓 .svelte 能編譯。resolve.conditions=['browser'] 讓 svelte 解析到 browser build，
// 元件才能在 jsdom 正常掛載（@testing-library/svelte v4 對 Svelte 4 的必要設定；
// v5 才有的 svelteTesting() plugin 需 Svelte 5，這裡不適用）。cleanup 由 vitest-setup.ts
// 以 afterEach 手動註冊。
export default defineConfig({
  plugins: [svelte({ hot: false })],
  resolve: {
    conditions: ['browser'],
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./vitest-setup.ts'],
    include: ['src/**/*.{test,spec}.ts'],
    // 2GiB host：比照 Python -p=1 / Go GOMAXPROCS=1，限制併發避免 OOM。
    pool: 'forks',
    poolOptions: { forks: { singleFork: true } },
  },
})
