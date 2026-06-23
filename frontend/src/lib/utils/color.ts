// 台灣股市慣例：漲紅跌綠
export function priceColor(change: number): string {
  if (change > 0) return 'text-rise'
  if (change < 0) return 'text-fall'
  return 'text-flat'
}

export function trendColor(trend: string): string {
  if (trend === 'BULLISH') return 'text-rise'
  if (trend === 'BEARISH') return 'text-fall'
  return 'text-muted'
}

export function directionColor(direction: string): string {
  if (direction === 'BUY') return 'bg-rise'
  if (direction === 'SELL') return 'bg-fall'
  return 'bg-yellow-600'
}
