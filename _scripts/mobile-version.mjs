// Zvýší verzi mobilní aplikace před release buildem (just mobile-aab,
// mobile-ipa, mobile-testflight).
//
// Obchody nepřijmou build se stejným číslem, jaké už mají, a na zvýšení
// v app.json se snadno zapomene – zjistí se to až po pětiminutovém buildu.
// iOS (`ios.buildNumber`) i Android (`android.versionCode`) mají jedno
// společné číslo buildu, takže je vždy jasné, který build patří ke které
// verzi.
//
// Volba „nic neměnit“ je pro druhou platformu téhož vydání: `just mobile-aab`
// číslo zvýší, následný `just mobile-ipa` ho už jen použije.
//
// Neinteraktivně (CI): LIBRITER_BUMP=build|patch|minor|major|none.
import { readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { createInterface } from 'node:readline/promises'
import { fileURLToPath } from 'node:url'

const file = join(dirname(fileURLToPath(import.meta.url)), '..', 'libriter-mobile', 'app.json')
const app = JSON.parse(readFileSync(file, 'utf8'))
const { expo } = app

const version = expo.version
const build = Math.max(Number(expo.ios.buildNumber) || 0, Number(expo.android.versionCode) || 0)
const [major, minor, patch] = version.split('.').map(Number)

const choices = [
  ['none', version, build, 'nic neměnit (druhá platforma téhož vydání)'],
  ['build', version, build + 1, 'jen build'],
  ['patch', `${major}.${minor}.${patch + 1}`, build + 1, 'patch'],
  ['minor', `${major}.${minor + 1}.0`, build + 1, 'minor'],
  ['major', `${major + 1}.0.0`, build + 1, 'major'],
]

let choice = process.env.LIBRITER_BUMP
if (!choice) {
  if (!process.stdin.isTTY) {
    console.error('Chybí terminál pro volbu verze – nastav LIBRITER_BUMP=build|patch|minor|major|none.')
    process.exit(1)
  }
  console.log(`Mobilní aplikace: verze ${version}, build ${build}`)
  choices.forEach(([, v, b, label], i) => console.log(`  ${i}) ${label.padEnd(10)} → ${v} (build ${b})`))
  const rl = createInterface({ input: process.stdin, output: process.stdout })
  const answer = await rl.question('Volba [1]: ').then(
    (a) => a.trim() || '1',
    () => process.exit(1), // Ctrl+C / Ctrl+D
  )
  rl.close()
  choice = choices[Number(answer)]?.[0] ?? answer
}

const picked = choices.find(([key]) => key === choice)
if (!picked) {
  console.error(`Neznámá volba: ${choice}`)
  process.exit(1)
}
const [, newVersion, newBuild] = picked

expo.version = newVersion
expo.ios.buildNumber = String(newBuild)
expo.android.versionCode = newBuild
writeFileSync(file, JSON.stringify(app, null, 2) + '\n')
console.log(`✓ Verze ${newVersion}, build ${newBuild} (libriter-mobile/app.json)`)
