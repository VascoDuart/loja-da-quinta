/**
 * Tema da interface (ADR-0013).
 *
 * Por omissão segue o sistema (prefers-color-scheme). Se a pessoa escolher
 * à mão, a escolha fica no atributo data-theme do <html> e é lembrada neste
 * browser. É só uma preferência visual: nunca guardar aqui dados de sessão.
 */
export type Theme = 'light' | 'dark'

const KEY = 'loja-da-quinta:tema'

function storage(): Storage | undefined {
  try {
    // eslint-disable-next-line no-restricted-globals -- só a preferência de tema, sem dados sensíveis
    return localStorage
  } catch {
    return undefined // modo privado ou armazenamento bloqueado
  }
}

export function systemTheme(): Theme {
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function savedTheme(): Theme | undefined {
  try {
    const v = storage()?.getItem(KEY)
    return v === 'light' || v === 'dark' ? v : undefined
  } catch {
    return undefined
  }
}

export function currentTheme(): Theme {
  const attr = document.documentElement.dataset.theme
  if (attr === 'light' || attr === 'dark') return attr
  return systemTheme()
}

export function applyTheme(theme: Theme): void {
  document.documentElement.dataset.theme = theme
  try {
    storage()?.setItem(KEY, theme)
  } catch {
    // sem armazenamento: o tema vale só nesta página
  }
}

/** Aplica a escolha guardada antes do primeiro render, para não piscar. */
export function initTheme(): void {
  const saved = savedTheme()
  if (saved) document.documentElement.dataset.theme = saved
}
