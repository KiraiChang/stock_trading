// jest-dom 的 vitest 版本：讓 expect 支援 toBeInTheDocument 等 DOM matcher。
import '@testing-library/jest-dom/vitest'
import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/svelte'

// @testing-library/svelte v4 沒有 auto-cleanup plugin，手動在每個 test 後卸載元件，
// 避免跨測試 DOM 殘留互相污染。
afterEach(() => {
  cleanup()
})
