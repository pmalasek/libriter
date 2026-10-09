# Sceniter – filmy a seriály

Sceniter je druhý produkt na stejném serveru jako Libriter. Sdílí s ním účty,
přihlášení, role, administraci uživatelů a nasazení (jedna binárka, jedna
databáze, jeden balíček `.deb`). Má vlastní webové rozhraní na vlastní doméně
a vlastní API pod `/api/v1/sceniter/`.

## Zapnutí

Sceniter je vypnutý, dokud není nastavený `VIDEO_ROOT`. Minimální konfigurace
v `/etc/libriter/libriter.env` (vývoj: `server/.env`):

```bash
VIDEO_ROOT=/srv/video                 # adresář s filmy a seriály (stačí čtení)
SCENITER_HOST=video.helpdesk24.works  # host, na kterém se ukáže web Sceniteru
LIBRITER_HOST=audiobooks.helpdesk24.works
TMDB_API_KEY=…                        # nepovinné, bez něj nejsou plakáty ani popisy
```

Po změně `sudo systemctl restart libriter`. Při startu server aplikuje migrace
modulu `sceniter` (tabulky `sc_*`) a projde knihovnu.

Všechny proměnné:

| Proměnná | Výchozí | Význam |
|---|---|---|
| `VIDEO_ROOT` | – | Kořen videoknihovny. Prázdný = Sceniter vypnutý. |
| `SCENITER_MOVIES_DIRS` | `Filmy,Movies` | Podadresáře `VIDEO_ROOT` s filmy. |
| `SCENITER_SHOWS_DIRS` | `Seriály,Serialy,Shows,Series,TV` | Podadresáře se seriály. |
| `SCENITER_IMAGE_ROOT` | vedle `COVER_ROOT`: `sceniter-images` | Plakáty a obrázky z TMDB. |
| `SCENITER_RESCAN_MINUTES` | `0` | Pravidelný průchod knihovnou (síťové disky). |
| `TMDB_API_KEY` | – | Klíč API v3 nebo čtecí token v4 z themoviedb.org. |
| `TMDB_LANGUAGE` | `cs-CZ` | Jazyk metadat; chybějící popis se doplní z angličtiny. |
| `TRANSCODE_ROOT` | dočasný adresář | Pracovní adresář převodů (balíček: `/var/cache/libriter/transcode`). |
| `TRANSCODE_MAX_SESSIONS` | `2` | Kolik videí se smí převádět najednou. |
| `TRANSCODE_IDLE_SECONDS` | `60` | Převod, na který nikdo nesáhl, se ukončí a smaže. |
| `TRANSCODE_MAX_HEIGHT` | `1080` | Strop výšky obrazu při plném převodu. |
| `LIBRITER_HOST`, `SCENITER_HOST` | – | Hosty webů (čárkou oddělené). |
| `DEFAULT_PRODUCT` | `libriter` | Web pro ostatní hosty (IP adresa, localhost). |

`VIDEO_ROOT` nesmí ležet pod `/home` – služba běží s `ProtectHome=true`. Leží-li
na síťovém disku, přidejte do služby `RequiresMountsFor=` (viz komentář
v `libriter.service`) a zapněte `SCENITER_RESCAN_MINUTES`, protože sledování
změn (fsnotify) na NFS a SMB nefunguje. Když `VIDEO_ROOT` při průchodu chybí
(disk není připojený), scanner nic nemaže.

## Uspořádání souborů

Scanner zařazuje podle adresáře, ne podle obsahu:

```
VIDEO_ROOT/
├── Filmy/
│   ├── 21 gramů (2003 - Alejandro González Iñárritu).avi    → „21 gramů“, 2003
│   ├── Křídla vášně - The Wings of the Dove (1997).avi      → „Křídla vášně“ / „The Wings of the Dove“, 1997
│   ├── Křídla vášně - The Wings of the Dove (1997).srt      → titulky k filmu výš
│   ├── 22.míle.2018.Mile.22.BRRip.720p.x264.AC3.CZ.mkv      → „22 míle“, 2018
│   └── Pelíšky (1999)/pelisky.mkv                            → film v adresáři: název a rok z adresáře
└── Seriály/
    └── Docent/                                               → seriál „Docent“
        ├── S01/Docent - 01x02 - Preso s mlíčkem.mkv          → 1. řada, 2. díl „Preso s mlíčkem“
        └── S03/Docent - 02. díl - Příliš mnoho podezřelých.mkv → 3. řada (z adresáře), 2. díl
```

- **Filmy**: soubor naplocho je film sám; soubor v podadresáři patří filmu
  toho adresáře (více souborů = více verzí, přehrává se největší). Název končí
  rokem v závorce nebo prvním rokem či značkou vydání (`720p`, `BRRip`,
  `x264`, `CZ`, `Dabing`…). „Český název - Original Title“ se rozdělí na název
  a původní název.
- **Seriály**: první podadresář je seriál, další úroveň řada (`S03`,
  `Season 3`, `Série 3`, `3. řada`). Díl se pozná podle `S01E02`, `1x02`,
  `02. díl` nebo `Díl 2`; díl bez čísla dostane pořadí podle názvu souboru.
- Soubor mimo tyto adresáře: `SxxEyy` nebo `NxNN` v názvu je díl, jinak film.
- **Titulky** vedle videa se stejným začátkem názvu: `Film.srt`, `Film.cs.srt`,
  `Film.forced.cze.srt`. Formáty SRT, VTT, ASS/SSA. České titulky ve
  Windows-1250 se převedou správně.
- Přeskakuje se `@eaDir`, `#recycle`, `.vsmeta` (Synology), `*.part`, skryté
  soubory a ukázky (`sample`).

Ručně opravený název (editor, tlačítko Upravit) scanner nepřepíše; soubor,
který už v knihovně je, se nikdy nepřeřadí jinam.

## Metadata z TMDB

S nastaveným `TMDB_API_KEY` se po každém průchodu knihovnou zkusí spárovat
nespárované tituly. Automaticky se párují jen jisté shody: stejný rok (±1)
a stejný název bez diakritiky, nebo jediný výsledek se shodným názvem. Zbytek
zůstane „nespárovaný“ a admin ho spáruje ručně v **Administrace → Párování**
(nebo editor tlačítkem *Spárovat s TMDB* v detailu). Dvě verze téhož filmu se
po spárování sloučí do jednoho titulu. *Nepárovat* titul vyřadí z automatického
párování.

Podmínky TMDB vyžadují uvést zdroj – web to dělá v patičce stránek.

## Přehrávání

Prohlížeč pošle svůj profil (`web` pro Chrome, Firefox a Edge, `safari` pro
Safari) a server zvolí nejlevnější režim:

| Režim | Kdy | Zátěž serveru |
|---|---|---|
| přímé přehrání | MP4/WebM s kodeky, které prohlížeč umí | žádná (soubor s Range) |
| přebalení (remux) | MKV s H.264 a AAC (např. díly Docenta) | nízká (ffmpeg `-c copy`) |
| převod zvuku | obraz v pořádku, zvuk AC3/DTS (CZ dabing v Chrome) | nízká |
| plný převod | AVI/Xvid, MPEG-4, HEVC mimo Safari, 10bit H.264 | vysoká (libx264) |

Převody běží jako HLS (fMP4 segmenty po 6 s) v `TRANSCODE_ROOT`. Plný převod má
playlist s celou délkou filmu: přehrávač zná délku a skok dopředu restartuje
převod od zvoleného místa. U přebalení a převodu zvuku playlist během převodu
roste: přetáčet jde do už převedené části, která přibývá rychlostí čtení disku
(celý film obvykle za desítky sekund). Když přímé přehrání v
prohlížeči selže, přehrávač sám zkusí přebalení a pak plný převod.

Zvukové stopy se přepínají v přehrávači (nový převod od stejného místa).
Titulky: textové vložené stopy i soubory vedle videa se převádějí do WebVTT;
obrázkové (PGS, VobSub) zobrazit nejdou.

Běžící převody vidí admin v **Administrace → Převody** a může je ukončit.

Rozkoukanost se ukládá každých 10 s, při pauze a při odchodu ze stránky.
Platí novější záznam (`recorded_at`), takže starší událost z jiného zařízení
novější pozici nepřepíše. Dokoukáno je od 90 % délky nebo méně než 3 minuty
před koncem.

## API

Vše pod `/api/v1/sceniter`. Přehrávací cesty nesou streamovací token v adrese
(`?t=`, vydává ho `GET /auth/stream-token` nebo rovnou `/playback`).

| Metoda a cesta | Role | Popis |
|---|---|---|
| `GET /config` | přihlášený | Je Sceniter zapnutý, TMDB, převody |
| `GET /home` | reader | Pokračovat, nové filmy a díly, seznam |
| `GET /movies`, `/shows` | reader | Výpis (`?q=&genre=&sort=title\|year\|added\|rating`) |
| `GET /movies/{id}`, `/shows/{id}`, `/episodes/{id}` | reader | Detail se soubory a stavem uživatele |
| `GET /genres`, `/search?q=` | reader | Žánry, hledání |
| `GET /media/{id}/playback?profile=&audio=&mode=` | reader | Rozhodnutí o přehrávání a adresy |
| `GET /media/{id}/stream?t=` | token | Soubor tak, jak je (Range) |
| `GET /media/{id}/hls/master.m3u8?t=&mode=&audio=` | token | Spustí převod, master playlist |
| `GET /hls/{session}/{file}?t=` | token | Media playlist, `init.mp4`, segmenty |
| `DELETE /hls/{session}?t=`, `POST /hls/{session}/stop?t=` | token | Ukončí převod |
| `GET /media/{id}/subtitles/{key}.vtt?t=` | token | Titulky ve WebVTT (`s<index>` / `x<uuid>`) |
| `GET /images/{name}` | veřejné | Plakát nebo obrázek |
| `GET /progress`, `PUT /progress` | reader | Rozkoukanost; 409 = na serveru je novější |
| `POST /progress/sync` | reader | Dávka s `event_id` (offline klienti) |
| `PUT /progress/{movie\|episode}/{id}/watched`, `DELETE /progress/…` | reader | Zhlédnuto / nezhlédnuto |
| `GET /watchlist`, `PUT\|DELETE /watchlist/{movie\|show}/{id}` | reader | Seznam „chci vidět“ |
| `PUT /ratings/{movie\|show}/{id}` | reader | Hodnocení 1–10 nebo `null` |
| `PATCH /movies/{id}`, `/shows/{id}` | editor | Ruční úprava názvu, roku, popisu |
| `GET /metadata/search?kind=&q=&year=` | editor | Hledání v TMDB |
| `PUT\|DELETE /movies/{id}/match`, `PUT /movies/{id}/ignore` (i `shows`) | editor | Párování s TMDB |
| `GET /admin/scanner`, `POST /admin/scanner/rescan` | admin | Stav a spuštění průchodu |
| `POST /admin/metadata/match-all` | admin | Automatické párování |
| `GET /admin/library/unmatched`, `GET /admin/stats` | admin | Nespárované tituly, statistiky |
| `GET /admin/transcode`, `DELETE /admin/transcode/{session}` | admin | Běžící převody |

Chyby mají stejný tvar jako v Libriteru: `{"error": "…", "code": "sceniter.disabled"}`.

## Nasazení na helpdesk24.works

Pořadí kroků při prvním nasazení verze 2.0:

1. **Záloha databáze** na backendu (172.24.0.46):
   `sudo systemctl stop libriter && sudo cp -a /var/lib/libriter/libriter.db{,.pre-2.0}`
   (a soubory `-wal`/`-shm`, pokud existují), pak `sudo systemctl start libriter`.
2. **Balíček**: `sudo apt install ./libriter_2.0.0_amd64.deb`. Konfigurace
   zůstane, na její konec se doplní nové proměnné.
3. **Konfigurace** v `/etc/libriter/libriter.env`: `LIBRITER_HOST`,
   `SCENITER_HOST`, `VIDEO_ROOT`, případně `TMDB_API_KEY`; pak
   `sudo systemctl restart libriter` a `journalctl -u libriter -f` (migrace
   `module=sceniter`, průchod videoknihovnou).
4. **DNS**: záznam A `video.helpdesk24.works` → 78.156.131.251.
5. **nginx** (78.156.131.251): upstream se přesouvá do samostatného souboru,
   protože ho sdílí obě domény.
   ```bash
   sudo cp deploy/nginx/libriter-upstream.conf /etc/nginx/conf.d/
   sudo cp deploy/nginx/audiobooks.helpdesk24.works.conf deploy/nginx/video.helpdesk24.works.conf /etc/nginx/sites-available/
   sudo ln -s /etc/nginx/sites-available/video.helpdesk24.works.conf /etc/nginx/sites-enabled/
   sudo nginx -t && sudo systemctl reload nginx
   sudo certbot --nginx -d video.helpdesk24.works
   ```
   Nový `audiobooks…conf` už upstream neobsahuje – když se zkopíruje bez
   `libriter-upstream.conf`, `nginx -t` selže na neznámém upstreamu; když se
   naopak ponechá starý soubor s upstreamem, selže na duplicitě. Certbot
   přidal do starého souboru blok `:443` – před kopírováním ho přeneste, nebo
   po kopírování spusťte `certbot --nginx -d audiobooks.helpdesk24.works` znovu.
6. **Ověření**: `curl -sI https://video.helpdesk24.works/health`, přihlášení
   stejným účtem jako do Libriteru, přehrání filmu; mobilní aplikace Libriteru
   (přihlášení, přehrání kapitoly, synchronizace pozice).

## Omezení

- Obrázkové titulky (PGS, VobSub) se nezobrazí.
- Plný převod je softwarový (libx264, `veryfast`); na serveru bez GPU zvládne
  jen několik současných převodů – proto `TRANSCODE_MAX_SESSIONS`.
- Při přebalení (MKV → HLS) se seká jen na klíčových snímcích, segmenty proto
  nemají přesně 6 s; prohlížeče to zvládají.
- Přihlášení je na každé doméně zvlášť (vlastní úložiště prohlížeče), účet je
  ale stejný.
- Uživatele a role spravuje administrace Libriteru.
- TV aplikace (Apple TV, Google TV) zatím nejsou; API, profily přehrávání,
  token v adrese a synchronizace pozic s `recorded_at` jsou na ně připravené.
