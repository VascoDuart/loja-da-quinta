import {
  type ButtonHTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  useId,
} from 'react'

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

/** Etiqueta curta com cor; o texto diz sempre o mesmo que a cor (WCAG 1.4.1). */
export function Pill({
  tone = 'neutral',
  children,
}: {
  tone?: 'ok' | 'warn' | 'bad' | 'neutral'
  children: ReactNode
}) {
  return <span className={`pill pill--${tone}`}>{children}</span>
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
  hint?: string | undefined
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

type Option<T extends string> = { value: T; label: string }

type RadioGroupProps<T extends string> = {
  legend: string
  name: string
  value: T
  options: readonly Option<T>[]
  onChange: (value: T) => void
  disabled?: boolean
  hint?: string | undefined
}

/** Escolha entre poucas opções, todas à vista (melhor que um select com duas). */
export function RadioGroup<T extends string>({
  legend,
  name,
  value,
  options,
  onChange,
  disabled,
  hint,
}: RadioGroupProps<T>) {
  const hintId = useId()
  return (
    <fieldset className="choice" disabled={disabled} aria-describedby={hint ? hintId : undefined}>
      <legend className="field__label">{legend}</legend>
      <div className="choice__options">
        {options.map((o) => (
          <label key={o.value} className="choice__option">
            <input
              type="radio"
              name={name}
              value={o.value}
              checked={value === o.value}
              onChange={() => onChange(o.value)}
            />
            {o.label}
          </label>
        ))}
      </div>
      {hint && (
        <span id={hintId} className="field__hint">
          {hint}
        </span>
      )}
    </fieldset>
  )
}

type CheckboxProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'id'> & {
  label: string
  hint?: string | undefined
}

/** Caixa de verificação com etiqueta clicável e área de toque de 44 px. */
export function Checkbox({ label, hint, ...props }: CheckboxProps) {
  const id = useId()
  const hintId = hint ? `${id}-hint` : undefined
  return (
    <div className="field">
      <label htmlFor={id} className="choice__option">
        <input id={id} type="checkbox" aria-describedby={hintId} {...props} />
        {label}
      </label>
      {hint && (
        <span id={hintId} className="field__hint">
          {hint}
        </span>
      )}
    </div>
  )
}

type SelectFieldProps = Omit<SelectHTMLAttributes<HTMLSelectElement>, 'id'> & {
  label: string
  options: readonly { value: string; label: string }[]
  hint?: string | undefined
}

/** Lista de escolha com etiqueta visível, para mais opções do que cabem num RadioGroup. */
export function SelectField({ label, options, hint, className = '', ...props }: SelectFieldProps) {
  const id = useId()
  const hintId = hint ? `${id}-hint` : undefined
  return (
    <div className={`field ${className}`.trim()}>
      <label htmlFor={id} className="field__label">
        {label}
      </label>
      <select id={id} className="field__input" aria-describedby={hintId} {...props}>
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
      {hint && (
        <span id={hintId} className="field__hint">
          {hint}
        </span>
      )}
    </div>
  )
}
