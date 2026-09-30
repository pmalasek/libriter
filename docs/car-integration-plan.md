# Libriter mobile → Android Auto + Apple CarPlay

## Kontext

Libriter má v autě zatím jen to, co dá systém zadarmo: react-native-track-player
(RNTP, `5.0.0-alpha0-nightly-359af5a…`) publikuje media session, takže **ovládání
právě hrající knihy** (play/pauza, ±skoky, seek) funguje přes Bluetooth, na zamčené
obrazovce a částečně i v Android Auto. Aby se ale aplikace v autě **objevila jako
samostatná aplikace s knihovnou** (vybrat rozposlouchanou knihu, stažené knihy…),
chybí tohle:

| Oblast | Stav dnes |
|---|---|
| Android Auto: deklarace v manifestu (`automotive_app_desc.xml`) | chybí → AA aplikaci nevypíše |
| Android Auto: strom knihovny (`onGetLibraryRoot/onGetChildren/onGetItem/onSetMediaItems`) | RNTP je nemá implementované ([MusicService.kt](../node_modules/react-native-track-player/android/src/main/java/com/doublesymmetry/trackplayer/service/MusicService.kt) `InnerMediaSessionCallback`) → prázdný seznam, z auta nejde nic spustit |
| CarPlay: entitlement `com.apple.developer.carplay-audio` | chybí, nutno požádat Apple |
| CarPlay: scéna `CPTemplateApplicationScene` + šablony | chybí; UIScene cyklus už ale máme ([withIosSceneLifecycle.js](../libriter-mobile/plugins/withIosSceneLifecycle.js)) – to je dobrý základ |
| Spuštění přehrávání bez UI | **hlavní překážka**: `playBook`/`openSession`/`load`, ukládání pozice a počítání poslechu žijí v React kontextu [PlayerProvider.tsx](../libriter-mobile/src/player/PlayerProvider.tsx#L206-L285). Auto často startuje aplikaci bez obrazovky (headless) → nic by se nepřehrálo ani neuložilo |
| Token na zamčeném telefonu | ✅ už řešeno – `AFTER_FIRST_UNLOCK` v [storage.ts](../libriter-mobile/src/auth/storage.ts) |
| Now playing metadata, Speech/SpokenAudio kategorie | ✅ už nastaveno v [setup.ts](../libriter-mobile/src/player/setup.ts) |

`android/` je v `.gitignore` (prebuild) → všechny nativní změny musí jít přes
config pluginy v `plugins/`, stejně jako stávající `withIosSceneLifecycle`.

---

## Krok 0 – Administrativa (spustit hned, běží dlouho)

1. **Apple CarPlay entitlement** – požádat na developer.apple.com/contact/carplay
   (kategorie *Audio*). Schválení trvá dny až týdny. Po schválení přidat
   entitlement do App ID `cz.libriter.app` a přegenerovat provisioning profil
   (`eas credentials`).
2. **Google Play** – v Play Console zapnout formát *Android Auto*
   (Nastavení → Pokročilá nastavení → Typy vydání). Každé vydání pak prochází
   kontrolou kvality Android Auto (driver distraction guidelines).

## Krok 1 – Vytáhnout přehrávání z Reactu (společné pro obě platformy)

Nový modul `src/player/controller.ts` (bez Reactu), do něj přesunout z `PlayerProvider`:
- `load` (sestavení fronty, token pro stream, lokální soubory, seek, rate),
- `openSession`, `playBook`, `switchSession`, `step` (kapitoly),
- stav „co hraje" (session/book/chapters/chapter) jako malý store s `subscribe()`,
- počítání `listened` sekund a `savePosition` z [positionSaver.ts](../libriter-mobile/src/player/positionSaver.ts) navázané na
  `Event.PlaybackProgressUpdated` / `PlaybackActiveTrackChanged` / `PlaybackQueueEnded`
  – registrovat v [service.ts](../libriter-mobile/src/player/service.ts), aby běželo i headless.

`PlayerProvider` se zúží na tenkou vrstvu: odebírá store controlleru do React
stavu a řeší jen UI věci (toasty, `Alert` dialogy, `ContinueSeriesModal`,
invalidace react-query). Místa s `confirmPlayable(..., interactive)` volat z auta
s `silent` (nestažená kniha bez sítě se v autě jen nenabídne/neprovede).

Pozor: obnova stavu po restartu Reactu ([PlayerProvider.tsx:293](../libriter-mobile/src/player/PlayerProvider.tsx#L293)) pak čte ze
store controlleru místo z `getActiveTrack()` hacku – zjednoduší se.

## Krok 2 – Katalog pro auto (společný JS)

`src/car/catalog.ts` – jedna funkce `getChildren(parentId)` vracející uzly
`{ id, title, subtitle, artworkUri, playable, browsable }`. Data jen z lokální
SQLite ([db/library.ts](../libriter-mobile/src/db/library.ts), [db/downloads.ts](../libriter-mobile/src/db/downloads.ts)) + `withSource` jako fallback:

```
/ (root, max 4 záložky – limit AA i CarPlay tab baru)
├─ continue   „Pokračovat"   listSessions()  → playable (session id)
├─ downloads  „Stažené"      listDownloads() complete → playable (book id)
├─ series     „Série"        listSeries()    → browsable → knihy série
└─ books      „Knihy"        listBooks() podle abecedy / nedávné, limit ~100
```

ID uzlů s prefixem (`session:…`, `book:…`, `series:…`), aby šlo z ID poznat akci.
Hloubka max 3–4 úrovně (pravidla AA). Texty přes i18n (`mobile.car.*` klíče
v `libriter-shared` pro všechny jazyky). Nepřihlášený stav → jeden uzel
„Přihlaste se v telefonu".

Obaly: AA i CarPlay chtějí obrázek bez autorizační hlavičky. Ověřit, jestli
`/books/:id/cover` z backendu jde načíst bez tokenu; když ne, stahovat obaly
do cache (`expo-file-system`) a předávat `file://`/`content://` URI.

## Krok 3 – Android Auto

1. **Config plugin `plugins/withAndroidAuto.js`**:
   - `res/xml/automotive_app_desc.xml` s `<automotiveApp><uses name="media"/></automotiveApp>`,
   - do `<application>` meta-data `com.google.android.gms.car.application` → `@xml/automotive_app_desc`,
   - volitelně `com.google.android.gms.car.notification.SmallIcon` (monochromatická ikona).
   - Zapsat do `app.json` `plugins`.
2. **Browse strom v RNTP** – `patch-package` (přidat do devDependencies +
   `postinstall`) nad `MusicService.kt` / `MusicModule.kt` / `src/trackPlayer.ts`:
   - nová nativní metoda `setBrowseTree(map<parentId, MediaItem[]>)` – JS
     předplní strom z `catalog.ts` (po startu, po syncu, po změně stažení),
   - `onGetLibraryRoot` → kořen s `EXTRAS_KEY_CONTENT_STYLE_*` (grid pro obaly),
   - `onGetChildren` / `onGetItem` → čte z mapy,
   - `onSetMediaItems` / `onAddMediaItems` (výběr položky v autě) → emit nové
     události `RemotePlayId { id }` do JS a vrátit aktuální frontu; v
     [service.ts](../libriter-mobile/src/player/service.ts) ji obsloužit voláním `controller.playFromCarId(id)`,
   - volitelně `onSearch`/`onGetSearchResult` → `RemotePlaySearch` (hlasové
     „Ok Google, pusť … v Libriteru") – může být až v druhé vlně.
   - Alternativa: přejít na fork lovegaoshi/react-native-track-player
     (má `setBrowseTree` a `RemotePlayId` hotové) – ověřit kompatibilitu s RN 0.86
     / novou architekturou; pokud sedí, ušetří to vlastní patch.
3. **Headless start** – RNTP při připojení `com.google.android.projection.gearhead`
   startuje službu i headless JS; zajistit, že `service.ts` zavolá
   `ensurePlayer()`, načte katalog a zaregistruje controller.
4. **Ovládání v autě**: custom layout už má ±skoky (`JUMP_BACKWARD/FORWARD`);
   přidat vlastní akci pro rychlost přehrávání (volitelné).

## Krok 4 – CarPlay

1. **Entitlement** (`app.json` → `ios.entitlements`:
   `"com.apple.developer.carplay-audio": true`) – až po schválení z kroku 0,
   jinak build selže při podpisu.
2. **Nativní část – lokální Expo modul `modules/libriter-carplay`** (Swift):
   - `CarPlaySceneDelegate: CPTemplateApplicationSceneDelegate`,
   - kořen `CPTabBarTemplate` se 4 `CPListTemplate` podle katalogu,
     `CPListItem` s obalem, `handler` → posílá JS událost `onSelect(id)`,
   - `CPNowPlayingTemplate.shared` (bere data z `MPNowPlayingInfoCenter` a
     `MPRemoteCommandCenter`, které RNTP už plní) + tlačítko rychlosti
     `CPNowPlayingPlaybackRateButton`,
   - JS API: `setTree(nodes)`, `onSelect`, `onConnect/onDisconnect`.
   - Alternativa: knihovna `react-native-carplay` – ověřit podporu RN 0.86 /
     new architecture a scénového cyklu; vlastní modul je malý (jen list šablony),
     proto ho doporučuji jako výchozí volbu.
3. **Rozšířit `withIosSceneLifecycle.js`** (nebo nový `withCarPlay.js`) –
   do `UIApplicationSceneManifest.UISceneConfigurations` přidat roli
   `CPTemplateApplicationSceneSessionRoleApplication` se
   `UISceneDelegateClassName` CarPlay delegáta; `UIApplicationSupportsMultipleScenes` zůstává `false`
   (CarPlay scéna se nepočítá).
4. **Start Reactu bez telefonní scény** – dnes RN spouští až
   `ExpoAppSceneDelegate` telefonní scény. Když uživatel pustí aplikaci z
   CarPlay při zamčeném telefonu, telefonní scéna nevznikne → JS neběží.
   CarPlay delegát musí při `didConnect` zajistit start React Native
   factory bez okna (přes `ExpoReactNativeFactoryProvider`, který AppDelegate
   už poskytuje) a teprve pak volat JS. **Nejrizikovější bod – ověřit jako první prototyp.**
5. Stejná JS obsluha výběru jako u Androidu: `controller.playFromCarId(id)`.

## Krok 5 – Testování

- **Android Auto**: Desktop Head Unit (DHU z Android SDK → Extras), v aplikaci
  Android Auto zapnout vývojářský režim + „Neznámé zdroje", spustit
  `adb forward tcp:5277 tcp:5277` a `desktop-head-unit`. Scénáře: studený start
  z auta (aplikace zabitá), výběr rozposlouchané knihy, stažená kniha
  offline, nepřihlášený uživatel, pozice se uloží a na webu je vidět.
- **CarPlay**: potřeba Mac s Xcode – Simulator → I/O → External Displays →
  CarPlay (dev build přes `expo run:ios` / EAS development build). Stejné
  scénáře + start jen z CarPlay se zamčeným telefonem. Nakonec reálné auto /
  CarPlay jednotka.
- `npm run typecheck` po refaktoru controlleru; ruční regrese přehrávače v
  telefonu (přehrání, přepínání poslechů, další díl série, dotaz na smazání
  staženého, sleep timer, obnova po zabití Activity).

## Doporučené pořadí

1. Krok 0 (žádosti) – hned.
2. Krok 1 (controller) – samostatný PR, bez viditelné změny chování.
3. Krok 2 + 3 (Android Auto) – lze celé otestovat na Linuxu s DHU.
4. Krok 4 (CarPlay) – až dorazí entitlement; začít prototypem bodu 4.4.
