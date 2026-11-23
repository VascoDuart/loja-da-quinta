import { describe, expect, it } from 'vitest'

import rawConfig from '../../vercel.json'

type Header = { key: string; value: string }
type Rewrite = { source: string; destination: string; has?: { type: string; value: string }[] }
type VercelConfig = {
  rewrites: Rewrite[]
  headers: { source: string; headers: Header[] }[]
}

const config = rawConfig as VercelConfig
const global = config.headers.find((h) => h.source === '/(.*)')?.headers ?? []
const header = (key: string) => global.find((h) => h.key === key)?.value ?? ''
const fallbackRule = config.rewrites.find((r) => r.destination === '/index.html')

describe('vercel.json (ADR-0007 e controlos de segurança)', () => {
  it('no domínio de staging, /api vai para a API de staging', () => {
    const staging = config.rewrites[0]
    expect(staging?.has).toEqual([{ type: 'host', value: 'loja-da-quinta-staging.vercel.app' }])
    expect(staging?.destination).toBe('https://loja-da-quinta-api-staging.onrender.com/api/:path*')
  })

  it('nos outros domínios, /api vai para a API de produção', () => {
    const prod = config.rewrites[1]
    expect(prod?.has).toBeUndefined()
    expect(prod?.destination).toBe('https://loja-da-quinta-api.onrender.com/api/:path*')
  })

  it('as regras de /api vêm antes do fallback da SPA', () => {
    const lastApi = config.rewrites.findLastIndex((r) => r.source === '/api/:path*')
    expect(lastApi).toBeLessThan(config.rewrites.indexOf(fallbackRule as Rewrite))
  })

  it('o fallback da SPA não apanha /api', () => {
    const fallback = new RegExp(`^${fallbackRule?.source ?? ''}$`)
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
