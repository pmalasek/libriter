# libriter-mobile

Mobilní přehrávač Libriteru (iOS + Android, React Native / Expo).

Aplikace je **jen přehrávač**: přihlášení, knihovna, detail knihy, přehrávač,
stažené knihy a nastavení. Správa knihovny – úpravy knih, metadata, pořadí
kapitol, administrace – zůstává výhradně ve webovém rozhraní.

## Jak to funguje

**Rozhraní čte vždycky z lokální SQLite databáze**, nikdy přímo ze sítě. Díky
tomu vypadá aplikace v letadle stejně jako doma na Wi-Fi a nikde v kódu není
větev „co ukázat, když není signál“. Server je synchronizační partner, ne
zdroj pravdy pro vykreslení.

Poslech se ukládá do fronty (`pending_events`) i tehdy, když je telefon
online. Jedna cesta ven znamená, že se pozice nemůže ztratit mezi dvěma
větvemi kódu; odeslání obstará `src/sync/syncEngine.ts` dávkově přes
`POST /sessions/sync`. Každá událost má vlastní UUID, takže opakovaně poslaná
dávka (ztracená odpověď, restart) v deníku poslechu minuty nezdvojí.

| Soubor | Co dělá |
|--------|---------|
| `src/db/schema.ts` | Schéma lokální databáze a migrace |
| `src/db/library.ts` | Zrcadlo knihovny a poslechů |
| `src/db/events.ts` | Fronta pozic čekajících na odeslání |
| `src/sync/syncEngine.ts` | Odeslání fronty, stažení zrcadla, backoff |
| `src/downloads/downloadManager.ts` | Stahování knih po kapitolách |
| `src/player/PlayerProvider.tsx` | Přehrávač nad react-native-track-player |
| `src/player/service.ts` | Ovládání ze zamčené obrazovky |

Pravidla poslechu (interval zápisu, počítání odposlouchaných sekund, rychlosti)
jsou v `libriter-shared`, aby se web a mobil nemohly rozejít – jinak by se
rozešel i deník poslechu.

## Server musí být aktualizovaný

Aplikace stojí na endpointech `POST /auth/mobile-token` a `POST /sessions/sync`
a na poli `size_bytes` v kapitolách. Proti staršímu serveru přihlášení projde,
ale výměna za mobilní token skončí na `404` – aplikace to pozná a napíše, že
server mobilní aplikaci nepodporuje. Řešením je nasadit backend z tohohle
repozitáře (migrace 012 a 013 se aplikují samy při startu).

## Vývoj

Závislosti se instalují **v kořeni repozitáře** (npm workspaces):

```bash
npm install
```

Aplikace potřebuje **dev build**, ne Expo Go: react-native-track-player je
nativní modul, který Expo Go neobsahuje.

```bash
just mobile-ios       # npx expo run:ios --device
just mobile-android   # npx expo run:android --device
just mobile-start     # Metro pro už nainstalovaný dev build
```

Kontrola typů: `just typecheck` (projede shared, web i mobil).

## Vlastní config plugin: UIScene

Nejnovější iOS SDK aplikaci bez scénového životního cyklu při startu vůbec
nepustí. Expo SDK 58 už projekt generuje se `SceneDelegate`, SDK 57 – na kterém
tahle aplikace stojí – ještě ne, ačkoliv třídu `ExpoAppSceneDelegate` v balíčku
`expo` má. Chybějící kousek doplňuje
[`plugins/withIosSceneLifecycle.js`](plugins/withIosSceneLifecycle.js): scene
manifest v `Info.plist`, `SceneDelegate.swift` zařazený do Xcode projektu a
úprava `AppDelegate`.

Po přechodu na SDK 58 se plugin i jeho zápis v `app.json` můžou smazat.

## Dvě verze Reactu

Web vyžaduje React ≥ 19.2.7 (react-router), Expo SDK si pinuje 19.2.3. Jedna
společná verze tedy nejde a obě leží vedle sebe: kořenové `node_modules` mají
verzi webu, `libriter-mobile/node_modules` verzi Expa.

Do bundlu se dostane právě jedna – zařizuje to `metro.config.js`: hledání
balíčků je zúžené na dvě cesty v pevném pořadí a hierarchické prohledávání
nadřazených adresářů je vypnuté, takže `require('react')` z čehokoliv skončí
u verze, kterou má u sebe mobil. `npx expo-doctor` proto hlásí dvě varování
(duplicitní React, upravený Metro config); jsou to dvě strany téhož vědomého
rozhodnutí.

## Známá varování při startu

react-native-track-player hlásí čtyři varování o metodách časovače spánku,
které v nativním modulu na iOS nejsou. Aplikace je nepoužívá – časovač vypnutí
je vlastní, v JavaScriptu (`src/components/SleepTimer.tsx`).

## Distribuce

Nativní projekty (`ios/`, `android/`) se generují z konfigurace a do gitu
nepatří:

```bash
npx expo prebuild                 # vygeneruje ios/ i android/
```

**iOS / TestFlight** – v Xcode otevřít `ios/libritermobile.xcworkspace`,
nastavit tým a podepsání, `Product → Archive`, pak `Distribute App →
App Store Connect`.

**Android / APK** – `just mobile-apk` spustí prebuild a `gradlew
assembleRelease`; výsledek je v
`android/app/build/outputs/apk/release/app-release.apk`. Pro instalaci mimo
Play Store je potřeba podepisovací klíč v `android/app/build.gradle`.
