import { describe, it, expect } from 'vitest'
import { formatPrice, formatChangePct, formatVolume } from './format'

describe('formatPrice', () => {
  it('固定兩位小數', () => {
    expect(formatPrice(12.5)).toBe('12.50')
    expect(formatPrice(0)).toBe('0.00')
  })
})

describe('formatChangePct', () => {
  it('正數帶 + 號、負數保留 -、零不帶號', () => {
    expect(formatChangePct(1.2)).toBe('+1.20%')
    expect(formatChangePct(-1.2)).toBe('-1.20%')
    expect(formatChangePct(0)).toBe('0.00%')
  })
})

describe('formatVolume', () => {
  it('依量級縮寫 M / K / 原值', () => {
    expect(formatVolume(3_400_000)).toBe('3.4M')
    expect(formatVolume(12_000)).toBe('12K')
    expect(formatVolume(999)).toBe('999')
  })
})
