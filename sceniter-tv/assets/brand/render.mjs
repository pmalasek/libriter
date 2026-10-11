// Vyrenderuje ikony a bannery TV aplikace z mark.svg do ../*.png.
// Spuštění: node render.mjs <cesta k @resvg/resvg-js> (viz brand-assets.sh).
import { readFileSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const { Resvg } = createRequire(process.argv[2] + '/')('@resvg/resvg-js')
const font = join(here, '../../node_modules/@expo-google-fonts/bricolage-grotesque/700Bold/BricolageGrotesque_700Bold.ttf')

const mark = readFileSync(join(here, 'mark.svg'), 'utf8')
const markInner = mark.replace(/^[\s\S]*?<svg[^>]*>/, '').replace(/<\/svg>\s*$/, '')

/** Tmavé pozadí s jemnou tyrkysovou září – stejné jako v aplikaci. */
function background(w, h) {
  return `
  <defs>
    <radialGradient id="glow" cx="0.3" cy="0.2" r="0.9">
      <stop offset="0" stop-color="#0E4F4F"/>
      <stop offset="0.55" stop-color="#0D1A1D"/>
      <stop offset="1" stop-color="#0B0B0F"/>
    </radialGradient>
  </defs>
  <rect width="${w}" height="${h}" fill="url(#glow)"/>`
}

function markAt(x, y, size) {
  return `<svg x="${x}" y="${y}" width="${size}" height="${size}" viewBox="0 0 32 32">${markInner}</svg>`
}

/** Značka + slovní logo vycentrované na ploše w×h; s = výška značky. */
function lockup(w, h, s) {
  const fontSize = s * 0.62
  const textW = fontSize * 3.92 // „Sceniter“ v Bricolage Bold
  const gap = s * 0.32
  const x0 = (w - (s + gap + textW)) / 2
  const y0 = (h - s) / 2
  return `${markAt(x0, y0, s)}
  <text x="${x0 + s + gap}" y="${h / 2 + fontSize * 0.36}" font-family="Bricolage Grotesque" font-weight="700" font-size="${fontSize}" fill="#F4F7F7" letter-spacing="${-fontSize * 0.02}">Sceniter</text>`
}

function svg(w, h, body) {
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">${body}</svg>`
}

function render(name, w, h, body) {
  const png = new Resvg(svg(w, h, body), { font: { fontFiles: [font], loadSystemFonts: false, defaultFontFamily: 'Bricolage Grotesque' } }).render().asPng()
  writeFileSync(join(here, '..', name), png)
  console.log(`  ${name} (${w}×${h})`)
}

// Ikona aplikace (Android TV launcher, obecná ikona Expo).
render('icon.png', 1024, 1024, markAt(0, 0, 1024))
// Spouštěcí obrazovka – značka na průhledném pozadí, barvu dává app.json.
render('splash.png', 512, 512, markAt(56, 56, 400))
// Android TV banner (launcher Google TV) 320×180.
render('tv-banner.png', 320, 180, background(320, 180) + lockup(320, 180, 52))
// tvOS: ikona aplikace (malá a velká) a Top Shelf.
for (const [name, w, h, s] of [
  ['tvos-icon.png', 1280, 768, 300],
  ['tvos-icon-small.png', 400, 240, 94],
  ['tvos-icon-small-2x.png', 800, 480, 188],
  ['tvos-topshelf.png', 1920, 720, 200],
  ['tvos-topshelf-2x.png', 3840, 1440, 400],
  ['tvos-topshelf-wide.png', 2320, 720, 200],
  ['tvos-topshelf-wide-2x.png', 4640, 1440, 400],
]) {
  render(name, w, h, background(w, h) + lockup(w, h, s))
}
