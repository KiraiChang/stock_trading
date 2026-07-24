import { describe, it, expect, afterEach } from 'vitest'
import { get } from 'svelte/store'
import { currentRoute } from './router'

afterEach(() => {
  currentRoute.set('dashboard')
})

describe('currentRoute', () => {
  it('預設為 dashboard', () => {
    expect(get(currentRoute)).toBe('dashboard')
  })

  it('可切換到其他 route', () => {
    currentRoute.set('chips')
    expect(get(currentRoute)).toBe('chips')
  })
})
