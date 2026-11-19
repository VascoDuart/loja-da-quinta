import { act, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { slowRequests } from '../../api/slowRequests'
import { WakingServer } from './WakingServer'

afterEach(() => {
  vi.useRealTimers()
})

describe('WakingServer', () => {
  it('só aparece quando um pedido passa dos 3 s, e desaparece quando acaba', () => {
    vi.useFakeTimers()
    render(<WakingServer />)
    const status = screen.getByRole('status', { hidden: true })
    expect(status).not.toBeVisible()

    let done = () => {}
    act(() => {
      done = slowRequests.track(3_000)
    })
    act(() => {
      vi.advanceTimersByTime(2_999)
    })
    expect(status).not.toBeVisible()

    act(() => {
      vi.advanceTimersByTime(1)
    })
    expect(status).toBeVisible()
    expect(status).toHaveTextContent('A acordar o servidor')

    act(() => done())
    expect(status).not.toBeVisible()
  })

  it('um pedido rápido nunca mostra o aviso', () => {
    vi.useFakeTimers()
    render(<WakingServer />)
    act(() => {
      const done = slowRequests.track(3_000)
      vi.advanceTimersByTime(500)
      done()
      vi.advanceTimersByTime(5_000)
    })
    expect(screen.getByRole('status', { hidden: true })).not.toBeVisible()
    expect(slowRequests.count()).toBe(0)
  })
})
