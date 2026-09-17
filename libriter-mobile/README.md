# libriter-mobile

Mobilní přehrávač Libriteru (iOS + Android, React Native / Expo).

Aplikace zrcadlí **čtecí část webu**: Domů, Knihy, Autoři, Série, Právě
posloucháno, detaily knihy / autora / série, přehrávač – se stejnými
komponentami, řazením, hledáním a vzhledem (barevné schéma a světlý/tmavý
režim z profilu, fonty Bricolage Grotesque a Geist). Správa knihovny – úpravy
knih, metadata, pořadí kapitol, administrace – zůstává výhradně ve webovém
rozhraní. Navíc oproti webu umí knihy stahovat do telefonu.

## Dva režimy

Přepínač je v Nastavení; výchozí je **online**.

| Režim | Data | Kdy |
|-------|------|-----|
| **Online** | Čtou se živě ze serveru, jako na webu (react-query nad `apiFetch`). Lokální databáze drží jen stažené knihy a poslechy, kterých se dotkl přehrávač. Bez signálu se sáhne do ní – stažené knihy jdou poslouchat vždycky. | Doma, na Wi-Fi, běžně. |
| **Offline** | Celá knihovna (knihy, autoři, série, kapitoly, poslechy, stav knih) se zrcadlí do SQLite a rozhraní čte z ní. Zapnutí spustí první plné stažení (SyncBadge ukazuje průběh). | Před cestou, v letadle, na horách. |

O tom, odkud data přijdou, rozhoduje jediné místo: `src/data/sources.ts`
(`withSource`). Obrazovky používají hooky z `src/data/hooks.ts` – pojmenované
stejně jako na webu (`useBooks`, `useAuthors`, `useSeriesList`, `useSessions`,
`useBookProgress`, …) – a o režimu nevědí.

## Jak to funguje

Poslech se ukládá do fronty (`pending_events`) v obou režimech. Jedna cesta
ven znamená, že se pozice nemůže ztratit mezi dvěma větvemi kódu; odeslání
obstará `src/sync/syncEngine.ts` dávkově přes `POST /sessions/sync` – v online
režimu do dvou sekund, offline po připojení. Každá událost má vlastní UUID,
takže opakovaně poslaná dávka v deníku poslechu minuty nezdvojí.

Poslech se ukládá do fronty (`pending_events`) i tehdy, když je telefon
online. Jedna cesta ven znamená, že se pozice nemůže ztratit mezi dvěma
větvemi kódu; odeslání obstará `src/sync/syncEngine.ts` dávkově přes
`POST /sessions/sync`. Každá událost má vlastní UUID, takže opakovaně poslaná
dávka (ztracená odpověď, restart) v deníku poslechu minuty nezdvojí.

| Soubor | Co dělá |
|--------|---------|
| `src/data/mode.ts`, `ModeProvider.tsx` | Režim online / offline |
| `src/data/sources.ts` | `LibrarySource`: server a lokální DB, výběr podle režimu |
| `src/data/hooks.ts` | Hooky obrazovek (názvy jako na webu) |
| `src/data/listPrefs.ts` | Předvolby seznamů (zobrazení, řazení) |
| `src/db/schema.ts` | Schéma lokální databáze a migrace |
| `src/db/library.ts` | Zrcadlo knih, kapitol, autorů, sérií, poslechů a stavu knih |
| `src/db/events.ts` | Fronta pozic čekajících na odeslání |
| `src/sync/syncEngine.ts` | Odeslání fronty; v offline režimu stažení zrcadla; backoff |
| `src/downloads/downloadManager.ts` | Stahování knih po kapitolách |
| `src/player/PlayerProvider.tsx` | Přehrávač nad react-native-track-player – akce jako na webu |
| `src/player/sessions.ts` | Založení poslechu (kniha / série / seznam), offline provizorně |
| `src/player/service.ts` | Ovládání ze zamčené obrazovky |
| `src/theme/palette.ts` | Paleta z týchž OKLCH hodnot jako `index.css` webu |
| `src/theme/ThemeProvider.tsx` | Vzhled z profilu (`color_scheme`, `theme_mode`) |
| `src/components/` | `BookCard`, `BookGrid`, `Shelf`, `SeriesCoverStack`, … – protějšky webu |

Pravidla poslechu (interval zápisu, počítání odposlouchaných sekund, rychlosti),
formátování, řazení, seznam barevných schémat i odvození stavu knihy jsou
v `libriter-shared`, aby se web a mobil nemohly rozejít.

## Výpisy a výkon

Knihovna má stovky knih a každá dlaždice je nativní pohled se dvěma obrázky.
Seznamy Knihy, Autoři a Série proto stojí na `FlatList`
(`src/components/ListScreen.tsx`) s hlavičkou v `ListHeaderComponent` – bez
virtualizace se při psaní do hledání překresloval celý seznam a klávesnice
nestíhala přijímat písmena. `BookCard` a `BookRow` jsou `memo`.

Detailové stránky (knihy autora, díly série) používají `BookGrid` uvnitř
`Screen` – tam jde o jednotky až desítky položek, takže je virtualizace zbytečná.

U mřížky s procentní šířkou dlaždic se **nepoužívá `gap` na kontejneru**:
sečetlo by se se 100 % šířky a do řádku by se vešla jediná dlaždice. Mezery
dělá vnitřní odsazení položek.

## Vzhled

Barvy se počítají ze **stejných OKLCH hodnot, jaké má web v `index.css`**
(`src/theme/palette.ts` + převod v `src/theme/oklch.ts`): tyrkysová má čísla
zapsaná napevno, ostatní schémata se odvozují z odstínu přes `schemeHues` ze
shared. Změna palety webu se tedy promítá sem, nikde jinde barvy nejsou.

Schéma a světlý/tmavý režim se berou z profilu uživatele jako na webu; změna
ve „Více → Vzhled“ se uloží přes `PUT /users/{id}/appearance`, takže platí
i na webu. Lokální kopie v `settings` slouží pro start bez sítě.

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
