import { describe, expect, it } from 'vitest'

import rawConfig from '../../vercel.json'

type Header = { key: string; value: string }
type VercelConfig = {
  rewrites: { source: string; destination: string }[]
  headers: { source: string; headers: Header[] }[]
}

const config = rawConfig as VercelConfig
const global = config.headers.find((h) => h.source === '/(.*)')?.headers ?? []
const header = (key: string) => global.find((h) => h.key === key)?.value ?? ''

describe('vercel.json (ADR-0007 e controlos de segurança)', () => {
  it('encaminha /api para a API antes do fallback da SPA', () => {
    expect(config.rewrites[0]?.source).toBe('/api/:path*')
    expect(config.rewrites[0]?.destination).toMatch(/^https:\/\/[^/]+\.onrender\.com\/api\/:path\*$/)
    expect(config.rewrites[1]?.destination).toBe('/index.html')
  })

  it('o fallback da SPA não apanha /api', () => {
    const fallback = new RegExp(`^${config.rewrites[1]?.source ?? ''}$`)
    expect(fallback.test('/app/venda')).toBe(true)
    expect(fallback.test('/api/v1/healthz')).toBe(false)
  })

  it('CSP só permite a própria origem e não aceita inline', () => {
    const csp = header('Content-Security-Policy')
    for (const d of [
      "default-src 'self'",
      "script-src 'self'",
      "connect-src 'self'",
      "frame-ancestors 'none'",
      "object-src 'none'",
    ]) {
      expect(csp).toContain(d)
    }
    expect(csp).not.toContain('unsafe-inline')
    expect(csp).not.toContain('unsafe-eval')
  })

  it.each([
    'Strict-Transport-Security',
    'X-Content-Type-Options',
    'X-Frame-Options',
    'Referrer-Policy',
    'Permissions-Policy',
  ])('envia %s', (key) => {
    expect(header(key)).not.toBe('')
  })
})
