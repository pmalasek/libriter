# Pluginy metadat

Zdroje metadat (knihy v Libriteru, filmy a seriály ve Sceniteru) jsou samostatné
programy. Server je spouští pro každý dotaz znovu a mluví s nimi přes stdin a
stdout. Plugin tak může být v libovolném jazyce, jde ho vyměnit bez nového
sestavení serveru a jeho pád server neshodí.

## Instalace

Plugin je spustitelný soubor `mediater-meta-<jméno>` v adresáři pluginů:

| Prostředí | Adresář |
|---|---|
| balíček `.deb` | `/usr/lib/mediater/plugins` (`PLUGIN_DIR` v `/etc/libriter/libriter.env`) |
| `just build` | `bin/plugins` vedle `bin/mediater` |
| `just dev-backend` | `bin/plugins` (`PLUGIN_DIR=../bin/plugins`) |

Server soubory projde při startu a znovu po kliknutí na *Načíst pluginy znovu*
v administraci (Libriter: Zdroje metadat, Sceniter: Administrace → Zdroje
metadat). Plugin se nenačte, když:

- není spustitelný, nebo do něj smí zapisovat skupina či ostatní (`chmod 755`);
  jinak by ho mohl kdokoli vyměnit za vlastní program, který spustí server;
- neodpoví na `describe` do 10 s, nebo má neplatný manifest;
- `name` v manifestu neodpovídá názvu souboru.

Adresář jde nastavit jen v prostředí serveru, ne v administraci.

## Volání

```
<soubor> <příkaz>        stdin: jeden JSON objekt      stdout: jeden JSON objekt
```

- Exit 0 = úspěch, stdout je odpověď příkazu.
- Chyba: nenulový exit a na stdout `{"error": {"code": "…", "message": "…"}}`.
  Kódy: `not_found`, `rate_limited`, `unavailable`, `bad_request`.
- Stderr jde do logu serveru (úroveň debug); poslední řádek je zpráva chyby,
  když plugin spadne bez JSON odpovědi.
- Strop jednoho volání je 60 s, odpovědi 4 MB.
- Prostředí pluginu je oříznuté na `PATH`, `HOME`, `TMPDIR`, `LANG`, `LC_ALL`,
  `TZ`, proxy (`HTTP(S)_PROXY`, `NO_PROXY`) a `SSL_CERT_*`. Tajemství serveru
  (`JWT_SECRET`) plugin nevidí.

### Odstup mezi voláními

Proces nemá stav mezi voláními, takže odstup mezi požadavky na cizí web drží
server: jeden plugin běží nejvýš jednou najednou a mezi koncem jednoho a
začátkem dalšího volání čeká `min_interval_ms`. Plugin, který během jednoho
volání stahuje víc stránek, si odstup mezi nimi drží sám.

Když zdroj začne odmítat požadavky (HTTP 429, stránka ochrany proti botům),
plugin vrátí kód `rate_limited`. Server ho pak pozastaví – nejdřív na minutu,
s každým dalším odmítnutím na dvojnásobek, nejvýš na 30 minut; úspěšné
volání pauzu vynuluje. Během pauzy volání pluginu i stahování jeho obrázků
skončí hned chybou `rate_limited` a automatické párování pokračuje až po ní.
Odmítnutí obrázku (HTTP 429 nebo 403) pozastaví plugin stejně.

### Obrázky

Plugin vrací jen adresy obrázků (`image_url`, `poster_url`, `still_url`, …).
Stahuje je server, a jen z hostitelů v `image_hosts` – plugin na disk nepíše.

## Příkazy

### `describe`

Bez vstupu (stdin je `{}`), nesmí chodit na síť. Vrací manifest:

```json
{
  "protocol": 1,
  "name": "csfd",
  "title": "ČSFD (csfd.cz)",
  "version": "1.0.0",
  "kinds": ["movie", "show"],
  "languages": ["cs", "sk"],
  "config": [
    {"key": "api_key", "label": "API key", "secret": true, "required": true, "env": "TMDB_API_KEY"}
  ],
  "url_patterns": {
    "movie": ["(?i)^https?://(www\\.)?csfd\\.(cz|sk)/film/\\d+"],
    "show":  ["(?i)^https?://(www\\.)?csfd\\.(cz|sk)/film/\\d+"]
  },
  "image_hosts": ["image.pmgstatic.com"],
  "min_interval_ms": 2000
}
```

| Pole | Význam |
|---|---|
| `protocol` | Verze protokolu, teď `1`. |
| `name` | `[a-z0-9_]`, max. 40 znaků; shodné s názvem souboru za `mediater-meta-`. |
| `kinds` | `book`, `author` (Libriter), `movie`, `show` (Sceniter). |
| `languages` | Jazyky (ISO 639-1), pro které má zdroj smysl; `*` = všechny. Jen nápověda v administraci. |
| `config` | Nastavení, které admin vyplní v administraci. `secret` se nikdy nevrací zpět do prohlížeče. `env` je proměnná prostředí serveru s výchozí hodnotou. |
| `url_patterns` | Pro každý druh regulární výrazy (RE2) adres detailu, které plugin umí. Allowlist pro `fetch` podle URL – cizí adresu server pluginu nepošle. |
| `image_hosts` | Hostitelé obrázků (platí i s `www.`). |
| `min_interval_ms` | Odstup mezi voláními, viz výš. |

### `search`

```json
{"kind": "movie", "language": "cs",
 "query": {"title": "Pelíšky", "original_title": "", "author": "", "year": 1999},
 "config": {"api_key": "…"}}
```

`author` vyplňuje Libriter u knih, `year` a `original_title` Sceniter (nemusí
být známé). U `kind: "author"` je hledané jméno v `title`.

```json
{"results": [
  {"id": "4570", "url": "https://www.csfd.cz/film/4570-pelisky/prehled/",
   "title": "Pelíšky", "original_title": "", "year": 1999,
   "author": "", "note": "Česko, Komedie / Drama", "image_url": "https://…"}
]}
```

`id` je identifikátor u zdroje (řetězec). Výsledky řaďte od nejlepšího –
Sceniter páruje automaticky jen shodu názvu a roku (±1), nebo první výsledek
se shodným rokem.

### `fetch`

```json
{"kind": "movie", "id": "4570", "language": "cs", "config": {}}
{"kind": "book", "url": "https://www.databazeknih.cz/prehled-knihy/…", "language": "cs"}
```

Přijde `id`, nebo `url` (jen adresa z `url_patterns`). Odpověď je
`{"item": {…}}`; tvar položky podle druhu:

- `book` – `BookMetadata` (`server/internal/libriter/metadata/metadata.go`):
  `title`, `author`, `description`, `genres`, `cover_url`, `rating` (0–100),
  `publisher`, `year`, `original_title`, `series`, `series_position`,
  `narrator`, `language`, `source_url`;
- `author` – `AuthorMetadata`: `name`, `bio`, `pseudonyms`, `image_url`,
  `birth_year`, `death_year`, `source_url`;
- `movie`, `show` – `VideoMetadata` (`server/internal/core/plugin/video.go`):

```json
{"item": {
  "id": "9499", "url": "https://www.csfd.cz/film/9499/prehled/",
  "title": "Matrix", "original_title": "The Matrix", "year": 1999,
  "overview": "…", "tagline": "", "status": "", "runtime_min": 136,
  "rating": 90, "genres": ["Akční", "Sci-Fi"],
  "poster_url": "https://…", "backdrop_url": "",
  "external_ids": {"csfd": "9499", "imdb": "tt0133093"},
  "credits": {
    "directors": [{"id": "3112", "name": "Lana Wachowski"}],
    "writers": [], "creators": [],
    "cast": [{"id": "6384", "name": "Keanu Reeves", "character": "Neo",
              "image_url": "https://…/keanu.jpg"}]
  },
  "seasons": []
}}
```

`rating` je v procentech. `external_ids` s klíči `imdb`, `tmdb`, `csfd`,
`tvmaze`, `thetvdb` web zobrazí jako odkazy. `seasons` jsou čísla řad seriálu.

Osoby v `credits` mají pořadí jako ve zdroji (hlavní role první):

| Pole | Význam |
|---|---|
| `id` | ID osoby u zdroje. Podle něj server pozná stejného herce v různých titulech; bez ID páruje jen podle jména. |
| `name` | Jméno (povinné). |
| `character` | Postava (jen `cast`). |
| `image_url` | Fotka osoby; server ji stáhne jen z `image_hosts`, a jen pokud ji osoba ještě nemá. |

Místo objektu jde napsat i samotné jméno (`"directors": ["Lana Wachowski"]`)
– starší tvar, osoba pak nemá ID ani fotku.

### `season`

Jen pro `kind: "show"`:

```json
{"kind": "show", "id": "1024280", "season": 1, "episodes": [1, 2, 5],
 "language": "cs", "config": {}}
```

`episodes` jsou čísla dílů, které server má na disku. Plugin, který stahuje
každý díl zvlášť (scraping), smí ostatním dílům vrátit jen číslo a název;
prázdné nebo chybějící `episodes` = všechny díly.

```json
{"season": {"number": 1, "name": "Série 1", "overview": "…", "poster_url": "…",
  "episodes": [{"number": 1, "title": "Lesní vražda", "overview": "",
                "air_date": "2023-01-09", "still_url": "", "runtime_min": 0}]}}
```

Server uloží metadata jen k dílům, které jsou na disku.

## Příklad v Pythonu

```python
#!/usr/bin/env python3
# mediater-meta-ukazka – plugin, který zná jediný film.
import json, sys

MANIFEST = {
    "protocol": 1, "name": "ukazka", "title": "Ukázka", "version": "1.0",
    "kinds": ["movie"], "languages": ["*"],
    "url_patterns": {"movie": [r"^https://example\.com/film/\d+$"]},
    "image_hosts": ["example.com"],
}

def reply(obj, code=0):
    json.dump(obj, sys.stdout)
    sys.exit(code)

cmd = sys.argv[1] if len(sys.argv) > 1 else ""
req = json.load(sys.stdin)
if cmd == "describe":
    reply(MANIFEST)
if cmd == "search":
    if "pelíšky" in req["query"]["title"].lower():
        reply({"results": [{"id": "1", "title": "Pelíšky", "year": 1999}]})
    reply({"results": []})
if cmd == "fetch" and req.get("id") == "1":
    reply({"item": {"id": "1", "title": "Pelíšky", "year": 1999, "rating": 91,
                    "genres": ["Komedie"], "credits": {}}})
reply({"error": {"code": "not_found", "message": "neznámý film"}}, 1)
```

```bash
install -m 755 mediater-meta-ukazka /usr/lib/mediater/plugins/
echo '{"kind":"movie","query":{"title":"Pelíšky"}}' | /usr/lib/mediater/plugins/mediater-meta-ukazka search
```

Pak v administraci *Načíst pluginy znovu* a u knihovny zvolit zdroj „Ukázka“.

## Pluginy v repozitáři

Pluginy dodávané s balíčkem jsou v Go (`server/cmd/plugins/<jméno>`) a používají
SDK `server/internal/core/plugin/sdk`, které obstará příkazy, JSON a chybové
kódy. Knižní pluginy jen obalují vestavěné scrapery (`bookplugin.Run`).

| Plugin | Druhy | Zdroj |
|---|---|---|
| `databazeknih`, `cbdb` | knihy, autoři | databazeknih.cz, cbdb.cz (scraping) |
| `openlibrary` | knihy, autoři | Open Library API |
| `googlebooks` | knihy | Google Books API (klíč nepovinný, `GOOGLE_BOOKS_API_KEY`) |
| `audible_com`, `audible_de` | knihy | Audible (audioknihy, vypravěč) |
| `goodreads` | knihy | goodreads.com (scraping) |
| `csfd` | filmy, seriály | csfd.cz (scraping) |
| `tmdb` | filmy, seriály | The Movie Database API (klíč `TMDB_API_KEY`) |
| `tvmaze` | seriály | TVmaze API |

Hostitel pluginů je `server/internal/core/plugin` (spouštění, allowlisty,
odstup, nastavení v tabulce `settings` pod klíčem `plugins.<jméno>`).
