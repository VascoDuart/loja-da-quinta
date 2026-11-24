import { type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode, useId } from 'react'

import './ui.css'

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'ghost'
  size?: 'md' | 'lg'
}

/** Botão do design system. "primary" é a ação principal de cada ecrã (só uma). */
export function Button({ variant = 'primary', size = 'md', className = '', type = 'button', ...props }: ButtonProps) {
  return <button type={type} className={`btn btn--${variant} btn--${size} ${className}`.trim()} {...props} />
}

type ChipProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'aria-pressed'> & { selected: boolean }

/** Filtro que se liga e desliga (ex.: categoria). */
export function Chip({ selected, className = '', ...props }: ChipProps) {
  return <button type="button" aria-pressed={selected} className={`chip ${className}`.trim()} {...props} />
}

export type StockLevel = 'disponivel' | 'pouco' | 'esgotado'

const stockLabel: Record<StockLevel, string> = {
  disponivel: 'Disponível',
  pouco: 'Pouco',
  esgotado: 'Esgotado',
}

/** Nível de stock: a cor acompanha sempre o texto (WCAG 1.4.1). */
export function StockPill({ level }: { level: StockLevel }) {
  return <span className={`pill pill--${level}`}>{stockLabel[level]}</span>
}

/** Dica da época: destaque discreto em âmbar. */
export function SeasonTip({ title, children, image }: { title: string; children: ReactNode; image?: ReactNode }) {
  return (
    <aside className="season-tip" aria-label="Dica da época">
      <div className="season-tip__text">
        <span className="season-tip__eyebrow">{title}</span>
        <span className="season-tip__body">{children}</span>
      </div>
      {image}
    </aside>
  )
}

type TextFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'id'> & {
  label: string
  /** Texto de ajuda por baixo do campo. */
  hint?: string
}

/** Campo de texto com etiqueta visível (WCAG 3.3.2) e altura de toque de 44 px. */
export function TextField({ label, hint, className = '', ...props }: TextFieldProps) {
  const id = useId()
  const hintId = hint ? `${id}-hint` : undefined
  return (
    <div className={`field ${className}`.trim()}>
      <label htmlFor={id} className="field__label">
        {label}
      </label>
      <input id={id} className="field__input" aria-describedby={hintId} {...props} />
      {hint && (
        <span id={hintId} className="field__hint">
          {hint}
        </span>
      )}
    </div>
  )
}

/**
 * Mensagem dentro da página. "error" é anunciada logo pelos leitores de
 * ecrã (role="alert"); "info" é anunciada quando houver uma pausa.
 */
export function Alert({ tone = 'error', children }: { tone?: 'error' | 'info'; children: ReactNode }) {
  return (
    <div role={tone === 'error' ? 'alert' : 'status'} className={`alert alert--${tone}`}>
      {children}
    </div>
  )
}
