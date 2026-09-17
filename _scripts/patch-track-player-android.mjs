// Spraví kompilaci react-native-track-player pro Android.
//
// `Track.originalItem` je v knihovně `Bundle?`, ale `Arguments.fromBundle()`
// má od React Native 0.86 parametr anotovaný jako nenullovatelný. Kotlin 2.1
// to už neodpustí a `assembleRelease` padá na
// „Argument type mismatch: actual type is 'Bundle?', but 'Bundle' was
// expected“ ve dvou místech MusicModule.kt.
//
// Knihovna to opravené nevydala (4.1.2 je poslední stabilní), takže se soubor
// upravuje po instalaci: null se propustí až do `callback.resolve()`, kde
// znamená „kapitola bez metadat“ – přesně to, co vrací i větev pro prázdnou
// frontu o pár řádků výš.
//
// Skript se pouští z `postinstall` v kořeni. Když knihovna chybí nebo je už
// spravená, tiše skončí.
import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const file = join(
  root,
  'node_modules/react-native-track-player/android/src/main/java',
  'com/doublesymmetry/trackplayer/module/MusicModule.kt',
)
if (!existsSync(file)) process.exit(0)

const patches = [
  {
    from: 'callback.resolve(Arguments.fromBundle(musicService.tracks[index].originalItem))',
    to: 'callback.resolve(musicService.tracks[index].originalItem?.let { Arguments.fromBundle(it) })',
  },
  {
    from: `else Arguments.fromBundle(
                musicService.tracks[musicService.getCurrentTrackIndex()].originalItem
            )`,
    to: `else musicService.tracks[musicService.getCurrentTrackIndex()]
                .originalItem?.let { Arguments.fromBundle(it) }`,
  },
]

let source = readFileSync(file, 'utf8')
let changed = 0
for (const { from, to } of patches) {
  if (source.includes(to)) continue
  if (!source.includes(from)) {
    console.warn('react-native-track-player: MusicModule.kt vypadá jinak, než se čekalo – přeskočeno')
    continue
  }
  source = source.replace(from, to)
  changed += 1
}

if (changed > 0) {
  writeFileSync(file, source)
  console.log(`react-native-track-player: MusicModule.kt spraven (${changed}×)`)
}
