# Plán: mobilní přehrávač Libriter (iOS + Android, offline-first)

> Stav: schváleno, čeká na implementaci. Fáze se odškrtávají v tabulce níže.
> Spuštění na jiném počítači: otevřít Claude Code v kořeni repa a zadat např.
> „Implementuj fázi 1 podle docs/mobile-app-plan.md“.

| Fáze | Stav |
|---|---|
| 1. Backend: mobilní token + velikost kapitol | ☐ |
| 2. Backend: dávková synchronizace pozic | ☐ |
| 3. npm workspaces + libriter-shared | ☐ |
| 4. Mobil: kostra jen online | ☐ |
| 5. Mobil: lokální DB + stahování | ☐ |
| 6. Mobil: offline fronta + sync engine | ☐ |
| 7. Leštění + distribuce | ☐ |

## Kontext

Libriter má Go backend (chi, SQLite) a webový přehrávač (React 19 + Vite). README
už počítá s `libriter-mobile/`, `.gitignore` má blok pro Expo, ale nic z toho neexistuje.
Cílem je mobilní aplikace **pouze jako přehrávač**: přihlášení, knihovna, detail knihy,
přehrávač, stažené knihy a nastavení. Správa knihovny zůstává výhradně na webu.

Klíčový požadavek je **offline režim**: kniha se celá stáhne do telefonu, poslech
generuje pozice a odposlouchané sekundy lokálně, a po připojení se dávkově odešlou.

Zmapované překážky na serveru (proto jde server první):
- Pozice se ukládá per kapitola přes `PUT /sessions/{id}/position`, bez klientského
  časového razítka a ID zařízení. Platí "poslední zápis vyhrává", offline dávka by
  přepsala novější pozici z jiného zařízení.
- Deník poslechu (`listening_log`) připisuje sekundy k dnešnímu dni **serveru**
  (`storage/listening.go:44-59`) a omezuje 600 s na jeden zápis.
- Přihlašovací JWT platí 72 h bez refreshe, streamovací token 24 h (`service/auth.go`).
- `GET /books/{id}/chapters` nevrací velikost souboru, mobil neumí odhadnout místo.
- `ReorderChapters` (`storage/chapter.go:168`) nezvedá `books.updated_at`, mobil by
  nepoznal změnu pořadí kapitol.

Co už funguje a mobil to jen použije: Range requesty na audio (`http.ServeContent`
v `handler/audio.go:107`), veřejné obálky, session model, `book_progress`.

## Rozhodnutí (dohodnuto)

| Téma | Volba |
|---|---|
| Technologie | React Native + Expo, TypeScript, expo-router, react-native-track-player |
| Sdílení kódu | npm workspaces v kořeni, nový balíček `libriter-shared` |
| Auth v mobilu | dlouhodobý JWT se scope `mobile` (výchozí 365 dní), bez refresh tokenů a DB tabulky |
| Konflikty pozic | vyhrává novější `recorded_at` vzniklé na klientovi |
| Den v deníku | UTC den z `recorded_at` (konzistentní s migrací 011 a admin přehledy) |
| Knihovna | plný fetch `GET /books`, žádný delta endpoint (osobní knihovna, malý seznam) |
| Velikost souboru | nový sloupec `chapters.size_bytes` plněný scannerem |
| Distribuce | lokální buildy na Macu, TestFlight (iOS) + ruční APK (Android) |
| Pořadí | nejdřív server, pak workspace/shared, pak mobil |

---

## Fáze 1: Backend, mobilní token + velikost kapitol

**Soubory:** `internal/service/auth.go`, `internal/api/handler/auth.go`,
`internal/api/middleware/auth.go`, `internal/config/config.go`, `env.example`,
`cmd/server/serve.go`, `internal/storage/chapter.go`, `internal/scanner/ingest.go`,
`internal/model/model.go`, nová migrace `012_chapter_size.sql`.

1. **Mobilní token**
   - `POST /api/v1/auth/mobile-token` v `Authenticate` skupině vedle `GET /auth/stream-token`.
     Tělo `{"device_name": "..."}` (volitelné, jen do logu). Odpověď `{"token","expires_at"}`
     stejně jako `AuthHandler.StreamToken`.
   - `const ScopeMobile = "mobile"`, `GenerateMobileToken(userID, role)` podle `GenerateStreamToken`.
   - `ParseToken` (`service/auth.go:182-191`): povolit scope prázdný **nebo** `mobile`.
     `ParseStreamToken` zůstává přesná shoda na `stream`, takže mobilní token neotevře
     audio bez `?t=` a streamovací token neotevře API.
   - Scope uložit do request contextu; `MobileToken` handler odmítne volajícího se scope
     `mobile` (403), aby uniklý token nemohl razit další tokeny donekonečna.
   - Config `JWT_MOBILE_EXPIRY_DAYS` (výchozí 365) → `JWTConfig.MobileExpiry`.
   - Middleware `Authenticate` se nemění: dál čte uživatele z DB, takže smazání účtu
     token efektivně zneplatní.
2. **`chapters.size_bytes`**
   - Migrace `012_chapter_size.sql`: `ALTER TABLE chapters ADD COLUMN size_bytes INTEGER NOT NULL DEFAULT 0 CHECK (size_bytes >= 0);`
   - `model.Chapter.SizeBytes int64 \`json:"size_bytes"\``; doplnit do SELECT/RETURNING
     v `storage/chapter.go` a do `ON CONFLICT (file_path) DO UPDATE`.
   - Scanner `appendChapter` (`scanner/ingest.go` ~286): `os.Stat` souboru. Staré řádky
     mají 0 = neznámé; admin "Rescan" je doplní. Mobil bere 0 jako neznámé a použije
     `Content-Length`.
3. **`ReorderChapters` zvedne `books.updated_at`** (jeden UPDATE v téže transakci), ať mobil
   pozná změnu pořadí kapitol podle `book.updated_at`.
4. **Testy** (vzor `handler/play_session_test.go`, env z `admin_test.go:91-226`):
   mobilní token projde `Authenticate`, ale ne `/chapters/{id}/audio`; stream token
   neprojde API; mobilní token nevyrazí další mobilní token; `size_bytes` v odpovědi po
   rescanu; reorder mění `updated_at`.

**Ověření:**
```bash
cd libriter-backend && go test ./... && go vet ./...
# login → mobile token → GET /books s mobilním tokenem = 200, audio ?t=<mobile> = 401
```

## Fáze 2: Backend, dávková synchronizace pozic

**Soubory:** nová migrace `013_position_sync.sql`, `internal/storage/play_session.go`,
`internal/storage/listening.go`, nový `internal/storage/sync.go`,
`internal/service/play_session.go`, `internal/api/handler/play_session.go`,
`cmd/server/serve.go`, nový `handler/sync_test.go`, README sekce API.

1. **Migrace `013_position_sync.sql`**
   ```sql
   ALTER TABLE play_session_items ADD COLUMN position_recorded_at DATETIME;
   ALTER TABLE play_sessions      ADD COLUMN position_recorded_at DATETIME;
   CREATE TABLE sync_events (
     id         TEXT NOT NULL PRIMARY KEY,
     user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
     device_id  TEXT NOT NULL,
     applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
   );
   CREATE INDEX sync_events_applied_idx ON sync_events (applied_at);
   ```
   Nepoužité tabulky `devices` a `playback_positions` z migrace 001 nechat být (stejný
   postoj jako komentáře v 010/011).
2. **Endpoint `POST /api/v1/sessions/sync`** (reader+, registrovat před `/sessions/{id}`).
   Request (max 500 událostí, vejde se do 1 MB limitu `readJSON`):
   ```json
   {
     "device_id": "uuid",
     "events": [{
       "id": "uuid",
       "session_id": "uuid",
       "book_id": "uuid",
       "chapter_id": "uuid",
       "position_seconds": 1234,
       "playback_speed": 1.25,
       "listened_seconds": 10,
       "finished": false,
       "book_finished": false,
       "recorded_at": "2026-09-17T18:22:03+02:00"
     }]
   }
   ```
   Response:
   ```json
   {
     "results": [{"id": "uuid", "status": "applied|stale|duplicate|rejected", "error": "..."}],
     "sessions": [ PlaySession, ... ]
   }
   ```
   Sémantika: seřadit podle `recorded_at`, každou událost ve vlastní transakci:
   1. `INSERT INTO sync_events` → při konfliktu `duplicate`, nic dalšího (ochrana deníku
      před dvojím započtením při opakování dávky).
   2. Validace jako `PlaySessionService.SavePosition` (rychlost 0.5–3.0, kapitola patří
      knize, session je uživatele, kniha je v session) → jinak `rejected`.
   3. Pozice se zapíše jen když `recorded_at > position_recorded_at` (NULL = vždy),
      jinak `stale`. Kroky 4 a 5 běží i pro `stale`.
   4. `listening_log`: `listened_seconds` (clamp 0..600 per událost, stávající
      `maxListenedSecondsPerSave`) k `day = date(recorded_at UTC)`. `recorded_at` více než
      5 min v budoucnosti se ořízne na serverové "teď".
   5. `touchBookProgress` jako dnes.
   Na začátku každého volání `DELETE FROM sync_events WHERE applied_at < now-30d`.
   Porovnání časů dělat v Go (`parseSQLiteTime`), ne v SQL, kvůli různým formátům
   DATETIME v SQLite (viz `sqliteTimeLayouts` v `storage/listening.go`).
3. **Rozšíření `PUT /sessions/{id}/position`** o volitelné `recorded_at` a `device_id`.
   Web je neposílá (`DisallowUnknownFields` nevadí, pole jsou volitelná), pak platí
   `position_recorded_at = CURRENT_TIMESTAMP` a `day = date('now')`. Oba endpointy sdílí
   jednu storage cestu: `PlaySessionPosition.RecordedAt *time.Time`.
4. **`switchToBook`** (`service/play_session.go:230`) volá `UpdatePlaySessionPosition`
   s existující pozicí. Musí **nezvedat** `position_recorded_at`
   (`PreserveRecordedAt bool`), jinak otevření knihy na webu označí všechny čekající
   mobilní události jako `stale`.
5. **Storage funkce:** `UpdatePlaySessionPosition` vrací nový `ErrStalePosition`;
   `addListeningLog(ctx, q, userID, bookID, seconds, at time.Time)` s `date(?4)` a
   `last_at = MAX(last_at, ?4)`; `insertSyncEvent`, `PruneSyncEvents` v `sync.go`;
   `PlaySessionService.Sync(ctx, userID, deviceID, events)`; `PlaySessionHandler.Sync`.
6. **Testy `handler/sync_test.go`:** duplicitní id započte poslech jednou; `stale`
   nezmění pozici, ale připíše poslech; den odvozen z `recorded_at`; `rejected` neblokuje
   zbytek dávky; `switchToBook` nezvedá razítko; PUT s `recorded_at` se chová stejně.
7. **README:** doplnit sekci k `/auth/mobile-token`, `/sessions/sync` a `size_bytes`
   (vedle stávající dokumentace session API na `README.md:651-722`).

**Ověření:**
```bash
go test ./... ; go vet ./...
# curl dávku dvakrát → podruhé samé "duplicate"
sqlite3 data/libriter.db "select * from listening_log order by day desc limit 5"
# admin → Poslech → detail uživatele ukazuje minuty jen jednou a ve správný den
```

## Fáze 3: npm workspaces + `libriter-shared`

**Soubory:** nový kořenový `package.json`, nový `libriter-shared/`, přesuny ve
`libriter-frontend/src/`, `justfile`, `.gitignore`.

1. Kořen: `{"name":"libriter","private":true,"workspaces":["libriter-shared","libriter-frontend","libriter-mobile"]}`.
   Lockfile se přesune do kořene, `libriter-frontend/package-lock.json` smazat.
2. `libriter-shared/package.json`: `"type":"module"`, `"main"`/`"types"`/`"exports"` míří
   na `./src/index.ts` (TS zdroj bez build kroku; Vite 8 i Metro od Expo SDK 52 to
   zvládnou). Žádná závislost na Reactu ani DOM. Vlastní `tsconfig.json` (strict,
   `moduleResolution: bundler`, `noEmit`).
3. Přesun z frontendu (ověřeno, že jsou bez DOM/React závislostí):
   - `src/api/types.ts` → `shared/src/types.ts` beze změny.
   - `src/api/client.ts` → `shared/src/client.ts` + `setBaseUrl(url)`; fetch na
     `baseUrl + API_PREFIX + path`.
   - `src/auth/session.ts` rozdělit: `isTokenExpired`, typ `Session`, `parseSession` do
     shared; `loadSession/saveSession/clearSession` (localStorage) zůstávají ve webu.
     Každá appka vlastní perzistenci, shared jen parsování.
   - Konstanty a čisté helpery z `src/player/playerContext.ts` (`SPEEDS`, `SKIP_*`,
     `SAVE_INTERVAL_MS`, `REMOTE_SYNC_INTERVAL_MS`, `MAX_TIMEUPDATE_GAP_SECONDS`,
     `sessionItem`, `currentBookId`) → `shared/src/player.ts`.
   - `src/player/sessionLabels.ts`, `src/auth/permissions.ts`, objekt `queryKeys` z
     `src/api/hooks.ts` → shared. `hooks.ts`/`adminHooks.ts` zůstávají.
   - Ve frontendu nechat původní soubory jako tenké re-export shimy
     (`export * from 'libriter-shared'`), aby se ~50 importů neměnilo.
4. `justfile`: `frontend` = `npm ci` v kořeni + `npm run build -w libriter-frontend`;
   přidat `typecheck`, `mobile-start`, `mobile-ios`, `mobile-apk`. Do `.gitignore`
   přidat `libriter-mobile/ios/` a `libriter-mobile/android/` (prebuild výstupy).
5. README: opravit zmínku o `Makefile` (řádek 7) na justfile, popsat workspace.

**Ověření:** `npm ci` v kořeni, `just build`, web běží beze změny chování; `npx tsc --noEmit`
ve shared i frontendu.

## Fáze 4: Mobil, kostra jen online

Nástroje připraví `just setup`, stav ukáže `just doctor` (Android všude, iOS jen na macOS).

**Nový projekt `libriter-mobile/`** (`create-expo-app`, aktuální Expo SDK, dev build,
ne Expo Go). Závislosti: expo-router, expo-sqlite, expo-file-system, expo-secure-store,
expo-image, react-native-track-player, @react-native-community/netinfo,
@tanstack/react-query, libriter-shared.

Struktura:
```
app/_layout.tsx                 providery: QueryClient, Db, Auth, Sync, Player
app/(auth)/login.tsx            URL serveru + e-mail + heslo
app/(app)/_layout.tsx           taby + auth guard
app/(app)/index.tsx             knihovna: rozposlouchané session, všechny knihy
app/(app)/book/[id].tsx         detail, kapitoly, stáhnout/smazat, přehrát
app/(app)/player.tsx            celoobrazovkový přehrávač (modal)
app/(app)/downloads.tsx         app/(app)/settings.tsx
src/api/queries.ts              react-query hooky nad shared apiFetch
src/auth/                       SecureStore session, AuthProvider
src/db/                         schema.ts + repozitáře
src/sync/syncEngine.ts
src/downloads/downloadManager.ts
src/player/service.ts, PlayerProvider.tsx, positionSaver.ts
```

Deliverable fáze: přihlášení (login → `POST /auth/mobile-token` → mobilní token do
SecureStore, přihlašovací token zahodit), seznam knih, detail, streamované přehrávání
přes track-player s ovládáním na zamčené obrazovce, ukládání pozic přes
`/sessions/sync` (ještě bez fronty, rovnou online).

Přehrávač zrcadlí web (`PlayerProvider.tsx`): fronta = kapitoly aktuální knihy; uložení
každých `SAVE_INTERVAL_MS`, při pauze, seeku, změně kapitoly, přechodu do pozadí;
`listened_seconds` z `PlaybackProgressUpdated` se stejným pravidlem mezery jako `onTime`
(`PlayerProvider.tsx:540-548`); `book_finished` po poslední kapitole, `finished` když
není další kniha (`PlayerProvider.tsx:561-572`); dedup fingerprintem jako `savePosition`.
Pozici ze serveru přebírat jen při pauze a jen když je `updated_at` novější
(`syncFromServer`, `PlayerProvider.tsx:620-655`).

**Ověření:** `npx expo run:ios --device` a `run:android`; pozice se objeví ve webu; audio
hraje se zamknutým displejem.

## Fáze 5: Mobil, lokální DB + stahování

SQLite schéma:
```
books(id PK, title, updated_at, duration_seconds, chapter_count, series_id, json)
chapters(id PK, book_id, position, title, file_name, start_offset_seconds, duration_seconds, size_bytes)
downloads(book_id PK, state 'queued|downloading|paused|complete|error', bytes_total, bytes_done, error, updated_at)
chapter_files(chapter_id PK, book_id, path, size_bytes, state 'pending|done')
sessions(id PK, updated_at, local_only INTEGER, json)
pending_events(id PK, session_id, book_id, chapter_id, position_seconds, playback_speed,
               listened_seconds, finished, book_finished, recorded_at, attempts)
settings(key PK, value)   -- device_id, server_url, wifi_only, last_library_sync
```
UI čte vždy z lokální DB, server je jen synchronizační partner.

Download manager: per kapitola `createDownloadResumable` s `?t=<stream token>` (nový token
na dávku, při 401 obnovit a jednou zopakovat), 2 souběžně, cíl
`${documentDirectory}libriter/<bookId>/<chapterId>.<ext>`, obálka do `cover.jpg`.
Průběh ze `size_bytes` (fallback `Content-Length`), ověření velikostí po dokončení,
pauza drží `resumeData`, volba "jen Wi-Fi" přes NetInfo. Na iOS nastavit vyloučení z
iCloud zálohy. Smazání = adresář + řádky.

Přehrávač volí `url` = lokální soubor, když `chapter_files.state='done'`, jinak vzdálený.

**Ověření:** stáhnout knihu, režim letadlo, restart appky, přehrát; smazat a ověřit místo.

## Fáze 6: Mobil, offline fronta + sync engine

- Každé uložení pozice = řádek v `pending_events` + pokus o flush.
- Stavy `idle | syncing | offline | backoff(n)`. Spouštěče: start appky, NetInfo
  `isInternetReachable` false→true, `AppState` → active, debounce 2 s po vložení události,
  pull-to-refresh.
- Pořadí kroků: (1) vyřešit `local_only` session, (2) flush `pending_events` po
  `recorded_at` v dávkách 200 na `POST /sessions/sync`, po 2xx smazat řádky pro všechna
  vrácená id bez ohledu na status (`rejected` zalogovat), přepsat mirror vrácenými
  `sessions`, (3) `GET /sessions` → mirror, (4) `GET /books` → upsert, chybějící smazat
  (stažené soubory označit jako osiřelé), kapitoly znovu jen když se změnilo `updated_at`.
- Backoff 5/15/60/300 s s jitterem; 401 při online → odhlásit.
- Offline přehrání stažené knihy bez session: provizorní session s klientským UUID a
  `local_only=1`; při sync kroku (1) `POST /sessions {kind:'book', book_id}` a přepsat
  `session_id` v čekajících událostech na serverové id před flushem.
- Vypršelý mobilní token offline: přehrávání dál funguje, přihlášení se vyžádá až při
  prvním 401 online.

**Ověření:** poslouchat 5 min offline, připojit, web ukáže pozici, admin přehled poslechu
minuty ve správný den; dávku poslat dvakrát (simulace retry) a ověřit, že se minuty
nezdvojí.

## Fáze 7: Leštění + distribuce

Nastavení (jen Wi-Fi, přehled místa, odhlášení), rychlost přehrávání, sleep timer,
skoky vpřed/vzad, `UIBackgroundModes: audio`, Android foreground service, ikony a
splash, `expo prebuild`, upload do TestFlight, `gradlew assembleRelease` pro APK.
Volitelně později CarPlay/Android Auto.

---

## Rizika a otevřené body

- Dvě verze Reactu v jednom workspace (web 19.2 vs. verze pinovaná Expo). Shared na
  Reactu nezávisí, ale hoisting může zmást Metro. Řešení: sjednotit verze nebo per-app
  `overrides`.
- Špatně nastavené hodiny na zařízení: budoucí `recorded_at` se ořízne, minulost se přijme.
- Streamovací token 24 h: stahování dlouhých knih a streamování musí umět obnovit token.
- Web dál posílá bez `recorded_at` (server dosadí "teď"), pravidlo "novější vyhrává"
  platí. Upgrade webu na posílání `recorded_at` je volitelný krok.
- `sync_events` roste ~1 řádek na 10 s poslechu, 30denní prune stačí.
- Track-player a lokální soubory: `file://` URL a správné content-type pro `.m4b`/`.mp3`
  na Androidu ověřit brzy (fáze 5).
- Expo Go neumí track-player, od začátku se pracuje s dev buildem.
