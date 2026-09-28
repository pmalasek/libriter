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
// Druhá oprava se týká běhu, ne kompilace. Nová architektura (bridgeless,
// `newArchEnabled=true`) čte `@ReactMethod` metody přes reflexi a odmítne
// každou asynchronní, která nevrací `void`: aplikace pak na Androidu
// zůstane na červené obrazovce „Unable to parse @ReactMethod annotations
// from native module: TrackPlayerModule … assumes returnType == void iff
// the method is synchronous“. Knihovna má 36 metod zapsaných jako
// `fun x(...) = scope.launch { … }`, a ty z pohledu Javy vracejí `Job`.
// Volání se proto přesměruje na privátní obal `launch { … }`, který vrací
// `Unit`; stejné jméno drží v platnosti `return@launch` uvnitř těl metod.
//
// Třetí oprava je v MusicService.kt: `emit`/`emitList` sahají pro React
// kontext na `reactNativeHost`, který v nové architektuře hází „You should
// not use ReactNativeHost directly in the New Architecture“ – aplikace
// spadne hned po stisku play (první událost PLAYBACK_STATE). Nahrazuje se
// vlastností `reactContext` z HeadlessJsTaskService, která zvládá obě
// architektury.
//
// Skript se pouští z `postinstall` v kořeni. Když knihovna chybí, je už
// spravená, nebo jde o verzi 5+ (ta novou architekturu podporuje sama),
// tiše skončí.
import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const pkg = join(root, 'node_modules/react-native-track-player/package.json')
if (!existsSync(pkg)) process.exit(0)
if (Number(JSON.parse(readFileSync(pkg, 'utf8')).version.split('.')[0]) >= 5) process.exit(0)

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

// `fun x(...) = scope.launch {` → `fun x(...) = launch {` (i přes zalomený
// řádek). Volání `scope.launch {` uvnitř těl (onServiceConnected apod.)
// nemají před sebou `=` a zůstávají.
const launchHelper = `    private val scope = MainScope()

    // Obal pro @ReactMethod: vrací Unit, takže metoda je pro React Native
    // \`void\`. Přímé \`scope.launch\` by vracelo Job a nová architektura by
    // modul odmítla. Doplněno skriptem _scripts/patch-track-player-android.mjs.
    private fun launch(block: suspend kotlinx.coroutines.CoroutineScope.() -> Unit) {
        scope.launch(block = block)
    }
`
const launchPattern = /=(\s*)scope\.launch \{/g
if (!source.includes('private fun launch(')) {
  const anchor = '    private val scope = MainScope()\n'
  if (!source.includes(anchor) || !launchPattern.test(source)) {
    console.warn('react-native-track-player: MusicModule.kt vypadá jinak, než se čekalo – obal launch přeskočen')
  } else {
    source = source.replace(anchor, launchHelper).replace(launchPattern, '=$1launch {')
    changed += 1
  }
}

if (changed > 0) {
  writeFileSync(file, source)
  console.log(`react-native-track-player: MusicModule.kt spraven (${changed}×)`)
}

const serviceFile = join(
  root,
  'node_modules/react-native-track-player/android/src/main/java',
  'com/doublesymmetry/trackplayer/service/MusicService.kt',
)
if (existsSync(serviceFile)) {
  const legacy = 'reactNativeHost.reactInstanceManager.currentReactContext'
  const service = readFileSync(serviceFile, 'utf8')
  if (service.includes(legacy)) {
    writeFileSync(serviceFile, service.replaceAll(legacy, 'reactContext'))
    console.log('react-native-track-player: MusicService.kt spraven')
  }
}
