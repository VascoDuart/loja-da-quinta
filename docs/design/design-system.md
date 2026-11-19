# Design system · Horta Fresca

Decisão em [ADR-0013](../adr/0013-design-system-horta-fresca.md). Código em `web/src/shared/`.

## Princípios

1. **Verde orienta:** navegação, seleção e a ação principal. O resto é branco e cinza-esverdeado.
2. **Uma ação principal por ecrã** (botão verde). As outras são secundárias.
3. **Estado nunca só por cor:** cada etiqueta tem texto ("Disponível", "Pouco", "Esgotado").
4. **Tocável:** alvos com pelo menos 44 px (`--touch`).
5. **Preço sempre com unidade:** € / kg, € / L, € / dúzia, € / molho.
6. **Os componentes não sabem o tema:** só usam tokens.

## Tokens de cor

| Token | Claro | Escuro | Uso |
|---|---|---|---|
| `--color-page` | `#F6F8F4` | `#0F1511` | Fundo da página |
| `--color-surface` | `#FFFFFF` | `#171E19` | Cartões, painéis |
| `--color-image` | `#F1F4EF` | `#1E2620` | Fundo das imagens dos produtos |
| `--color-line` | `#DDE5DC` | `#2A342D` | Bordas e divisórias |
| `--color-text` | `#17251B` | `#E8EEE9` | Texto principal |
| `--color-muted` | `#4E5E53` | `#A3B0A6` | Texto secundário |
| `--color-green` | `#2F7D32` | `#2F7D32` | Ações, seleção |
| `--color-on-green` | `#FFFFFF` | `#FFFFFF` | Texto sobre verde |
| `--color-green-text` | `#1F5E23` | `#A7DDAA` | Texto verde, links |
| `--color-folha` | `#E3F1E1` | `#1C3121` | Destaques suaves (total, selecionado) |
| `--color-side` | `#2F7D32` | `#1E5225` | Barra lateral, cabeçalho |
| `--color-side-text` | `#E3F1E1` | `#CFE6D0` | Texto da barra lateral |
| `--color-side-active` | `#FFFFFF` | `#E3F1E1` | Item ativo do menu |
| `--color-tip` / `-text` | `#FDF0DF` / `#6B3405` | `#33260F` / `#F5C886` | Dica da época |
| `--color-ok` / `-text` | `#E3F1E1` / `#1F5E23` | `#1C3121` / `#A7DDAA` | Disponível |
| `--color-warn` / `-text` | `#FDF0DF` / `#8A4106` | `#3D2C14` / `#F5C27A` | Pouco |
| `--color-bad` / `-text` | `#FBE4E2` / `#9B1C14` | `#3F1E1B` / `#F4A7A0` | Esgotado |

Todos os pares de texto e fundo cumprem WCAG AA (4,5:1) nos dois modos.

## Tipografia, forma e espaço

| Grupo | Valores |
|---|---|
| Fonte | Plus Jakarta Sans (variável, servida localmente) |
| Tamanhos | `xs` 12 · `sm` 13 · `md` 15 · `lg` 17 · `xl` 28 · `2xl` 44 px |
| Pesos | 400 texto · 600 rótulos · 700 botões · 800 títulos e totais |
| Raios | `sm` 8 · `md` 12 · `lg` 14 px · `pill` |
| Espaço | escala de 4 px (`--space-1` a `--space-6`) |

## Componentes

| Componente | Ficheiro | Notas |
|---|---|---|
| `Button` | `shared/ui/ui.tsx` | `primary`, `secondary`, `ghost`; `type="button"` por omissão |
| `Chip` | `shared/ui/ui.tsx` | Filtro com `aria-pressed` |
| `StockPill` | `shared/ui/ui.tsx` | `disponivel`, `pouco`, `esgotado` |
| `SeasonTip` | `shared/ui/ui.tsx` | Região "Dica da época" |
| `ThemeToggle` | `shared/theme/ThemeToggle.tsx` | Troca e lembra o tema |
| Ícones | `shared/ui/icons.tsx` | Traço de 1,8 px, decorativos (`aria-hidden`) |

## Layouts

- **Backoffice:** barra lateral verde com letras brancas; item ativo em branco com texto verde; botão de tema no fundo. Em ecrãs estreitos, a barra passa para cima.
- **Público:** cabeçalho verde com a marca, "Entrar" e o botão de tema; conteúdo numa coluna de 48 rem.

## Tema

1. `<html data-theme="dark">` ou `"light"`: escolha manual, guardada no browser.
2. Sem atributo: segue `prefers-color-scheme`.
3. `initTheme()` corre antes do primeiro render, para não piscar.
