import { quotes } from '../stores/market'
import { signals } from '../stores/signals'
import type { Signal } from '../stores/signals'
import type { StockQuote } from '../stores/market'

type WsEvent = {
  type: 'candle' | 'indicator' | 'signal'
  symbol: string
  data: Record<string, unknown>
}

class MarketSocket {
  private ws: WebSocket | null = null
  private url = ''
  private reconnectDelay = 3000

  connect(url: string): void {
    this.url = url
    this.open()
  }

  private open(): void {
    this.ws = new WebSocket(this.url)

    this.ws.onopen = () => {
      console.log('[ws] connected')
    }

    this.ws.onmessage = (e: MessageEvent) => {
      try {
        const evt = JSON.parse(e.data as string) as WsEvent
        if (evt.type === 'candle')    this.handleCandle(evt)
        if (evt.type === 'indicator') this.handleIndicator(evt)
        if (evt.type === 'signal')    this.handleSignal(evt)
      } catch {
        // ignore malformed messages
      }
    }

    this.ws.onclose = () => {
      console.log('[ws] disconnected, reconnecting...')
      setTimeout(() => this.open(), this.reconnectDelay)
    }

    this.ws.onerror = () => {
      this.ws?.close()
    }
  }

  subscribe(symbols: string[]): void {
    this.ws?.send(JSON.stringify({ action: 'subscribe', symbols }))
  }

  unsubscribe(symbols: string[]): void {
    this.ws?.send(JSON.stringify({ action: 'unsubscribe', symbols }))
  }

  private handleCandle(evt: WsEvent): void {
    const d = evt.data as Partial<StockQuote> & { open?: number; close?: number }
    quotes.update((map) => {
      const existing = map.get(evt.symbol) ?? ({} as StockQuote)
      const updated: StockQuote = {
        ...existing,
        symbol: evt.symbol,
        open: (d.open as number) ?? existing.open,
        close: (d.close as number) ?? existing.close,
        high: (d.high as number) ?? existing.high,
        low: (d.low as number) ?? existing.low,
        volume: (d.volume as number) ?? existing.volume,
        change: (d.close as number ?? existing.close) - (d.open as number ?? existing.open),
        changePct: existing.open
          ? (((d.close as number ?? existing.close) - existing.open) / existing.open) * 100
          : 0,
        name: existing.name ?? '',
        volRatio: existing.volRatio ?? 0,
        ma5: existing.ma5 ?? 0,
        ma20: existing.ma20 ?? 0,
        rsi14: existing.rsi14 ?? 0,
        trend: existing.trend ?? '',
        hasSignal: existing.hasSignal ?? false,
      }
      map.set(evt.symbol, updated)
      return new Map(map)
    })
  }

  private handleIndicator(evt: WsEvent): void {
    const d = evt.data as Record<string, number>
    quotes.update((map) => {
      const existing = map.get(evt.symbol) ?? ({} as StockQuote)
      map.set(evt.symbol, {
        ...existing,
        symbol: evt.symbol,
        name: existing.name ?? '',
        close: existing.close ?? 0,
        open: existing.open ?? 0,
        high: existing.high ?? 0,
        low: existing.low ?? 0,
        change: existing.change ?? 0,
        changePct: existing.changePct ?? 0,
        volume: existing.volume ?? 0,
        volRatio: d.vol_ratio ?? existing.volRatio ?? 0,
        ma5: d.ma5 ?? existing.ma5 ?? 0,
        ma20: d.ma20 ?? existing.ma20 ?? 0,
        rsi14: d.rsi14 ?? existing.rsi14 ?? 0,
        trend: (d.trend as StockQuote['trend']) ?? existing.trend ?? '',
        hasSignal: existing.hasSignal ?? false,
      })
      return new Map(map)
    })
  }

  private handleSignal(evt: WsEvent): void {
    const sig = evt.data as Signal
    signals.update((list) => [sig, ...list].slice(0, 100))
    // 標記 quote 有訊號
    quotes.update((map) => {
      const existing = map.get(evt.symbol)
      if (existing) {
        map.set(evt.symbol, { ...existing, hasSignal: true })
      }
      return new Map(map)
    })
  }
}

export const socket = new MarketSocket()
