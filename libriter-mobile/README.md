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

Bezpečnou zónu nahoře drží **rám** obrazovky, ne odsazení uvnitř rolovacího
pohledu: iOS rolovacím pohledům dopočítává vlastní odsazení a obojí se
sčítalo – výpis pak po prvním otevření začínal až pod třetinou obrazovky
a srovnal se, až se s ním pohnulo.

## Vzhled

Plovoucí plochy (kapsle přehrávače, lišta tabů, hlavičky výpisů) mají
rozostřené pozadí přes `expo-blur` – protějšek utility `glass`
(`backdrop-blur`) na webu. Samotné rozostření nestačí: nad světlou obálkou by
text zesvětlal, proto se přes ně klade ještě závoj z palety (`colors.glass`),
stejně jako to dělá `--glass` v CSS. Společná komponenta je
`src/components/ui/Blur.tsx`.

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

### Podepisování pro iPhone

První `just mobile-ios` na novém Macu skončí na

```
CommandError: No code signing certificates are available to use.
```

Mac nemá žádný podpisový certifikát (`security find-identity -v -p
codesigning` hlásí `0 valid identities found`) a projekt nemá nastavený
Apple tým. Jednorázové nastavení:

1. **Apple ID v Xcode** – Xcode → Settings → Accounts → **+** → Apple ID.
   Stačí bezplatné Apple ID, placený Developer Program není potřeba.
2. **Tým pro projekt** – `open ios/Libriter.xcworkspace`, target
   **Libriter** → **Signing & Capabilities** → zapnout *Automatically manage
   signing* a vybrat Team. Xcode sám vytvoří certifikát „Apple Development“
   a provisioning profil.
   - S bezplatným účtem může být `cz.libriter.app` už zabrané. Pokud to Xcode
     nahlásí, změň `bundleIdentifier` na něco unikátního (např.
     `cz.libriter.app.<jméno>`).
3. **iPhone** – Nastavení → Soukromí a zabezpečení → **Režim pro vývojáře**
   zapnout (telefon se restartuje). Po první instalaci s bezplatným účtem
   ještě Nastavení → Obecné → Správa VPN a zařízení → důvěřovat svému
   vývojářskému profilu.
4. Znovu `just mobile-ios`.

Tým vybraný v Xcode žije jen v `ios/`, který je generovaný – `expo prebuild`
nebo `rm -rf ios` ho zahodí. Natrvalo patří do `app.json` jako
`ios.appleTeamId` (desetiznakové ID týmu z Xcode → Settings → Accounts).

Aplikace podepsaná bezplatným účtem po 7 dnech přestane jít spustit; stačí ji
znovu nahrát přes `just mobile-ios`.

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

## `npm ci` rozbije iOS build

Vygenerovaný projekt `ios/` odkazuje na soubory, které vznikají **uvnitř**
`node_modules` při `pod install`:

- podspec `expo-sqlite` si tam kopíruje vendorované `sqlite3.c` a `sqlite3.h`,
- React Native codegen generuje hlavičky (`RNCNetInfoSpec.h`,
  `ReactCodegen/…`, `rngesturehandler_codegen/…`) do `ios/build/generated/`.

`npm ci` – který spouští `just frontend` – přeinstaluje `node_modules` a tím je
smaže. `Podfile.lock` se přitom nemění, takže si toho `expo run:ios` nevšimne
a build spadne na `cannot find 'exsqlite3_open' in scope` nebo `… file not
found` u codegen hlaviček.

Vendorované zdroje SQLite vrací zpátky `postinstall` v kořeni
([`_scripts/restore-expo-sqlite-sources.mjs`](../_scripts/restore-expo-sqlite-sources.mjs)),
takže `npm ci` i `npm install` nechají strom v použitelném stavu. `just
mobile-ios` navíc spouští `pod install` před buildem. Když už build jednou
spadl na chybějící **codegen** hlavičky, je potřeba vygenerovat projekt
načisto:

```bash
rm -rf libriter-mobile/ios && just mobile-ios
```

`ios/` je celý generovaný z konfigurace a je v `.gitignore`, takže se tím nic
neztratí.

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

Produkční buildy se sestavují lokálně a končí v `libriter-mobile/dist/`:

```bash
just mobile-keystore   # jednou: podepisovací klíč pro Android
just mobile-keystore-import <soubor.jks>   # …nebo existující klíč na dalším stroji
just mobile-release    # verze + dist/libriter.aab + dist/Libriter.ipa naráz
just mobile-apk        # → dist/libriter.apk – instalace mimo Google Play
just mobile-aab        # → dist/libriter.aab – nahrání do Google Play Console
just mobile-ipa        # → dist/Libriter.ipa – TestFlight / App Store (jen macOS)
just mobile-testflight # sestaví a rovnou nahraje do App Store Connect
```

`mobile-release`, `mobile-aab`, `mobile-ipa` a `mobile-testflight` se před
buildem zeptají na verzi a zapíšou ji do `app.json`
([`_scripts/mobile-version.mjs`](../_scripts/mobile-version.mjs)):

```
Mobilní aplikace: verze 1.0.0, build 1
  0) nic neměnit (druhá platforma téhož vydání) → 1.0.0 (build 1)
  1) jen build  → 1.0.0 (build 2)
  2) patch      → 1.0.1 (build 2)
  3) minor      → 1.1.0 (build 2)
  4) major      → 2.0.0 (build 2)
Volba [1]:
```

Obchody nepřijmou build se stejným číslem, jaké už mají. iOS
(`ios.buildNumber`) i Android (`android.versionCode`) proto sdílejí jedno číslo
buildu, které se zvyšuje při každé volbě kromě `0`. Vydání pro obě platformy
je nejjednodušší přes `just mobile-release` – jedna otázka, pak oba buildy
(na Linuxu jen AAB). Při samostatných receptech se verze zvolí u první
platformy a u druhé `0`. Stejně tak `0`, když build spadl a pouští se znovu.
V CI jde volbu předat proměnnou `LIBRITER_BUMP=build|patch|minor|major|none`.
Změněný `app.json` je potřeba commitnout. `mobile-apk` k ruční instalaci verzi
nemění.

**Android** – šablona Expa podepisuje release build debug klíčem. Plugin
[`plugins/withAndroidReleaseSigning.js`](plugins/withAndroidReleaseSigning.js)
ho nahrazuje vlastním klíčem, pokud jsou v `~/.gradle/gradle.properties`
nastavené `LIBRITER_UPLOAD_*`; jinak build zůstane podepsaný debug klíčem
a recept na to upozorní. `just mobile-keystore` klíč vytvoří
v `~/.config/libriter/android-upload.jks` a hesla zapíše do
`gradle.properties`. **Klíč zálohuj** – bez něj už nejde vydat aktualizaci
aplikace, která je v Google Play nebo nainstalovaná z APK.

Na **dalším stroji** klíč nevytvářej znovu – byl by jiný a telefon by
aktualizaci odmítl. Zkopíruj `.jks` (ze zálohy nebo ze stroje, kde vznikl)
a naimportuj ho:

```bash
just mobile-keystore-import ~/Downloads/android-upload.jks
```

Recept se zeptá na heslo, ověří ho (i heslo ke klíči, pokud je jiné), klíč
zkopíruje do `~/.config/libriter/` a zapíše `gradle.properties`. Funguje i pro
klíč vytvořený jinde (Android Studio, EAS): u úložiště s víc klíči je potřeba
přidat alias, `just mobile-keystore-import <soubor> <alias>`.

**iOS** – `just mobile-ipa` spustí `xcodebuild archive` a `-exportArchive`
s automatickým podepisováním. Potřebuje **placený** Apple Developer účet
přihlášený v Xcode (Settings → Accounts) a Team ID v `app.json`
(`ios.appleTeamId`, případně proměnná `LIBRITER_APPLE_TEAM`). Výchozí `method`
je `app-store-connect`; `just mobile-ipa release-testing` vytvoří ad hoc IPA
pro zařízení registrovaná v Apple Developer účtu.

Cesta do TestFlightu:

1. **První build** – `just mobile-ipa`. Díky `-allowProvisioningUpdates` Xcode
   při prvním běhu sám zaregistruje App ID `cz.libriter.app`, vytvoří
   certifikát „Apple Distribution“ a App Store provisioning profil.
2. **Aplikace v App Store Connect** – appstoreconnect.apple.com → Apps → **+**
   → New App: platforma iOS, bundle ID `cz.libriter.app` (po kroku 1 je
   v nabídce), SKU libovolné (`libriter`). Název musí být v App Store
   unikátní; na název pod ikonou v telefonu nemá vliv.
3. **Nahrání** – `just mobile-testflight` sestaví release a nahraje ho rovnou
   do App Store Connect účtem přihlášeným v Xcode. Druhá možnost je přetáhnout
   `dist/Libriter.ipa` z kroku 1 do aplikace **Transporter** (Mac App Store).
4. **TestFlight** – po zpracování (obvykle 10–30 min) se build objeví
   v záložce TestFlight. Interní testery (členy týmu v App Store Connect) jde
   přidat hned, bez schvalování od Apple; aplikace se pak instaluje přes
   aplikaci TestFlight na iPhonu.

**Lokální certifikát Apple Distribution je nutný.** Bez něj Xcode podepisuje
certifikátem spravovaným v cloudu Apple a ten zapíše jméno s diakritikou
(„MALÁSEK“) do podpisu v jiném tvaru Unicode, než má certifikát. App Store
Connect pak IPA odmítne s „Invalid Signature … (90035)“ u každého frameworku.
Certifikát se založí v Xcode → Settings → Accounts → Manage Certificates →
**+** → Apple Distribution; tamtéž (pravým tlačítkem → Export) se zálohuje
do `.p12` pro další Mac. `just mobile-ipa` podpis exportovaného IPA ověřuje
a s rozbitým skončí dřív, než se nahraje.

Každé další vydání je už jen `just mobile-testflight` a volba verze.

`app.json` má v `infoPlist` `ITSAppUsesNonExemptEncryption: false` –
prohlášení pro Apple, že aplikace nepoužívá jiné šifrování než to, které je
součástí systému (HTTPS). Díky němu App Store Connect u každého buildu
nevyžaduje dotazník o exportu šifrování. Kdyby aplikace někdy začala šifrovat
sama (vlastní kryptografie, šifrovaný přenos mimo HTTPS), je potřeba hodnotu
přehodnotit.

Alternativou bez lokálních nástrojů je EAS Build (`eas build --profile
production`, profily jsou v `eas.json`) – sestavuje v cloudu Expa a klíče
spravuje sám.
