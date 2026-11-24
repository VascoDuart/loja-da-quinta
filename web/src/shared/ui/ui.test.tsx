import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Alert, Button, Chip, SeasonTip, StockPill, TextField } from './ui'

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

describe('formulários', () => {
  it('TextField liga a etiqueta e a ajuda ao campo', () => {
    render(<TextField label="E-mail" hint="O e-mail com que te registaram." type="email" />)
    const input = screen.getByLabelText('E-mail')
    expect(input).toHaveAttribute('type', 'email')
    expect(input).toHaveAccessibleDescription('O e-mail com que te registaram.')
  })

  it('Alert de erro é anunciado de imediato; o de informação, com calma', () => {
    render(
      <>
        <Alert>Falhou.</Alert>
        <Alert tone="info">A sessão terminou.</Alert>
      </>,
    )
    expect(screen.getByRole('alert')).toHaveTextContent('Falhou.')
    expect(screen.getByRole('status')).toHaveTextContent('A sessão terminou.')
  })
})
