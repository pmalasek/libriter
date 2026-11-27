# Sceniter – filmy a seriály

Sceniter je druhý produkt na stejném serveru jako Libriter. Sdílí s ním účty,
přihlášení, role, administraci uživatelů a nasazení (jedna binárka, jedna
databáze, jeden Docker image nebo balíček `.deb`). Má vlastní webové rozhraní na vlastní doméně
a vlastní API pod `/api/v1/sceniter/`.

## Zapnutí

Sceniter je vypnutý, dokud není nastavený `VIDEO_ROOT`. Minimální konfigurace
v `.env` (Docker: v adresáři instance, balíček: `/etc/libriter/libriter.env`,
vývoj: `server/.env`):

```bash
VIDEO_ROOT=/srv/video                 # adresář s filmy a seriály (stačí čtení)
SCENITER_HOST=video.helpdesk24.works  # host, na kterém se ukáže web Sceniteru
LIBRITER_HOST=audiobooks.helpdesk24.works
TMDB_API_KEY=…                        # nepovinné, výchozí klíč pluginu TMDB
```

Po změně `sudo systemctl restart libriter`. Při startu server aplikuje migrace
modulu `sceniter` (tabulky `sc_*`). Pak admin v **Administrace → Knihovny**
vybere podsložky `VIDEO_ROOT`, které se mají procházet (viz
[Knihovny](#knihovny)).

Všechny proměnné:

| Proměnná | Výchozí | Význam |
|---|---|---|
| `VIDEO_ROOT` | – | Kořen videoknihovny. Prázdný = Sceniter vypnutý. |
| `SCENITER_IMAGE_ROOT` | vedle `COVER_ROOT`: `sceniter-images` | Plakáty a obrázky ze zdrojů metadat. |
| `SCENITER_RESCAN_MINUTES` | `0` | Pravidelný průchod knihovnou (síťové disky). |
| `TMDB_API_KEY` | – | Výchozí klíč pluginu TMDB (v3 nebo čtecí token v4); klíč uložený v administraci má přednost. |
| `PLUGIN_DIR` | `plugins/` vedle binárky, jinak `/usr/lib/mediater/plugins` | Pluginy metadat, viz [plugins.md](plugins.md). |
| `TRANSCODE_ROOT` | dočasný adresář | Pracovní adresář převodů (balíček: `/var/cache/libriter/transcode`). |
| `TRANSCODE_MAX_SESSIONS` | `2` | Kolik videí se smí převádět najednou. |
| `TRANSCODE_IDLE_SECONDS` | `60` | Převod, na který nikdo nesáhl, se ukončí a smaže. |
| `TRANSCODE_MAX_HEIGHT` | `1080` | Strop výšky obrazu při plném převodu. |
| `LIBRITER_HOST`, `SCENITER_HOST` | – | Hosty webů (čárkou oddělené). |
| `DEFAULT_PRODUCT` | `libriter` | Web pro ostatní hosty (IP adresa, localhost). |

`VIDEO_ROOT` nesmí ležet pod `/home` – služba běží s `ProtectHome=true`. Leží-li
na síťovém disku, přidejte do služby `RequiresMountsFor=` (viz komentář
v `libriter.service`) a zapněte `SCENITER_RESCAN_MINUTES`, protože sledování
změn (fsnotify) na NFS a SMB nefunguje. Když `VIDEO_ROOT` nebo složka knihovny
při průchodu chybí (disk není připojený, složka přejmenovaná), scanner v ní
nic nemaže.

## Knihovny

Knihovna je podsložka přímo pod `VIDEO_ROOT`. Nastavuje ji admin v
**Administrace → Knihovny** (platí pro všechny uživatele):

| Vlastnost | Význam |
|---|---|
| Složka | Podsložka `VIDEO_ROOT`; nabízejí se jen ty, které knihovnou ještě nejsou. |
| Typ | Filmy, nebo seriály – určuje, jak scanner soubory zařadí. Po založení ho nejde změnit. |
| Zobrazovaný název | Položka v menu a nadpis řádku na domovské stránce. |
| Ikona | Ikona v menu z nabídky; bez volby podle typu (filmový pás / televize). |
| Jazyk metadat | Jazyk, ve kterém zdroj vrací názvy a popisy (ISO 639-1). |
| Výchozí zdroj metadat | Plugin, se kterým se tituly párují automaticky; bez zdroje se nepárují. |
| Zapnutá | Vypnutá knihovna se neprochází ani nezobrazuje, data zůstávají. |

Například:

| Složka | Typ | Zdroj |
|---|---|---|
| Filmy | filmy | ČSFD |
| Dokumenty | filmy | ČSFD |
| Seriály | seriály | TMDB |
| Dokumentární cykly | seriály | ČSFD |

Pořadí knihoven (šipky) je pořadí v menu a na domovské stránce. Smazání
knihovny smaže z databáze její tituly včetně rozkoukanosti a hodnocení;
soubory na disku zůstávají. Soubory mimo knihovny scanner ignoruje.

## Uspořádání souborů

Scanner zařazuje podle typu knihovny, ne podle obsahu:

```
VIDEO_ROOT/
├── Filmy/                                                    → knihovna typu „filmy“
│   ├── 21 gramů (2003 - Alejandro González Iñárritu).avi    → „21 gramů“, 2003
│   ├── Křídla vášně - The Wings of the Dove (1997).avi      → „Křídla vášně“ / „The Wings of the Dove“, 1997
│   ├── Křídla vášně - The Wings of the Dove (1997).srt      → titulky k filmu výš
│   ├── 22.míle.2018.Mile.22.BRRip.720p.x264.AC3.CZ.mkv      → „22 míle“, 2018
│   ├── Pelíšky (1999).cd1.avi, Pelíšky (1999).cd2.avi         → jeden film „Pelíšky“, 1999 (dva díly)
│   └── Divadelní hry/Lakomec.avi                              → „Lakomec“; podsložka jen třídí
└── Seriály/                                                  → knihovna typu „seriály“
    └── Docent/                                               → seriál „Docent“
        ├── S01/Docent - 01x02 - Preso s mlíčkem.mkv          → 1. řada, 2. díl „Preso s mlíčkem“
        └── S03/Docent - 02. díl - Příliš mnoho podezřelých.mkv → 3. řada (z adresáře), 2. díl
```

- **Knihovna filmů**: každý soubor je film sám, i v podsložce. Podsložky
  („Divadelní hry“, „Detektivky Hany Proškové“) jen třídí a do názvu ani
  roku nevstupují. Díly jednoho filmu ve stejné složce (`cd1`/`cd2`,
  `part1`, `disk1`) jsou jeden film. Název končí
  rokem v závorce nebo prvním rokem či značkou vydání (`720p`, `BRRip`,
  `x264`, `CZ`, `Dabing`…). „Český název - Original Title“ se rozdělí na název
  a původní název. Úvodní značky (`!(Dokument)-`, `[CZ]`) se zahodí a název
  bez mezer s pomlčkami (`Nigel-Marven-a-Invaze`) se rozdělí na slova.
- **Knihovna seriálů**: první podadresář je seriál (pořadové číslo
  `02. ` na začátku se zahodí), další úroveň řada (`S03`, `Season 3`,
  `Série 3`, `3. řada`). Díl se pozná podle `S01E02`, `1x02`, `e01`,
  `02. díl` nebo `Díl 2`; díl bez čísla dostane pořadí podle názvu souboru.
  Díl ležící přímo v knihovně patří seriálu podle textu před číslem dílu
  (`Show.Name.S02E05.mkv`). Rok seriálu se vezme z adresáře, jinak z názvu dílu.
- **Titulky** vedle videa se stejným začátkem názvu: `Film.srt`, `Film.cs.srt`,
  `Film.forced.cze.srt`. Formáty SRT, VTT, ASS/SSA. České titulky ve
  Windows-1250 se převedou správně.
- Přeskakuje se `@eaDir`, `#recycle`, `.vsmeta` (Synology), `*.part`, skryté
  soubory a ukázky (`sample`).

Ručně opravený název (editor, tlačítko Upravit) scanner nepřepíše; soubor,
který už v knihovně je, se nepřeřadí jinam. Výjimkou jsou filmy založené
starší verzí podle podsložky: ty se při průchodu rozdělí na filmy podle
souborů (film s jediným souborem si nechá metadata, sdružený se páruje znovu).

## Metadata

Metadata dodávají **pluginy** – samostatné programy, které server spouští pro
každý dotaz (protokol: [plugins.md](plugins.md)). Balíček obsahuje:

| Plugin | Filmy | Seriály | Poznámka |
|---|---|---|---|
| `csfd` | ✓ | ✓ | ČSFD, česky; scraping webu (odstup 2 s), řady a názvy dílů, hodnocení v %. |
| `tmdb` | ✓ | ✓ | The Movie Database, libovolný jazyk; potřebuje API klíč. Dává i IMDb ID. |
| `tvmaze` | – | ✓ | TVmaze, anglicky, bez klíče; dává IMDb a TheTVDB ID. |

IMDb samostatný zdroj nemá: oficiální API je placené a web blokuje scraping.
ID z IMDb ukládají `tmdb` a `tvmaze`, detail titulu na něj (a na ostatní
databáze) odkazuje. Klíče a nastavení pluginů jsou v **Administrace → Zdroje
metadat**; tam jde i znovu načíst adresář pluginů bez restartu serveru.

Po každém průchodu knihovnou se nespárované tituly zkusí spárovat se zdrojem
knihovny v jejím jazyce. Když zdroj knihovny jistou shodu nenajde, zkusí se
postupně ostatní nainstalované zdroje, které daný druh umí (zdroj, který
právě odmítá požadavky, titul jen odloží). Automaticky se párují jen jisté shody: stejný rok
(±1) a stejný název bez diakritiky, nebo jediný výsledek se shodným názvem.
Zbytek zůstane „nespárovaný“ a admin ho spáruje ručně v **Administrace →
Párování** (nebo editor tlačítkem *Spárovat se zdrojem metadat* v detailu),
přičemž může zvolit i jiný zdroj, než má knihovna. Titul, pro který žádný zdroj
jistou shodu nenašel (v administraci „nenalezeno“), se automaticky znovu hledá až po 7 dnech (šetří
požadavky na zdroj); hned ho zkusí **Spárovat vše** v administraci nebo
vrácení titulu do stavu „nespárovaný“. Dvě verze téhož filmu ve
stejné knihovně se po spárování sloučí do jednoho titulu. *Nepárovat* titul
vyřadí z automatického párování.

**Obsazení a štáb** se ukládají jako osoby: klik na herce nebo režiséra v
detailu titulu otevře jeho stránku s filmy a seriály z knihovny. TMDB a
TVmaze dodávají i fotky osob (soubory `person-<zdroj>-<id>` v
`SCENITER_IMAGE_ROOT`), ČSFD jen ID tvůrce. Osoby se stejným jménem z
různých zdrojů sdílejí stránku i fotku. Po přechodu na verzi s osobami mají
dříve spárované tituly obsazení jen podle jmen a bez fotek – doplní je
**Administrace → Přehled → Obnovit metadata**, která znovu načte všechny
spárované tituly z jejich zdroje (u ČSFD s odstupem 2 s na dotaz). Obnova
přepíše ruční úpravy názvu a popisu, stejně jako nové spárování.

**Velké knihovny** (tisíce titulů): pro první párování nastavte knihovnám
zdroj `tmdb` – oficiální API se štědrým limitem. ČSFD je scraping webu
s ochranou proti botům; párování probíhá po jednom titulu s odstupem 2 s,
seriál stojí stránku za každý díl na disku. Když zdroj začne odmítat
požadavky, server ho pozastaví (1 až 30 minut, viz [plugins.md](plugins.md))
a párování pak samo pokračuje; v logu je to hláška *zdroj odmítá požadavky*.
ČSFD je vhodné spíš na ruční dopárování jednotlivých titulů.

Podmínky TMDB vyžadují uvést zdroj – web to dělá v patičce stránek, pokud je
plugin TMDB nainstalovaný.

## Přehrávání

Klient pošle svůj profil (`web` pro Chrome, Firefox a Edge, `safari` pro
Safari, `tvos` pro Apple TV, `androidtv` pro Google TV) a server zvolí
nejlevnější režim:

| Režim | Kdy | Zátěž serveru |
|---|---|---|
| přímé přehrání | MP4/WebM s kodeky, které prohlížeč umí | žádná (soubor s Range) |
| přebalení (remux) | MKV s H.264 a AAC (např. díly Docenta) | nízká (ffmpeg `-c copy`) |
| převod zvuku | obraz v pořádku, zvuk AC3/DTS (CZ dabing v Chrome) | nízká |
| plný převod | AVI/Xvid, MPEG-4, HEVC mimo Safari, 10bit H.264 | vysoká (libx264) |

Převody běží jako HLS (fMP4 segmenty po 6 s) v `TRANSCODE_ROOT`. Plný převod má
playlist s celou délkou filmu: přehrávač zná délku a skok dopředu restartuje
převod od zvoleného místa. U přebalení a převodu zvuku playlist během převodu
roste a přibývá rychlostí čtení disku (celý film obvykle za desítky sekund).
Skok mimo už převedenou část (i pokračování v rozkoukaném) spustí převod znovu
od klíčového snímku před cílem: TV klient pošle `start=` v adrese master
playlistu a čas skutečného začátku si přečte z `#EXT-X-SESSION-DATA`
`cz.sceniter.start` (nula časové osy nového playlistu). Když přímé přehrání v
prohlížeči selže, přehrávač sám zkusí přebalení a pak plný převod.

Zvukové stopy se přepínají v přehrávači (nový převod od stejného místa).
Poměr stran jde v přehrávači přepnout ručně (Automaticky / 4:3 / 16:9) pro
soubory, které ho mají zapsaný špatně (např. 4:3 obraz uložený jako 768×432
se čtvercovými pixely). Volba se uloží k filmu či seriálu a platí pro všechny.
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
| `GET /config` | přihlášený | Je Sceniter zapnutý, zdroje metadat, převody |
| `GET /home` | reader | Pokračovat, nově přidané po knihovnách, seznam |
| `GET /libraries` | reader | Zapnuté knihovny v pořadí (menu) |
| `GET /movies`, `/shows` | reader | Výpis (`?library=&q=&genre=&sort=title\|year\|added\|rating`) |
| `GET /movies/{id}`, `/shows/{id}`, `/episodes/{id}` | reader | Detail se soubory, obsazením a stavem uživatele |
| `GET /movies/{id}/similar`, `/shows/{id}/similar` | reader | Podobné tituly (stejná knihovna, společné žánry) |
| `GET /people/{id}` | reader | Osoba a její filmy a seriály v knihovně |
| `GET /genres`, `/search?q=` | reader | Žánry, hledání (tituly i osoby) |
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
| `PUT /aspect/{movie\|show}/{id}` | reader | Ruční poměr stran pro přehrávač `4:3` / `16:9` nebo `null` (zpět na výchozí); platí jen pro volajícího uživatele |
| `PATCH /movies/{id}`, `/shows/{id}` | editor | Ruční úprava názvu, roku, popisu |
| `GET /metadata/providers` | editor | Zdroje video metadat (pluginy) |
| `GET /metadata/search?kind=&q=&year=&provider=&language=&title=` | editor | Hledání u zdroje; bez `provider` zdroj knihovny titulu |
| `PUT /movies/{id}/match` `{"provider","id"}`, `DELETE …/match`, `PUT …/ignore` (i `shows`) | editor | Párování se zdrojem |
| `GET\|POST /admin/libraries`, `PATCH\|DELETE /admin/libraries/{id}` | admin | Správa knihoven |
| `GET /admin/libraries/candidates`, `PUT /admin/libraries/order` | admin | Volné složky `VIDEO_ROOT`, pořadí knihoven |
| `GET /admin/scanner`, `POST /admin/scanner/rescan` | admin | Stav a spuštění průchodu |
| `POST /admin/metadata/match-all`, `GET /admin/metadata/match` | admin | Automatické párování, jeho průběh |
| `POST /admin/metadata/refresh-all`, `GET /admin/metadata/refresh` | admin | Obnova metadat spárovaných titulů, její průběh |
| `GET /admin/library/unmatched`, `GET /admin/stats` | admin | Nespárované tituly, statistiky |
| `GET /admin/transcode`, `DELETE /admin/transcode/{session}` | admin | Běžící převody |
| `GET /admin/watching` | admin | Přehled sledování po uživatelích: celkový čas, počet titulů, poslední sledování |
| `GET /admin/watching/{userId}?days=90` | admin | Sledování jednoho uživatele po dnech: titul (u epizody seriál, řada a díl), sekundy, první a poslední čas |

Poměr stran nastavený uživatelem se ukládá zvlášť pro každého
(`sc_user_aspect`); bez vlastní volby platí sdílená hodnota titulu, jinak
poměr podle souboru. Tvar API se tím nezměnil.

Statistika sledování (`sc_watch_history`) se zapisuje s každým uložením
pozice – titul × den se součtem sekund a časem prvního a posledního
sledování. Zpětně se nedopočítává; data přibývají až od verze, která ji
přidala. „Označit jako nezhlédnuté“ historii nemaže, smazání uživatele ano.

Chyby mají stejný tvar jako v Libriteru: `{"error": "…", "code": "sceniter.disabled"}`.

## TV aplikace (Apple TV, Google TV)

Jen přehrávač – prohlížení knihoven, detail, hledání a přehrávání, žádné úpravy.
Zdroj je v `sceniter-tv/` (Expo + react-native-tvos + expo-video, jedna
codebase pro obě platformy). Nejde o npm workspace: fork React Native by se
v jednom stromu pobil s mobilem Libriteru, proto má vlastní `node_modules`
a `package-lock.json`; `sceniter-shared` a `core-shared` bere Metro přímo ze
zdrojů. Recepty: `just tv-start`, `tv-android`, `tv-ios`, `tv-apk`, `tv-aab`,
`tv-testflight`.

**Ovládání.** Boční menu má Hledat, Domů, každou knihovnu zvlášť (název
a ikona z administrace, pořadí jako na webu) a Nastavení. Výpis knihovny má
jediné tlačítko řazení; volba se pamatuje v televizi pro každou knihovnu.
OK v menu otevře sekci a předá fokus do ní. Zpět v sekci zaměří menu, z menu
vede na Domů, z Domů ven z aplikace.

**Přehrávač.** Fokus drží posuvník: každý stisk šipky vlevo/vpravo přetočí
o 10 s (video skočí po chvíli bez stisku), držení posouvá posuvník plynule
a zrychluje s délkou držení (10 → 30 → 60 s za 0,2 s), video skočí hned po
puštění. Přetáčet jde po celé délce, i do části, kterou server ještě
nepřevedl. OK pozastaví. Dolů vede na tlačítka Zvuk a titulky, Od
začátku a Další díl. Zpět nejdřív skryje lištu, pak přehrávač zavře. V poslední
minutě dílu se nabídne další díl, po dohrání běží odpočet. React Native tvOS
posílá na Androidu do JS jen uvolnění klávesy, Přehrát/Pauzu obslouží media
session přehrávače.

**Adresa serveru.** Posledních pět serverů se pamatuje a nabízí jako
tlačítka. Kterým protokolem se televize připojí, záleží na zadání:

| Zadání | Co se zkusí |
|---|---|
| bez schématu, adresa v místní síti (privátní IPv4/IPv6, `.local`, `.lan`, `.home.arpa`, jednoslovný název) | `https://`, když selže, `http://` |
| bez schématu, veřejná doména nebo IP | jen `https://`; když selže, hláška poradí napsat `http://` výslovně |
| `https://…` | jen HTTPS |
| `http://…` | jen HTTP – přijme se, ale televize ukazuje varování o nešifrovaném spojení (i u uloženého serveru) |

Bez HTTPS putuje heslo i token čitelně po síti; doma za routerem je to
přijatelné, přes internet ne. Proto veřejná adresa nikdy nepřejde na HTTP
potichu. Cleartext v `app.json` zůstává povolený – Android neumí povolit jen
rozsahy místních IP. Mobilní aplikace Libriteru doplní k adrese bez schématu
`https://`, `http://` k veřejné adrese přijme až po potvrzení varování.

**Párování.** Televize se nepřihlašuje heslem. Po zadání adresy serveru ukáže
kód a QR; uživatel ho na telefonu nebo počítači potvrdí na stránce `/link`
(web Sceniteru i Libriteru) a televize dostane token se scope `tv` (platí jako
mobilní token, 365 dní). Stránka `/link` upozorňuje, ať uživatel párování
schválí jen tehdy, když kód vidí na své televizi – kdo kód podstrčí, dostal
by jinak přístup k účtu. Kód platí 10 minut a leží v databázi (tabulka
`device_codes`, jen hash `device_code`), takže párování přežije restart
serveru; prošlý kód televize sama nahradí novým.

Token televize smí jen to, co čtenář (i u účtu admina), a je to relace
zařízení: uživatel ji vidí ve webovém **Profilu → Zařízení** pod názvem
z párování a může ji odvolat, stejně jako **Odhlásit všude** nebo změna hesla.
Odvolaná televize dostane `401` a musí se spárovat znovu.

| Metoda a cesta | Role | Popis |
|---|---|---|
| `POST /api/v1/auth/device/start` `{"device_name"}` | veřejné | `device_code`, `user_code` (ABCD-EFGH), `verification_path`, `interval`; nejvýš 10 za 10 minut na IP |
| `POST /api/v1/auth/device/token` `{"device_code"}` | veřejné | 428 `auth.device_pending`, 429 `auth.device_slow_down`, 410 `auth.device_expired`, jinak `{token, user}`; nejvýš 60 za minutu na IP |
| `GET /api/v1/auth/device/{user_code}` | webová relace | Které zařízení se páruje |
| `POST /api/v1/auth/device/approve` `{"user_code"}` | webová relace | Schválí párování |

„Webová relace“ je přihlášení z prohlížeče; token mobilu nebo televize tu
dostane `403` `auth.web_session_required`, aby si zařízení nemohlo samo
schválit další token. Překročení limitu na IP vrací `429` `auth.rate_limited`
s `Retry-After`.

**Přehrávání** sdílí pravidla s webem (`sceniter-shared/src/playback.ts`):
eskalace režimu při chybě, keepalive převodu, ukládání pozice. U přebalení
čeká televize s pokračováním, až playlist k místu dojede. Titulky vykresluje
aplikace sama z WebVTT (AVPlayer externí titulky k přímo přehrávanému souboru
neumí), takže vypadají na obou platformách stejně.

## Nasazení na helpdesk24.works

Libriter i Sceniter běží v jednom Docker kontejneru na serveru 172.24.0.46,
produkce i demo každé zvlášť. Nginx s TLS běží na tomtéž serveru a proxuje
na backend přes `127.0.0.1` (weby Libriteru) i `172.24.0.46` (weby Sceniteru
přes upstream); compose proto publikuje port na obou adresách:

| Instance | Adresář na serveru | Port (127.0.0.1 a 172.24.0.46) | Domény |
|---|---|---|---|
| produkce | `/opt/libriter` | `8081` | Libriter a Sceniter na helpdesk24.works a pem-tech.eu (seznam v hlavičce compose) |
| demo | `/opt/libriter-demo` | `8082` | libriter.demo.helpdesk24.works, sceniter.demo.helpdesk24.works |

V adresáři instance je `compose.yaml` (vzor
[docker/compose.prod.yaml](../docker/compose.prod.yaml), resp.
[compose.demo.yaml](../docker/compose.demo.yaml)), `.env` a `data/`
(databáze, obálky, obrázky, import). Compose, `.env` ani nginx se při nasazení
**nekopírují** – po změně vzoru v repozitáři je upravte na serveru ručně.
Novou verzi nasadí z vývojového stroje:

```bash
just docker-deploy demo    # nejdřív demo
just docker-deploy prod    # pak produkce (zeptá se na potvrzení)
```

Recept sestaví image, přenese ho na server (`MEDIATER_DOCKER_HOST`, výchozí
172.24.0.46), spustí `docker compose up -d` a počká na healthcheck. Podrobně
v [README – Nasazení v Dockeru](../README.md#5-nasazení-v-dockeru).

### `.env` instance

Kromě cest a hostů (`LIBRITER_HOST`, `SCENITER_HOST`, `VIDEO_ROOT`, …) musí
`.env` obsahovat:

- **`SERVER_PORT`** – `8081` (produkce), `8082` (demo); musí sedět s `ports`
  v compose.
- **`JWT_SECRET`** – aspoň 32 bajtů (`openssl rand -hex 32`). Image běží se
  `SERVER_ENV=production`, takže s krátkou nebo vzorovou hodnotou
  (`change-me-before-production`) kontejner nenastartuje.
- **`TRUSTED_PROXIES`** – adresa, ze které kontejner vidí spojení od nginxu:
  vnitřní IP nginx stroje (na něm `ip route get 172.24.0.46`, údaj `src`).
  Bez ní server bere za IP klienta adresu nginxu a limity přihlášení sdílejí
  všichni uživatelé – jeden útočník by zablokoval přihlášení všem.

### nginx

Konfigurace je v [deploy/nginx/](../deploy/nginx/):

- `libriter-upstream.conf` → `/etc/nginx/conf.d/` – upstream na
  `172.24.0.46:8081`, formát logu bez query stringu (`mediater_noquery`)
  a zóna limitu pro `/api/v1/auth/`; vše sdílí oba virtuální servery.
- `audiobooks.helpdesk24.works.conf`, `video.helpdesk24.works.conf` →
  `/etc/nginx/sites-available/` (symlink do `sites-enabled/`).

```bash
sudo cp deploy/nginx/libriter-upstream.conf /etc/nginx/conf.d/
sudo cp deploy/nginx/audiobooks.helpdesk24.works.conf deploy/nginx/video.helpdesk24.works.conf /etc/nginx/sites-available/
sudo nginx -t && sudo systemctl reload nginx
```

Co konfigurace dělá:

- **Logy bez tokenů.** Přehrávání nese token v adrese (`?t=`); formát
  `mediater_noquery` zapisuje jen cestu (`$uri`), takže token v logu nezůstane
  na žádné cestě ani doméně – i když přes doménu Sceniteru přijde požadavek
  na audio Libriteru.
- **Limit `/api/v1/auth/`** 10 požadavků za minutu na IP (burst 20), mimo
  `stream-token`, `config` a `device/token`, které klienti volají často.
  Backend má navíc vlastní limity na IP a přihlašovací jméno.
- **Hlavičky k backendu:** `X-Real-IP`, `X-Forwarded-*`; `True-Client-IP` se
  maže, aby ji klient nepodvrhl.
- **HSTS** posílá nginx (`max-age` 2 roky, `includeSubDomains`), ostatní
  bezpečnostní hlavičky (CSP, `X-Frame-Options`, …) backend. `add_header`
  uvnitř `location` ruší hlavičky ze `server` bloku – žádná location ho
  proto nemá.

Soubory jsou záměrně jen HTTP; blok `:443` a přesměrování doplní
`sudo certbot --nginx -d <doména>`. Certbot zapisuje do souboru
v `sites-available/` – při kopírování nové verze z repozitáře se jeho blok
`:443` přepíše, proto po kopírování spusťte certbot pro dotčené domény znovu
(nebo blok přeneste ručně). Když `nginx -t` hlásí neznámý upstream, chybí
`libriter-upstream.conf` v `conf.d/`; když duplicitu, je upstream definovaný
ještě v jiném souboru.

### Ověření po nasazení

```bash
curl -sI https://video.helpdesk24.works/ | grep -iE 'strict-transport|content-security|x-frame'
ssh 172.24.0.46 'ss -ltnp | grep -E ":808[12]"'      # jen 127.0.0.1 a 172.24.0.46 na :8081 / :8082
ssh 172.24.0.46 'ls -l /opt/libriter/data/libriter.db*'  # -rw------- (0600)
sudo grep -c '?t=' /var/log/nginx/video.access.log /var/log/nginx/audiobooks.access.log  # na nginx stroji – po nasazení nepřibývá
```

Dál přihlášení stejným účtem do Libriteru i Sceniteru, přehrání filmu
a kapitoly, mobilní aplikace (přihlášení, přehrání, synchronizace pozice)
a spárování televize.

### Dopady aktualizace se zabezpečením

- **Registrace** se na instancích, kde ji admin nikdy neuložil, po
  aktualizaci zavře (nově výchozí stav). Na demu ji případně zapněte
  v **Administrace → Registrace**.
- **`JWT_SECRET`** zkontrolujte v `/opt/libriter/.env` i
  `/opt/libriter-demo/.env` ještě před nasazením (viz výš).
- **`TRUSTED_PROXIES`** nastavte a po nasazení ověřte, že se limit přihlášení
  počítá na klienta (několik chybných přihlášení z jedné IP nesmí zablokovat
  jinou).
- **Staré tokeny** (vydané před verzí s relacemi zařízení) zůstávají platné
  a nejdou odvolat jednotlivě; zneplatní je změna hesla nebo **Odhlásit
  všude** v profilu. U admin účtů to po nasazení doporučujeme udělat.
- **Práva souborů:** databáze se při startu sama nastaví na `0600`; importy
  nově vznikají `0644`, takže ostatní klienti NAS do nahraných knih nemohou
  zapisovat (číst ano).
- **Mobilní a TV aplikace** fungují dál bez aktualizace; nové sestavení je
  potřeba jen kvůli novému zacházení s adresou serveru (viz výš).

## Omezení

- Obrázkové titulky (PGS, VobSub) se nezobrazí.
- Plný převod je softwarový (libx264, `veryfast`); na serveru bez GPU zvládne
  jen několik současných převodů – proto `TRANSCODE_MAX_SESSIONS`.
- Při přebalení (MKV → HLS) se seká jen na klíčových snímcích, segmenty proto
  nemají přesně 6 s; prohlížeče to zvládají.
- Přihlášení je na každé doméně zvlášť (vlastní úložiště prohlížeče), účet je
  ale stejný.
- Uživatele, registraci a audit spravuje administrace kteréhokoli z obou webů
  (účty jsou společné).
- TV aplikace (Apple TV, Google TV) jsou v `sceniter-tv/` (viz výše).
