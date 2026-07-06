// toISOString() 回傳的是 UTC 日期，台灣時間 00:00-07:59 這段 UTC 日期仍是
// 前一天，會讓使用者早上查詢/同步時算錯「今天」（見 docs/review.md）。統一
// 用 Intl.DateTimeFormat 明確以 Asia/Taipei 時區格式化，en-CA locale 的輸出
// 格式就是 YYYY-MM-DD，不需要再手動組字串。抽成共用模組避免每個頁面各自
// 重寫一份，重蹈同一個時區 bug。
const taipeiDateFormatter = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Taipei' })

// 注意：函式體刻意維持兩個陳述式（而不是單行 return expression）——vite/
// esbuild 生產建置曾經把單行 return 的零參數函式呼叫靜默消除成 undefined
// （見 docs/review.md 與相關 commit），多寫一行區域變數可避免同一個 bug
// 再犯。
export function todayStr(): string {
  const d = new Date()
  return taipeiDateFormatter.format(d)
}

export function daysAgo(n: number): string {
  const d = new Date()
  d.setDate(d.getDate() - n)
  return taipeiDateFormatter.format(d)
}
