// Shoda barevných témat webu (core-web/src/styles/base.css) s jejich
// JS podobou (core-shared/src/themes.ts), ze které počítá barvy mobil
// a televize. Bez testu by se dvě kopie čísel časem rozešly.
//
//   npm test -w core-shared

import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { describe, it } from 'node:test'
import { fileURLToPath } from 'node:url'
import { COLOR_SCHEMES, parseScheme } from '../src/appearance'
import { THEMES, THEME_TOKENS, themeCssVar, type ResolvedThemeMode } from '../src/themes'
import type { ColorScheme } from '../src/types'

const css = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'core-web', 'src', 'styles', 'base.css'),
  'utf8',
)

/** Proměnné --t-* z bloku, jehož selektor přesně odpovídá zadanému. */
function block(selector: string): Map<string, string> {
  const re = /([^{}]+)\{([^{}]*)\}/g
  for (let m = re.exec(css); m; m = re.exec(css)) {
    const selectors = m[1]
      .replace(/\/\*[\s\S]*?\*\//g, '')
      .split(',')
      .map((s) => s.trim())
    if (!selectors.includes(selector)) continue
    const vars = new Map<string, string>()
    for (const decl of m[2].matchAll(/(--t-[\w-]+)\s*:\s*([^;]+);/g)) vars.set(decl[1], decl[2].trim())
    if (vars.size > 0) return vars
  }
  throw new Error(`blok ${selector} v base.css chybí`)
}

function selectorFor(scheme: ColorScheme, mode: ResolvedThemeMode): string {
  return mode === 'dark' ? `:root.dark[data-color-scheme="${scheme}"]` : `:root[data-color-scheme="${scheme}"]`
}

describe('barevná témata', () => {
  for (const { value: scheme } of COLOR_SCHEMES) {
    for (const mode of ['light', 'dark'] as const) {
      it(`${scheme} ${mode}: base.css = themes.ts`, () => {
        const vars = block(selectorFor(scheme, mode))
        for (const token of THEME_TOKENS) {
          const name = themeCssVar(token)
          assert.equal(vars.get(name), THEMES[scheme][mode][token].join(' '), name)
        }
        assert.equal(vars.size, THEME_TOKENS.length, 'v base.css je navíc proměnná, kterou themes.ts nezná')
      })
    }
  }

  it('výchozí blok je Midnight', () => {
    assert.deepEqual(block(':root'), block(selectorFor('midnight', 'light')))
    assert.deepEqual(block(':root.dark'), block(selectorFor('midnight', 'dark')))
  })

  it('stará schémata se převedou na nová', () => {
    assert.equal(parseScheme('teal'), 'midnight')
    assert.equal(parseScheme('blue'), 'midnight')
    assert.equal(parseScheme('green'), 'emerald')
    assert.equal(parseScheme('violet'), 'burgundy')
    assert.equal(parseScheme('emerald'), 'emerald')
    assert.equal(parseScheme(null), 'midnight')
    assert.equal(parseScheme('nesmysl'), 'midnight')
  })
})
