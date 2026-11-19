import { useState } from 'react'

import { IconMoon, IconSun } from '../ui/icons'
import { applyTheme, currentTheme, type Theme } from './theme'
import './ThemeToggle.css'

type Props = {
  /** "side" para a barra lateral e o cabeçalho verdes; "plain" para fundos claros. */
  tone?: 'side' | 'plain'
  /** Só o ícone (cabeçalho do telemóvel). */
  compact?: boolean
}

export function ThemeToggle({ tone = 'side', compact = false }: Props) {
  const [theme, setTheme] = useState<Theme>(currentTheme)
  const next: Theme = theme === 'dark' ? 'light' : 'dark'
  const label = next === 'dark' ? 'Mudar para o modo escuro' : 'Mudar para o modo claro'

  return (
    <button
      type="button"
      className={`theme-toggle theme-toggle--${tone}${compact ? ' theme-toggle--compact' : ''}`}
      aria-label={label}
      title={label}
      onClick={() => {
        applyTheme(next)
        setTheme(next)
      }}
    >
      {next === 'dark' ? <IconMoon /> : <IconSun />}
      {!compact && <span>{next === 'dark' ? 'Modo escuro' : 'Modo claro'}</span>}
    </button>
  )
}
