import { describe, expect, it } from 'vitest'
import { en, namespaces, pl } from './resources.ts'

// FR-1310: polskie i angielskie pliki mają te same klucze i te same
// parametry. Liczba mnoga to sufiksy i18next: po polsku _one/_few/_many/_other,
// po angielsku _one/_other — porównujemy klucz bazowy.

type Tree = { [key: string]: string | Tree }

const pluralSuffix = /_(zero|one|two|few|many|other)$/

function flatten(tree: Tree, prefix = ''): Map<string, string> {
  const out = new Map<string, string>()
  for (const [key, value] of Object.entries(tree)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof value === 'string') {
      out.set(path, value)
    } else {
      for (const [k, v] of flatten(value, path)) {
        out.set(k, v)
      }
    }
  }
  return out
}

/** Klucz bazowy → zbiór form liczby mnogiej (pusty dla zwykłego klucza). */
function baseKeys(flat: Map<string, string>): Map<string, Set<string>> {
  const out = new Map<string, Set<string>>()
  for (const key of flat.keys()) {
    const match = pluralSuffix.exec(key)
    const base = match ? key.slice(0, -match[0].length) : key
    const forms = out.get(base) ?? new Set<string>()
    if (match?.[1]) {
      forms.add(match[1])
    }
    out.set(base, forms)
  }
  return out
}

function params(text: string): string[] {
  return [...text.matchAll(/\{\{\s*([\w.]+)\s*(?:,[^}]*)?\}\}/g)].map((m) => m[1]!).sort()
}

/** Parametry klucza bazowego ze wszystkich jego form (formy mogą pomijać `count`). */
function paramsOf(flat: Map<string, string>, base: string): string[] {
  const all = new Set<string>()
  for (const [key, value] of flat) {
    if (key === base || (key.startsWith(`${base}_`) && pluralSuffix.test(key))) {
      params(value).forEach((p) => all.add(p))
    }
  }
  return [...all].sort()
}

describe.each(namespaces)('namespace %s', (ns) => {
  const plFlat = flatten(pl[ns] as Tree)
  const enFlat = flatten(en[ns] as Tree)
  const plBase = baseKeys(plFlat)
  const enBase = baseKeys(enFlat)

  it('has the same keys in pl and en', () => {
    expect([...enBase.keys()].sort()).toEqual([...plBase.keys()].sort())
  })

  it('has the same interpolation parameters in pl and en', () => {
    for (const base of plBase.keys()) {
      expect({ key: base, params: paramsOf(enFlat, base) }).toEqual({ key: base, params: paramsOf(plFlat, base) })
    }
  })

  it('has every plural form each language needs', () => {
    for (const [base, forms] of plBase) {
      if (forms.size > 0) {
        expect({ key: base, forms: [...forms].sort() }).toEqual({ key: base, forms: ['few', 'many', 'one', 'other'] })
        expect({ key: base, forms: [...(enBase.get(base) ?? [])].sort() }).toEqual({ key: base, forms: ['one', 'other'] })
      }
    }
  })

  it('has no empty translations', () => {
    for (const [key, value] of [...plFlat, ...enFlat]) {
      expect({ key, value: value.trim() === '' }).toEqual({ key, value: false })
    }
  })
})
