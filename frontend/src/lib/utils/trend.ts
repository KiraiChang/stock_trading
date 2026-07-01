import type { StockQuote } from '../stores/market'

// 後端的 HH/HL 結構趨勢（signal.Engine 的 DetectTrend）只在有訊號時才會
// 隨 Signal 一起送出；沒有訊號紀錄可查時，用 MA5 相對 MA20 的排列位置
// 當作簡化版趨勢判斷，讓監控清單在沒有訊號的股票上也能顯示大致趨勢。
export function deriveTrendFromMA(ma5: number, ma20: number): StockQuote['trend'] {
  if (!ma5 || !ma20) return ''
  if (ma5 > ma20) return 'BULLISH'
  if (ma5 < ma20) return 'BEARISH'
  return 'SIDEWAYS'
}
