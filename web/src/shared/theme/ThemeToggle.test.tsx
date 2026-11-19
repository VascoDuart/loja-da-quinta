import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { initTheme, savedTheme } from './theme'
import { ThemeToggle } from './ThemeToggle'

describe('ThemeToggle', () => {
  it('troca entre claro e escuro e lembra a escolha', async () => {
    render(<ThemeToggle />)
    const button = screen.getByRole('button', { name: 'Mudar para o modo escuro' })

    await userEvent.click(button)
    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(savedTheme()).toBe('dark')
    expect(screen.getByRole('button', { name: 'Mudar para o modo claro' })).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Mudar para o modo claro' }))
    expect(document.documentElement.dataset.theme).toBe('light')
    expect(savedTheme()).toBe('light')
  })

  it('sem escolha guardada, segue o tema do sistema', () => {
    initTheme()
    expect(document.documentElement.dataset.theme).toBeUndefined()
    render(<ThemeToggle />)
    expect(screen.getByRole('button', { name: 'Mudar para o modo escuro' })).toBeInTheDocument()
  })

  it('a escolha guardada é aplicada no arranque', () => {
    // eslint-disable-next-line no-restricted-globals -- simular uma escolha anterior
    localStorage.setItem('loja-da-quinta:tema', 'dark')
    initTheme()
    expect(document.documentElement.dataset.theme).toBe('dark')
  })
})
