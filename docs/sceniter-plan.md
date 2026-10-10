# Plán: Sceniter (filmy a seriály) jako druhý produkt v monorepu `mediater`

> `mediater` je pracovní zastřešující název. Pokud se změní, jde o mechanickou
> náhradu (Go module path, kořenový `package.json`, název deb balíčku, repo na GitHubu).

> Stav: implementováno ve větvi `sceniter`. Ověřeno automatickými testy
> (Go včetně end-to-end testu se skutečným ffmpeg, typecheck všech workspaců,
> i18n kontrola), buildy webů, sestavením mobilního bundlu, sestavením `.deb`
> nanečisto a snímky obou webů v headless Chromiu. Co zbývá ověřit ručně a na
> produkci, je na konci v oddílu „Zbývá“.

| Fáze | Obsah | Stav |
|---|---|---|
| 0 | Přejmenování a přeuspořádání repa, beze změny chování | ☑ (repo na GitHubu nepřejmenováno) |
| 1 | Backend: jádro, rozhraní `Product`, migrace per modul, výběr SPA podle hostname | ☑ |
| 2 | Sceniter backend: schéma, scanner, TMDB, direct play + remux + audio transkód | ☑ |
| 3 | `core-shared`, `sceniter-shared`, `core-web`, `sceniter-frontend` | ☑ |
| 4 | Plný HLS transkód, seek, titulky | ☑ |
| 5 | Deploy: balíček, nginx pro druhou doménu, README, release | ☑ soubory; produkce čeká |

## Kontext

Uživatel chce druhou aplikaci **Sceniter** pro správu a přehrávání videí (filmy, seriály)
se **společnou databází uživatelů** s Libriterem, ale se samostatným webovým frontendem.
TV aplikace (Apple TV, Google TV) jsou **odložené na později**; architektura je má
umožnit bez přestavby.

Dnešní stav:
- Go backend `libriter-backend/` (chi, SQLite přes modernc, jedna binárka s `go:embed` webu).
  Všechno DI a všechny routy jsou v `cmd/server/serve.go:runServe()`.
- Identita (`users`, `user_roles`, `settings`, `audit_log`, `sync_events`) leží ve stejné DB
  a Go modulu jako knihy. JWT (HS256, `service/auth.go`) nemá `aud`, scope `""`/`mobile`/`stream`.
- Migrace jsou jedna řada `001–017` (`internal/db/migrate.go`, `schema_migrations(version PK)`).
- npm workspaces `libriter-shared` / `libriter-frontend` / `libriter-mobile`, orchestrace `justfile`.
- Nasazení: deb balíček (`packaging/deb/`), systemd, nginx `deploy/nginx/audiobooks.helpdesk24.works.conf`.

**Tvrdé omezení:** mobilní aplikace `cz.libriter.app` je v distribuci. Cesty `/api/v1/...`,
tvary odpovědí a tok `/auth/login` → `/auth/mobile-token` se nesmí změnit.

Znovu použijeme:
- Range streamování a rušení `WriteTimeout` v `handler/audio.go` (`http.ServeContent`,
  `http.NewResponseController`) a spouštění ffmpeg s `flushWriter`.
- Streamovací token `?t=` (`ScopeStream`, `ParseStreamToken`, `GET /auth/stream-token`).
- Sémantiku synchronizace pozic „novější `recorded_at` vyhrává“ a tabulku `sync_events` (migrace 013).
- fsnotify + stabilizaci velikosti ze `scanner/scanner.go`, přirozené řazení `scanner/natural.go`.
- `middleware.Authenticate` / `RequireRole`, `respond.go` (`writeJSON`/`writeError` se stabilními kódy).
- `web.Handler()` (cache hlavičky, SPA fallback, 503 stránka bez buildu).

## Rozhodnutí

| Téma | Volba |
|---|---|
| Backend | jeden Go modul, jedna binárka, jedna SQLite; `internal/core` + `internal/libriter` + `internal/sceniter` |
| Klienti | teď jen web; TV později (viz „Připraveno pro TV“) |
| Video | lokální soubory, scanner + ffprobe, direct play s Range; jinak HLS přes ffmpeg (remux → audio transkód → plný transkód) |
| Metadata | TMDB (`TMDB_API_KEY`, jazyk `cs-CZ`, fallback `en-US`) |
| Repo | stejné monorepo přejmenované na `mediater`; backend `git mv libriter-backend server` |
| Go module | `mediater` (importy se v Fázi 0 mění kvůli přesunům stejně) |
| Migrace | `schema_migrations(module, version)`; 001–017 zůstávají jako modul `core` |
| API | Libriter beze změny na `/api/v1/...`; jádro `/api/v1/auth|users|admin/users|…`; Sceniter `/api/v1/sceniter/...` |
| Výběr SPA | podle `Host`: `LIBRITER_HOST`, `SCENITER_HOST`, `DEFAULT_PRODUCT=libriter`; API na hostu nezávisí |
| Role | globální (admin/editor/reader); per-produkt přístup jako budoucí `user_products` |
| Tabulky Sceniteru | prefix `sc_` (snadné budoucí oddělení) |
| Sdílené web UI | nový workspace `core-web` (shadcn ui, theme, auth, Login/Register/Profile) |
| Deb | balíček `mediater` s `Replaces/Breaks/Provides: libriter`; `/var/lib/libriter` a uživatel `libriter` zůstávají |
| Doména | `video.helpdesk24.works` (stejný nginx stroj i upstream) |

## Cílová struktura

```
mediater/
├── server/                      # Go modul `mediater` (dnes libriter-backend/)
│   ├── cmd/server/              # main.go, serve.go (jen sestavení), user_cmd.go, repair_cmd.go
│   └── internal/
│       ├── core/  config db(+migrations 001–017) model storage service httpx middleware handler web version product imagestore fswatch
│       ├── libriter/  product.go scanner importer metadata audiostore model storage service handler db/migrations web/{web.go,dist}
│       └── sceniter/  product.go config.go model storage service handler scanner metadata/tmdb stream db/migrations web/{web.go,dist}
├── core-shared/    libriter-shared/    sceniter-shared/    core-web/
├── libriter-frontend/  libriter-mobile/  sceniter-frontend/
├── packaging/deb/  deploy/nginx/  docs/  _scripts/  justfile  package.json
```

---

## Fáze 0: Přejmenování a přeuspořádání (beze změny chování)

Každý krok samostatný commit, přesuny výhradně `git mv` (zachová `git log --follow`).

1. Repo na GitHubu přejmenovat na `mediater` (GitHub drží redirect), upravit `origin`,
   kořenový `package.json` `"name": "mediater"`.
2. `git mv libriter-backend server`. Upravit `justfile` (`backend_dir`, `bin := "bin/mediater"`,
   `dist`), `libriter-frontend/vite.config.ts` `outDir`, `.gitignore`, `_scripts/*`
   (`release.sh`, `doctor.sh`, `init_backend.sh`), `README.md`.
   `config.findEnvFile` hledá `server/.env` i starý `libriter-backend/.env`.
3. `go mod edit -module mediater` + sed importů `"libriter/internal` → `"mediater/internal`;
   ldflags `-X mediater/internal/core/version.Version`; `LIBRITER_VERSION` → `MEDIATER_VERSION`.
4. Rozdělit Go balíčky (jen přesuny + importy):
   - `config`, `db`, `version`, `web`, `api/middleware` → `internal/core/...`;
   - `api/handler/respond.go` → `core/httpx` s exportovanými funkcemi; v handler balíčcích
     privátní aliasy (`var writeJSON = httpx.WriteJSON`), ať se nemění stovky volání;
   - `model.go`: identita (`User`, `RoleLevel`, settings, audit) → `core/model`, knihy → `libriter/model`;
   - `storage`: `store.go`, `user.go`, `settings.go`, `audit.go`, `sync.go` → `core/storage`
     (helpery `querier`, `isUniqueViolation`… exportovat); `libriter/storage.Store` embeduje
     `*corestorage.Store`, takže volání `store.GetUserByID` zůstanou beze změny;
   - `service/{auth,login,user,audit,settings}.go` → `core/service`; zbytek → `libriter/service`;
   - `handler/{auth,user,health}.go` → `core/handler`; zbytek → `libriter/handler`;
   - `scanner`, `importer`, `metadata`, `audiostore` → `internal/libriter/...`; `imagestore` → `core`.
5. Deb balíček se zatím nepřejmenovává, Fáze 0 jde vydat jako běžný patch.

**Ověření**
```bash
cd server && gofmt -l . && go build ./... && go vet ./... && go test ./...
just build && ./bin/mediater serve      # /health, login, přehrání kapitoly s ?t=
grep -rn "libriter-backend" --include=*.{sh,mjs,md,ts,json} . | grep -v node_modules
DRY_RUN=1 bash _scripts/release.sh
```

## Fáze 1: Jádro, `Product`, migrace per modul, SPA podle hostname

**Soubory:** `core/product/product.go`, `core/db/migrate.go` (+ test), `core/web/{web.go,hostmux.go}`,
`core/config/config.go`, `env.example`, `libriter/product.go`, `libriter/web/web.go`,
`cmd/server/serve.go`, rozdělení `core/service/settings.go` a `admin.go`.

1. **Rozhraní**
   ```go
   type Deps struct { Ctx context.Context; Cfg *config.Config; DB *sql.DB; Store *storage.Store
                      Auth *service.AuthService; Users *service.UserService
                      Settings *service.SettingsService; Audit *service.AuditService; StartedAt time.Time }
   type Product interface {
       Name() string                 // "libriter" | "sceniter"
       Migrations() fs.FS            // NNN_*.sql, číslování od 001 v rámci modulu
       Init(d Deps) error            // DI, start scanneru
       RegisterRoutes(r chi.Router)  // r = router /api/v1
       Web() fs.FS                   // sestavená SPA
       Shutdown(ctx context.Context) error
   }
   ```
   `serve.go`: config → db → core služby → `Migrate(core, products…)` → `Init` → core routy +
   `p.RegisterRoutes` → `web.HostMux`. Libriter registruje na `/api/v1` (kompatibilita),
   Sceniter na `r.Route("/sceniter", …)`.
2. **Migrace per modul:** runner zjistí přes `PRAGMA table_info(schema_migrations)`, zda chybí
   sloupec `module`, a v jedné transakci přestaví tabulku na
   `PRIMARY KEY(module, version)` s existujícími řádky jako `core`.
   `Migrate(ctx, db, sets ...MigrationSet{Module, FS})`, pořadí core → produkty.
   Test: DB se starým schématem (17 řádků) + fake modul `x/001` → 18 řádků, `(x,1)` přítomen.
3. **Config:** `LIBRITER_HOST`, `SCENITER_HOST` (čárkou oddělené seznamy, bez portu,
   case-insensitive), `DEFAULT_PRODUCT`. Exportovat `envStr/envInt/envPath`, ať si produkt
   načte vlastní proměnné sám.
4. **Web:** `web.New(dist fs.FS, title string)` = dnešní `Handler()` s parametrem;
   `HostMux{Default, ByHost}`. Embed v `libriter/web` (a později `sceniter/web`);
   `vite.config.ts` → `../server/internal/libriter/web/dist`.
5. **Sdílené routy do core:** `/auth/*`, `/users/*`, `/admin/users`,
   `/admin/settings/registration`, `/admin/audit`. Knižní admin zůstává v libriter.
   `SettingsService` rozdělit na core (registrace, obecné get/set) a libriter (metadata, knihovna).
6. Dev: `http://libriter.localhost:8080` / `http://sceniter.localhost:8080` (prohlížeč mapuje
   `*.localhost` na loopback); Vite dev servery 5173 / 5174 proxují `/api` na `:8080`.

**Ověření**
```bash
cd server && go test ./... && go vet ./...
LIBRITER_HOST=libriter.localhost SCENITER_HOST=sceniter.localhost go run ./cmd/server
curl -s http://sceniter.localhost:8080/ | head -3     # 503 „Sceniter – frontend není sestaven“
curl -s http://localhost:8080/api/v1/nonsense          # JSON chyba
sqlite3 data/libriter.db 'select module,version from schema_migrations order by 1,2' | tail
# regrese mobilu: login → POST /auth/mobile-token → GET /books → GET /chapters/{id}/audio?t=
```
Před nasazením zálohovat produkční `libriter.db` a vyzkoušet migraci na její kopii.

## Fáze 2: Sceniter backend

**Soubory:** `sceniter/{product.go,config.go}`, `sceniter/db/migrations/001_sceniter_init.sql`,
`sceniter/model`, `sceniter/storage/{movie,show,media,progress,watchlist}.go`,
`sceniter/scanner/{scanner,naming,naming_test,probe,ingest}.go`, `sceniter/metadata/tmdb`,
`sceniter/service/{library,match,progress}.go`, `sceniter/stream/{decision,direct,hls,session}.go`,
`sceniter/handler/*`, `core/fswatch`.

1. **Config:** `VIDEO_ROOT` (prázdné = produkt vypnutý, API vrací `sceniter.disabled`),
   `SCENITER_MOVIES_DIRS=Filmy,Movies`, `SCENITER_SHOWS_DIRS=Seriály,Shows,Series`,
   `SCENITER_IMAGE_ROOT`, `TRANSCODE_ROOT`, `TMDB_API_KEY`, `TMDB_LANGUAGE=cs-CZ`,
   `TRANSCODE_MAX_SESSIONS=2`, `TRANSCODE_IDLE_SECONDS=60`.
2. **Schéma** (`sc_` prefix): `sc_movies`, `sc_shows`, `sc_seasons`, `sc_episodes`,
   `sc_media_files` (cesta, velikost, kontejner, délka, video kodek, rozlišení; patří filmu
   **nebo** epizodě přes CHECK), `sc_media_streams` (audio/titulky z ffprobe),
   `sc_subtitle_files` (externí .srt/.vtt), `sc_genres` + vazby, `sc_people` + `sc_credits` (top 10).
   Per uživatel: `sc_watch_progress` (PK user+media, `position_s`, `completed`, `device_id`,
   `recorded_at`), `sc_watch_log` (UTC den z `recorded_at`), `sc_watchlist`, `sc_ratings`.
   Všechny `match_status` = `unmatched|auto|manual|ignored`.
3. **Scanner:** watcher + stabilizace vytažené do `core/fswatch` (pokud libriter testy
   projdou beze změn; jinak kopie a sjednocení později). Ignorovat `@eaDir`, `*.vsmeta`,
   `*.part`, skryté soubory. `naming.go`: `ParseMovie` (`Název (2003).mkv`, scene názvy
   s tečkami a tagy 720p/BRRip/x264/CZ…), `ParseEpisode` (`S01E02`, `1x02`, `02. díl`,
   sezóna z adresáře `S03|Season 3|Série 3|Sezóna 3`). `probe.go` přes `ffprobe -print_format json`.
   ffprobe běží **mimo** DB transakci; zápisy dávkovat po filmu/sezóně (SQLite má jedno spojení).
4. **TMDB:** search/detail pro film i seriál, sezóny, credits; obrázky do
   `SCENITER_IMAGE_ROOT/{posters,backdrops,stills,profiles}` přes `core/imagestore`;
   limiter 40 req/10 s. Automatický match jen při shodě roku ±1 nebo jediném výsledku,
   jinak `unmatched` → ruční match v adminu.
5. **Rozhodnutí o přehrávání** (`stream/decision.go`): profily `web` a `safari` (kontejnery,
   video a audio kodeky, HLS). Výstup `direct | remux | transcode_audio | transcode`.
   V této fázi: `direct`, `remux` (`-c copy`) a `transcode_audio`
   (`-c:v copy -c:a aac -ac 2`, pokryje MKV h264 + AC3 CZ dabing); `transcode` vrací
   501 `sceniter.transcode_unavailable` do Fáze 4.
6. **Streamování:** `GET|HEAD /sceniter/media/{id}/stream?t=` podle `audio.go`
   (kontrola cesty proti úniku z `VIDEO_ROOT`, `ServeContent`). HLS session
   (media, uživatel, režim, stopa) v `TRANSCODE_ROOT/<id>/`, ffmpeg `-f hls -hls_segment_type fmp4
   -hls_time 4`, master playlist generuje Go; idle → kill + úklid; limit souběžných → 429.
7. **Routy** pod `/api/v1/sceniter`:
   ```
   bez přihlášení: GET /images/{kind}/{id}; GET|HEAD /media/{id}/stream?t=; GET /media/{id}/hls/{file}?t=
   reader+: /movies, /movies/{id}, /shows, /shows/{id}, /shows/{id}/seasons/{n}, /episodes/{id},
            /media/{id}, /media/{id}/playback?profile=, /continue, /recent, /search,
            /progress (GET, PUT /{media_id}, POST /sync, DELETE), /watchlist, /ratings
   editor+: PATCH /movies|shows/{id}, POST|DELETE …/{id}/match, GET /metadata/search
   admin:   /admin/scanner, POST /admin/scanner/rescan, /admin/library/unmatched, /admin/stats,
            /admin/transcode/sessions, /admin/settings
   ```
   Progres: bez `recorded_at` server dosadí „teď“; zápis jen s novějším razítkem;
   `completed` při ≥ 95 % délky; `/progress/sync` deduplikuje přes core `sync_events`.

**Ověření**
```bash
cd server && go test ./internal/sceniter/... && go vet ./...
go test ./internal/sceniter/scanner -run TestParse -v      # tabulka reálných názvů z testovací knihovny
VIDEO_ROOT=… TMDB_API_KEY=… go run ./cmd/server
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/sceniter/movies | jq '.[0]'
curl -H "Authorization: Bearer $TOKEN" "localhost:8080/api/v1/sceniter/media/$ID/playback?profile=web"
curl -sI -H 'Range: bytes=0-1023' "localhost:8080/api/v1/sceniter/media/$ID/stream?t=$ST"   # 206
ffplay "http://localhost:8080/api/v1/sceniter/media/$ID/hls/master.m3u8?t=$ST&mode=remux"
```

## Fáze 3: Sdílené balíčky a `sceniter-frontend`

1. **`core-shared`** (git mv z `libriter-shared`): `client.ts`, `session.ts`, `permissions.ts`,
   `appearance.ts`, obecné části `format.ts`, i18n instance + namespaces `common, auth, errors,
   profile, admin (uživatelé/systém), language, layout`, `scripts/i18n-check.ts` pro všechny balíčky.
   `libriter-shared` si nechá knižní věci a přidá `export * from 'core-shared'`, takže
   **`libriter-mobile` ani `libriter-frontend` se nemění**. Produktové balíčky registrují
   své namespaces přes `i18n.addResourceBundle`.
2. **`sceniter-shared`:** `types.ts` (Movie, Show, Season, Episode, MediaFile, MediaStream,
   PlaybackDecision, WatchProgress), `queryKeys.ts`, `watchProgress.ts`, `profiles.ts`
   (web/safari z UA), i18n `video`.
3. **`core-web`** (React + DOM, bez buildu): git mv z `libriter-frontend/src` `components/ui/*`,
   `lib/utils.ts`, `theme/*`, `auth/*` (AuthContext parametrizovaný `storageKey`, názvem, logem),
   `AuthShell`, přepínače vzhledu a jazyka, `LoginPage`, `RegisterPage`, `ProfilePage`,
   OKLCH paleta z `index.css` → `core-web/src/styles/base.css`. V libriter-frontend zůstanou
   re-export shimy. Tailwind 4 `@source "../../core-web/src"`, Vite `server.fs.allow` na kořen.
4. **`sceniter-frontend`:** skelet podle libriter-frontend (port 5174,
   `outDir ../server/internal/sceniter/web/dist`). Stránky: Home (pokračovat ve sledování,
   nově přidané, watchlist), Filmy + detail, Seriály + detail + sezóna, Přehrávač
   (`<video>` + hls.js, v Safari nativní HLS; výběr audio stopy/titulků = nový dotaz `/playback`),
   Watchlist, Profil, Admin (uživatelé z core-web, knihovna/scanner/unmatched + TMDB match,
   transkódovací session, nastavení, audit). Progres každých 15 s a při pauze/odchodu (`keepalive`).
5. `justfile`: `frontend` sestaví oba weby, nový `dev-sceniter-frontend`; `typecheck` přes všechny workspaces.

**Ověření**
```bash
npm ci && npm run typecheck && npm run i18n:check -w core-shared
just build && LIBRITER_HOST=libriter.localhost SCENITER_HOST=sceniter.localhost ./bin/mediater serve
# prohlížeč: sceniter.localhost:8080 → login stejným účtem jako Libriter → film → přehrání (Chrome i Safari)
# libriter.localhost:8080 funguje beze změny
cd libriter-mobile && npx expo export --platform ios     # mobil se dál bundluje
```

## Fáze 4: Plný transkód, seek, titulky

1. **Transkód:** `libx264 -preset veryfast -crf 22`, max 1080p, klíčové snímky po 4 s,
   AAC stereo. Syntetický VOD playlist z `duration_s` (celá délka hned, seek funguje);
   požadavek na vzdálený segment → restart ffmpeg s `-ss` a `-start_number`
   (čekat na `Wait()` předchozího procesu). Pro `remux` volitelně keyframes z ffprobe při ingestu.
2. **Titulky:** `GET /media/{id}/subtitles/{idx}.vtt?t=` z vestavěné stopy (ffmpeg → webvtt)
   nebo externího souboru (detekce UTF-8/Windows-1250, převod SRT → VTT v Go). Bitmapové
   PGS/VOBSUB v1 nepodporuje. Master playlist přidá `EXT-X-MEDIA TYPE=SUBTITLES`.
3. **Admin:** přehled běžících ffmpeg session a jejich ukončení.

**Ověření:** `go test ./internal/sceniter/stream/... -run 'Playlist|Subtitle'`; přehrání
AVI/xvid v Chrome se seekem; české titulky s diakritikou; třetí souběžný transkód → 429.

## Fáze 5: Deploy a release

1. **Deb `mediater` v2.0.0:** `Replaces: libriter`, `Breaks: libriter (<< 2.0)`, `Provides: libriter`,
   `Depends: ffmpeg`. Binárka `/usr/bin/mediater` + symlink `/usr/bin/libriter`.
   Unit `mediater.service` (`User=libriter`, `ReadWritePaths=/var/lib/libriter`,
   `CacheDirectory=mediater` pro `TRANSCODE_ROOT`). `postinst` převezme `/etc/libriter/libriter.env`,
   zachová `JWT_SECRET` a doplní nové klíče (`LIBRITER_HOST=audiobooks.helpdesk24.works`,
   `SCENITER_HOST=video.helpdesk24.works`, `VIDEO_ROOT`, `SCENITER_IMAGE_ROOT`,
   `TRANSCODE_ROOT`, `TMDB_API_KEY`); vypne starý unit, zapne nový. `VIDEO_ROOT` nesmí být
   pod `/home` kvůli `ProtectHome=true`.
2. **nginx:** sdílený upstream do `mediater-upstream.conf`, nový `video.helpdesk24.works.conf`
   s blokem pro `/api/v1/sceniter/media/.../(stream|hls|subtitles)` (timeouty 1 h, bez access
   logu kvůli `?t=`). DNS + `certbot --nginx -d video.helpdesk24.works`.
3. **Dokumentace:** kořenový README jako rozcestník, detaily Libriteru do `docs/libriter.md`,
   nový `docs/sceniter.md` (konvence názvů souborů, profily přehrávání, proměnné).
   Tento plán uložit jako `docs/sceniter-plan.md`.
4. **Produkce:** záloha DB → `apt install ./mediater_2.0.0_amd64.deb` → v journalu migrace
   `core` a `sceniter/001` → test mobilu, webu Libriteru i Sceniteru.

**Ověření**
```bash
DRY_RUN=1 bash _scripts/release.sh && dpkg -c dist/mediater_*.deb
sudo nginx -t && curl -sI https://video.helpdesk24.works/health
```

## Připraveno pro budoucí TV klienty (teď se nedělá)

- Sceniter API má vlastní prefix a nezávisí na hostname, TV ho použije stejně jako web.
- Profil přehrávání je parametr (`?profile=`); doplní se `tvos` a `androidtv`.
- Video jde přes stejný streamovací token `?t=` jako audio.
- Pro přihlášení se přidá device-code flow (kód na TV, schválení na webu `/link`) a scope `tv`
  podle vzoru `mobile`; volitelně `aud` claim a `users.tokens_revoked_before`.
- Klient vznikne jako workspace `sceniter-tv` (Expo + react-native-tvos + expo-video) a použije
  `sceniter-shared`. Pozor na dvě verze React Native v jednom monorepu (metro `nodeModulesPaths`).

## Rizika

| Riziko | Opatření |
|---|---|
| Kompatibilita distribuované mobilní aplikace | Libriter cesty beze změny, regresní testy `mobile_token`/`sync`, re-exporty v `libriter-shared` |
| Přestavba `schema_migrations` na produkci | jedna transakce, test na kopii produkční DB, záloha před upgradem |
| SQLite s jedním spojením vs. první sken stovek videí | ffprobe mimo transakce, dávky, debounce progresu; případně druhý read-only pool |
| CPU ffmpeg na VM bez GPU | preferovat direct/remux, limit session, `veryfast`, max 1080p |
| Rename rozbije skripty | grep starých cest, `DRY_RUN=1 release.sh`, rozšířit `doctor.sh` |
| Nejednotné názvy souborů v knihovně | tabulkové testy na reálných názvech, stav `unmatched`, ruční match |
| Tailwind/Vite napříč workspaces | `@source`, `server.fs.allow`, jedna verze Reactu přes `overrides` |
| Upgrade deb `libriter` → `mediater` | `Replaces/Breaks/Provides`, zachovat `JWT_SECRET`, krátký plánovaný výpadek |

---

## Odchylky od plánu při implementaci

| Plán | Skutečnost | Proč |
|---|---|---|
| Přestavba `schema_migrations` na `(module, version)` | Nová tabulka `module_migrations`, `schema_migrations` beze změny | Produkční databáze se nepřestavuje; stejný výsledek s menším rizikem. |
| Deb balíček `mediater` s `Replaces/Breaks/Provides: libriter` | Balíček dál `libriter` (2.0.0), binárka `/usr/bin/mediater`, `/usr/bin/libriter` na ni odkazuje | Starý balíček by po náhradě zůstal ve stavu „rc“ a jeho `postrm purge` maže uživatele `libriter`, pod kterým služba běží. Přejmenování je samostatný krok, až bude jméno definitivní. |
| Nové proměnné v dodávaném `libriter.env` | Conffile beze změny proti v1.1.3; proměnné doplní `postinst` z `/usr/share/libriter/env-additions` | Změněný conffile vyvolá dotaz dpkg; volba „nová verze“ by vynulovala `JWT_SECRET` a odhlásila mobilní aplikace. |
| Upstream nginx přejmenovat | Upstream `libriter` přesunut do `libriter-upstream.conf` (conf.d) | Obě domény ho sdílí; dvě definice nginx odmítne. |
| Detailní README Libriteru přesunout do `docs/libriter.md` | README doplněné o Sceniter a monorepo, Libriter zůstal v README | Přesun dlouhého textu bez obsahové změny by jen rozbil odkazy. |
| `core/fswatch` vytažené ze scanneru | Scanner Sceniteru má vlastní sledování změn | Knižní scanner zůstal nedotčený; sjednocení je samostatný refaktor. |
| Lidé a obsazení v tabulkách `sc_people`, `sc_credits` | Nejdřív sloupec `credits_json`; od migrace 004 tabulky `sc_people` a `sc_credits` (přenos při startu, `credits_json` zůstává prázdný) | Zpočátku se obsazení jen zobrazovalo. Stránka osoby a hledání osob tabulky potřebují. Fotky osob jsou v `SCENITER_IMAGE_ROOT` naplocho (`person-<zdroj>-<id>`), ne v podadresáři `profiles`. |
| Plný převod až ve fázi 4, titulky ve fázi 4 | Hotové spolu s backendem (fáze 2) | Správce HLS je od začátku navržený pro všechny režimy. |
| Katalogy jádra jen s obecnými texty | Jádro dostalo celé jmenné prostory `common, format, labels, errors, language, auth, layout, profile, admin` | Dělit pětijazyčné katalogy po klíčích by bylo křehké; několik knižních popisků v jádru je zapsané v README `core-shared`. |
| Správa uživatelů ve webu Sceniteru (`core-web`) | Administrace Sceniteru odkazuje na správu účtů v Libriteru | Stránky uživatelů v Libriteru závisí na knižních hookách; účty jsou stejně společné. |

## Navazující změny (2026-10-10)

- **Knihovny místo pevných složek:** `SCENITER_MOVIES_DIRS`/`SCENITER_SHOWS_DIRS`
  a odhad podle názvu nahradily knihovny v administraci (typ, název, jazyk
  a výchozí zdroj metadat pro každou podsložku `VIDEO_ROOT`). Migrace
  `sceniter/002` tabulky `sc_*` přestavěla a dříve načtená data smazala.
- **Metadata z pluginů:** TMDB už není vestavěné; zdroje metadat Libriteru
  i Sceniteru jsou samostatné programy (`tmdb`, `csfd`, `tvmaze` + knižní
  zdroje), viz [plugins.md](plugins.md). `TMDB_LANGUAGE` zrušeno – jazyk je
  vlastnost knihovny.

## Zbývá

- **Repo na GitHubu** přejmenovat na `mediater` (`gh repo rename`) a upravit
  `origin`; lokální adresář přejmenovat. Nic v kódu na tom nezávisí.
- **Produkce**: postup v [sceniter.md](sceniter.md#nasazení-na-helpdesk24works)
  (záloha DB, balíček, proměnné, DNS, nginx, certbot).
- **Ručně v prohlížeči**: přehrání MKV (remux) a AVI (plný převod) v Chrome
  a Safari, přepnutí zvukové stopy a titulků, odpočet dalšího dílu. Headless
  Chromium v sandboxu neumí H.264, takže obraz ověřen není (segmenty se
  stahují bez chyb).
- **Mobilní aplikace Libriteru** proti nové verzi serveru: přihlášení,
  přehrání kapitoly, synchronizace pozice (API je beze změny a hlídá ho
  `router_test.go`).
- **TMDB** se skutečným klíčem: automatické párování knihovny, plakáty,
  ruční spárování seriálu včetně dílů.
