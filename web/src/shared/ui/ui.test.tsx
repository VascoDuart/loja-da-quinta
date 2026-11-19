import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Button, Chip, SeasonTip, StockPill } from './ui'

describe('componentes base', () => {
  it('Button é type="button" por omissão, para não submeter formulários por engano', () => {
    render(<Button>Registar venda</Button>)
    expect(screen.getByRole('button', { name: 'Registar venda' })).toHaveAttribute('type', 'button')
  })

  it('Chip expõe o estado com aria-pressed', () => {
    render(
      <>
        <Chip selected>Fruta</Chip>
        <Chip selected={false}>Legumes</Chip>
      </>,
    )
    expect(screen.getByRole('button', { name: 'Fruta' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Legumes' })).toHaveAttribute('aria-pressed', 'false')
  })

  it.each([
    ['disponivel', 'Disponível'],
    ['pouco', 'Pouco'],
    ['esgotado', 'Esgotado'],
  ] as const)('StockPill %s mostra sempre texto, não só cor', (level, text) => {
    render(<StockPill level={level} />)
    expect(screen.getByText(text)).toHaveClass(`pill--${level}`)
  })

  it('SeasonTip é uma região com nome', () => {
    render(<SeasonTip title="Da época · novembro">Laranja e dióspiro</SeasonTip>)
    expect(screen.getByRole('complementary', { name: 'Dica da época' })).toHaveTextContent('Laranja e dióspiro')
  })
})
