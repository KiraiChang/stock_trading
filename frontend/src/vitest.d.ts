// 讓 svelte-check（type check）也知道 jest-dom 對 vitest Assertion 的擴充，
// 測試檔用 toBeInTheDocument 等 matcher 才不會被判成型別錯誤。
import '@testing-library/jest-dom/vitest'
