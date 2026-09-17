# Libriter

Osobní správce a přehrávač audioknih s webovým rozhraním a mobilní aplikací.

## Architektura

```
libriter/
├── Makefile            # build frontendu i backendu
├── libriter-backend/   # REST API + server webového rozhraní (Go)
│   └── internal/web/dist/   # sem se sestaví frontend, vkompiluje se do binárky
├── libriter-frontend/  # Webové rozhraní (React + Vite + Tailwind)
├── libriter-mobile/    # Mobilní aplikace
├── _scripts/           # Pomocné skripty
├── bin/                # Sestavená binárka (just build)
└── data/
    ├── libriter.db     # SQLite databáze (DB_PATH) – vytvoří se automaticky
    ├── audio/          # Audio soubory (AUDIO_ROOT)
    ├── covers/         # Obálky knih (COVER_ROOT)
    └── author-images/  # Fotky autorů (AUTHOR_IMAGE_ROOT)
```

Produkční build je **jediná binárka** – webové rozhraní je v ní vestavěné přes
`go:embed` a servíruje se ze stejného portu jako API.

Databázové schéma je v `libriter-backend/internal/db/migrations/` a aplikuje se
automaticky při startu backendu.

---

## Požadavky

### Systémové závislosti

| Nástroj | Verze | Účel | Instalace |
|---------|-------|------|-----------|
| **Go** | ≥ 1.22 | Backend | viz níže |
| **Node.js** | ≥ 22 (testováno 24) | Build frontendu | viz níže |
| **ffprobe** | libovolná | Délka audio souborů (scanner) | součást balíčku `ffmpeg` |

Node.js je potřeba jen pro **sestavení** frontendu. Hotová binárka už na něm
nezávisí – webové rozhraní je v ní vestavěné.

Databáze je **SQLite** vestavěná přímo v backendu (čistě Go driver `modernc.org/sqlite`,
bez cgo) – žádný databázový server není potřeba. Soubor `data/libriter.db` i schéma
vzniknou automaticky při prvním startu.

#### Go

```bash
# Linux – stáhnout z https://go.dev/dl/ nebo přes správce balíčků
sudo apt install golang-go          # Debian/Ubuntu (může být starší verze)

# Doporučená cesta – instalace konkrétní verze:
wget https://go.dev/dl/go1.24.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin  # přidat do ~/.bashrc nebo ~/.zshrc
```

#### Node.js

```bash
# Doporučená cesta – nvm (umožňuje více verzí vedle sebe)
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash
nvm install 24

# Ověření
node --version   # v24.x
npm --version
```

#### Developer tools

```bash
sudo apt install build-essential git cmake pkg-config
```

#### just (task runner)

Build a vývojové cíle se spouštějí přes [`just`](https://github.com/casey/just).
```bash
sudo apt install just     # nebo: cargo install just
```
`just` bez argumentů (nebo `just --list`) vypíše dostupné cíle.

#### ffprobe (ffmpeg)

Potřebný pro správné zjišťování délky audio souborů v scanneru.
Bez něj se délka uloží jako `1 s` (placeholder) a lze ji opravit přes API.

```bash
# Debian/Ubuntu
sudo apt install ffmpeg

# Arch Linux
sudo pacman -S ffmpeg

# Ověření
ffprobe -version
```

---

## Konfigurace backendu

Zkopírujte `env.example` do `.env` a upravte hodnoty:

```bash
cp libriter-backend/env.example libriter-backend/.env
```

```dotenv
# Server
SERVER_PORT=8080
SERVER_ENV=development          # development | production

# Databáze (SQLite – soubor i schéma se vytvoří automaticky při startu)
DB_PATH=./data/libriter.db

# JWT – NUTNÉ ZMĚNIT před nasazením do produkce!
JWT_SECRET=change-me-before-production
JWT_EXPIRY_HOURS=72

# Soubory
AUDIO_ROOT=./data/audio         # kořenový adresář audio souborů
COVER_ROOT=./data/covers        # kořenový adresář obálek knih
AUTHOR_IMAGE_ROOT=./data/author-images   # fotky autorů
MAX_UPLOAD_MB=500

# Zdroje metadat – výchozí pořadí, ve kterém se zkoušejí
METADATA_PROVIDERS=databazeknih,cbdb,openlibrary,googlebooks
GOOGLE_BOOKS_API_KEY=           # nepovinné, viz Metadata knih
```

> **Zdroje metadat** jsou tady jen výchozí hodnota pro prázdnou databázi.
> Jakmile je admin uloží v administraci, platí nastavení z databáze a změny
> v `.env` se už neprojeví (viz Administrace).

> **Bezpečnost:** `JWT_SECRET` musí být v produkci silný náhodný řetězec.
> Vygenerujte ho např. pomocí: `openssl rand -hex 32`

Webové rozhraní ani CLI žádné další proměnné nepotřebují – `.env` zůstává stejný.
Vývojový server Vite přebírá adresu backendu z `BACKEND_URL` (výchozí
`http://localhost:8080`), což je proměnná prostředí, ne součást `.env`.

**Jak se `.env` hledá:** v aktuálním adresáři, pak v jeho podadresáři
`libriter-backend/`, a takto dál v nadřazených adresářích. Cestu lze vynutit
proměnnou `LIBRITER_ENV_FILE`. Skutečné proměnné prostředí mají vždy přednost
před hodnotami z `.env`.

**Relativní cesty** (`DB_PATH`, `AUDIO_ROOT`, `COVER_ROOT`, `AUTHOR_IMAGE_ROOT`) zapsané v `.env` se
vztahují k adresáři toho `.env` – ne k aktuálnímu adresáři. `bin/libriter` tak
míří na stejná data, ať ho spustíte odkudkoli. Cesta předaná proměnnou prostředí
se ponechává tak, jak je.

---

## Instalace a spuštění

### 1. Vytvoření adresářů pro data

```bash
mkdir -p data/audio data/covers data/author-images
```

### 2. Produkční build – jedna binárka

```bash
just build          # npm ci + npm run build, potom go build
bin/libriter
```

Vznikne `bin/libriter` s vestavěným webovým rozhraním. Rozhraní pak najdete na
`http://localhost:8080`, API na `http://localhost:8080/api/v1`.

Binárku lze spustit z libovolného adresáře: `.env` se hledá v aktuálním adresáři
a v nadřazených (včetně podadresáře `libriter-backend/`) a relativní cesty v něm
(`DB_PATH`, `COVER_ROOT`, …) se vztahují k adresáři toho `.env`. Server na startu
vypíše, kterou databázi otevřel. Jiný soubor vynutíte přes `LIBRITER_ENV_FILE`.

### 3. Vývoj – dva procesy

| Terminál | Příkaz | Co běží |
|----------|--------|---------|
| 1 | `just dev-backend` | Go API na `http://localhost:8080` |
| 2 | `just dev-frontend` | Vite s hot reloadem na `http://localhost:5173` |

Pracujte na `http://localhost:5173`. Vite proxuje `/api` a `/health` na backend,
takže CORS není potřeba a frontend volá stejné relativní cesty jako v produkci.
Běží-li backend na jiném portu: `BACKEND_URL=http://localhost:9000 npm run dev`.

Oba servery naslouchají na všech rozhraních (`0.0.0.0`), takže když běží na jiném
stroji než prohlížeč, otevřete adresu, kterou Vite vypíše na řádku **Network**
(např. `http://172.24.0.46:5173`). Stejně tak produkční binárka je dostupná na
`http://<ip-serveru>:8080`.

První spuštění frontendu si vyžádá závislosti:

```bash
cd libriter-frontend && npm install
```

### Přehled cílů (justfile)

| Cíl | Popis |
|-----|-------|
| `just build` | Frontend i backend → `bin/libriter` |
| `just frontend` | `npm ci && npm run build` (výstup do `libriter-backend/internal/web/dist`) |
| `just backend` | `go build` s aktuálně sestaveným frontendem |
| `just dev-backend` | `go run ./cmd/server` |
| `just dev-frontend` | `npm run dev` |
| `just vet` | `go vet ./...` |
| `just clean` | Smaže `bin/` a sestavený frontend |

Backend jde sestavit i bez frontendu (`just backend` na čerstvém klonu) – server
pak na `/` vrátí stránku s návodem a HTTP 503, ale API funguje normálně.

> **Pozor:** backend spouštějte jako balíček (`go run ./cmd/server`), ne jako
> jediný soubor (`go run cmd/server/main.go`). Balíček `main` je rozdělený do
> `main.go`, `serve.go` a `user_cmd.go`, takže build jediného souboru selže.

Při prvním startu backend automaticky:
- vytvoří soubor databáze na cestě `DB_PATH` (včetně adresáře)
- aplikuje migrace z `internal/db/migrations/` – všechny tabulky
  (authors, series, books, book_authors, chapters, tags, users, roles,
  permissions, ratings, ...)
- založí výchozí role **admin**, **editor**, **reader** a jejich oprávnění

Aplikované migrace se evidují v tabulce `schema_migrations`; při dalších
startech se spustí jen nové. Zálohu databáze pořídíte prostým zkopírováním
souboru `libriter.db` (při běžícím serveru i souborů `-wal` a `-shm`).

---

## Webové rozhraní

React + TypeScript + Tailwind v4, komponenty shadcn/ui (Radix), routing
`react-router`, serverový stav `@tanstack/react-query`. Vše v češtině.

**Vzhled:** hlavní barvou je tyrkysová z loga, oranžová slouží jako doplněk
pro zvýraznění (počty dílů, nedodělky v administraci). Nadpisy sází
Bricolage Grotesque, běžný text Geist; obojí je součástí balíčku, nic se
nenačítá z cizích serverů. Barvy, zaoblení a fonty jsou pohromadě
v `libriter-frontend/src/index.css` – změna tokenů přebarví celou aplikaci.

**Co první verze umí:** přihlášení a registraci, seznam a detail knih, autory,
série, profil (změna jména, e-mailu a hesla), hledání v knihách, obálky knih,
světlý i tmavý režim podle systému.

**Seznamy knih a autorů** mají tři zobrazení (dlaždice, malé dlaždice, seznam)
a volitelné řazení – knihy podle názvu, autora (příjmení, křestní, prostřední
jméno), roku prvního vydání nebo data přidání; autoři podle příjmení (výchozí, jméno
se pak ukazuje katalogově „Čapek, Karel“), křestního jména nebo počtu knih.
Zvolené zobrazení i řazení si prohlížeč pamatuje (`localStorage`).
Autoři ani série bez jediné knihy se v seznamech neukazují – zůstávají
v databázi a objeví se, jakmile k nim nějaká kniha patří.

**Editace (role editor a vyšší):** na detailu knihy i autora je tlačítko
*Upravit*, které otevře formulář v dialogu. U knihy jde změnit název, autory
(včetně pořadí – první je hlavní), sérii a díl, vypravěče, délku, jazyk, rok
vydání, vlastní hodnocení a popis; tlačítko *Načíst metadata* vyhledá knihu ve
zdrojích (databazeknih.cz, cbdb.cz, OpenLibrary, Google Books – viz Metadata
knih) a předvyplní název, popis a rok prvního vydání (u překladů rok
originálu). Ukládá se přes
`PATCH /books/{id}`, takže odchází jen skutečně změněná pole. Tlačítko *Uložit
a další* (Ctrl+Enter) uloží a rovnou otevře editaci následující knihy v pořadí
seznamu – hodí se při procházení celé knihovny.

**Kapitoly:** detail knihy má sbalenou kartu *Kapitoly* se seznamem audio
souborů, jejich názvy a délkami. Editor ji může přepnout tlačítkem *Změnit
pořadí* do režimu přeskládání: řádky jdou přetahovat myší i prstem, posouvat
šipkami, nebo hromadně seřadit podle názvu souboru či názvu kapitoly. Hodí se
u souborů bez track tagů, kde pořadí odvozené z názvů nesedí. Uložené ruční
pořadí zůstává zachované i po opravě kapitol v administraci; soubory přidané
později se zařadí na konec.

**Hromadné zařazení do série:** v seznamu knih zapne tlačítko *Vybrat* režim
výběru; kliknutí na knihu ji označí. *Přidat do série* pak vybrané knihy
zařadí do existující nebo nové série a nechá upravit čísla dílů (předvyplní se
za poslední obsazený díl). Série může spojovat knihy různých autorů.

U autora se edituje jméno po částech, roky života a životopis
(`PUT /authors/{id}`). Stejné tlačítko *Načíst metadata* vyhledá autora ve
zdrojích a předvyplní životopis i roky a **přepíše jméno** (křestní, prostřední
i příjmení) tím ze zdroje – slouží k opravě jmen zkomolených v audio tazích.
Nabídnutá fotka se stáhne až při uložení (`PUT /authors/{id}/image`), takže
„Zrušit“ nic nezmění. Fotka se ukazuje na detailu autora i na kartách
v seznamu; autor bez fotky má zástupnou ikonu. Čtenář (role reader) tlačítka
nevidí.

**Poslech:** *Přehrát* v detailu knihy, *Přehrát sérii* u série a *Poslouchat
výběr* v režimu výběru knih otevřou poslech v liště u spodního okraje. Lišta je
vidět na všech stránkách, takže poslech nepřeruší procházení knihovny.

Poslech (session) je kniha, celá série, nebo ručně poskládaný seznam. Aktuální
kapitola i pozice v ní se drží na serveru, ne v prohlížeči – na jiném zařízení
tedy poslech pokračuje tam, kde skončil. Zapisuje se každých 10 sekund a při
každé změně (pauza, převíjení, změna kapitoly i zavření stránky).

V liště je hlasitost (na širokých obrazovkách; telefon a tablet mají vlastní
tlačítka a iOS hlasitost přes `<audio>` nastavit nedovolí) a panel *Obsah
poslechu* se všemi knihami poslechu i jejich soubory. Kapitoly se v něm
stahují až při otevření a rozbalená je ta kniha, která hraje – kliknutím na
kterýkoliv soubor se přejde přímo na něj.

Rozposlouchaných poslechů může být víc naráz. Přepíná se mezi nimi ikonou
sluchátek v liště a celý přehled je na stránce *Právě posloucháno* – ta se
v navigaci objeví jako první položka, jakmile je co poslouchat, a je to
i první pohled po otevření aplikace (kořenová adresa jinak vede do knihovny).
Dá se z ní pokračovat i uklidit doposlechnuté.

Každá kniha si navíc drží stav: *rozposlouchaná* se objeví po prvním poslechu,
*doposlechnutá* po dohrání poslední kapitoly. Značka je vidět na dlaždici i
v seznamu knih a v detailu knihy jde stav ručně přepnout – pro knihu slyšenou
jinde nebo omylem dohranou do konce. Stav i odposlouchaný čas zůstávají,
i když se poslech smaže; administrátor je vidí u každého účtu. *Přehrát* u knihy, která už
v nějakém poslechu je, pokračuje v něm místo zakládání nového. Tlačítko se
u právě hrané knihy mění na *Pozastavit*. Po doposlechnutí kapitoly navazuje
další, po poslední kapitole další kniha poslechu.

**Co ještě ne:** zakládání či
mazání knih, autorů a sérií z rozhraní – ty zakládá scanner nebo přímé volání
API. Obálku a cestu k audio souborům nelze z rozhraní měnit, spravuje je
scanner.

Přihlášený uživatel se drží v `localStorage` (JWT + profil). Profil i role se
při otevření rozhraní srovnají se serverem, takže změna role se projeví bez
nového přihlášení.

Barevné schéma (tyrkysová, modrá, fialová nebo zelená) a režim zobrazení
(`light`, `dark`, `system`) se ukládají automaticky do profilu v databázi.
Po přihlášení na jiném zařízení má profil přednost před místním nastavením.
Vzhled lze změnit pod ikonou palety i v sekci **Profil → Vzhled**.
Nepřihlášeným uživatelům se volba ukládá pouze v prohlížeči.

### Administrace (role admin)

Položka **Administrace** v navigaci vede na `/admin` a vidí ji jen
administrátor. Má sedm záložek:

| Záložka | Co umí |
|---------|--------|
| **Přehled** | Počty knih, autorů, sérií, uživatelů a kapitol, celková délka, kolik knih nemá obálku či popis a kolik kapitol má placeholder délku 1 s. Vedle toho verze serveru, prostředí, doba běhu, cesty k datům, dostupnost `ffprobe` a volné místo na disku s audiem. |
| **Uživatelé** | Seznam účtů, změna role přímo v řádku, reset hesla, smazání a založení nového účtu s libovolnou rolí. |
| **Poslechy** | U každého účtu rozposlouchané a doposlechnuté poslechy, které knihy už slyšel a deník poslechu – kolik času u které knihy za den odposlouchal. Deník i stav knih přežijí smazání poslechu. |
| **Zdroje metadat** | Zapnutí a vypnutí jednotlivých zdrojů, změna pořadí šipkami a klíč pro Google Books. Uložení platí okamžitě, server se nerestartuje. |
| **Knihovna** | Stav scanneru (běží / poslední průchod / počet souborů / chyby), ruční spuštění kontroly knihovny a oprava kapitol s náhledem před provedením. |
| **Registrace** | Přepínač veřejné registrace a role, kterou nový účet dostane. |
| **Audit** | Výpis administrativních zásahů – kdo, kdy, co a s jakými detaily. |

Vlastní účet si admin nemůže smazat ani si sám snížit roli a poslední
administrátor v systému nejde smazat ani degradovat; server takový pokus
odmítne (`400`, resp. `409`) bez ohledu na to, jestli přijde z rozhraní, nebo
z CLI.

---

## Správa uživatelů z příkazové řádky

Registrace přes API dává roli podle nastavení (výchozí **reader**), takže
prvního administrátora vytvořte přes CLI stejné binárky. Další účty už jde
zakládat i v administraci webového rozhraní.

```bash
# nové konto (bez --password se heslo zadá interaktivně, skrytě a dvakrát)
bin/libriter user add --email admin@example.com --name "Jan Novák" --role admin

# povýšení už registrovaného účtu
bin/libriter user set-role --email jan@example.com --role editor

# přehled účtů
bin/libriter user list
```

Ve vývoji bez buildu: `go run ./cmd/server user add --email … --name …`.

| Příkaz | Přepínače |
|--------|-----------|
| `user add` | `--email` a `--name` (povinné), `--role` (`admin`\|`editor`\|`reader`, výchozí `reader`), `--password` (min. 8 znaků) |
| `user set-role` | `--email`, `--role` |
| `user list` | – |

CLI čte stejný `.env` jako server a samo aplikuje chybějící migrace, takže
funguje i na prázdné databázi. Server ani scanner přitom nespouští a **nevyžaduje
`JWT_SECRET`** – žádné tokeny nepodepisuje.

Před dotazem na heslo vypíše, který `.env` a kterou databázi použil, takže je
hned vidět, kdyby mířil jinam, než chcete.

`user set-role` odmítne odebrat roli poslednímu administrátorovi – nejdřív
povyšte někoho dalšího.

---

## Audio scanner

Scanner se spustí **automaticky při startu backendu** a:

1. Při startu projde `AUDIO_ROOT` a přidá do databáze všechny audio soubory, které v ní ještě nejsou (totéž spustí admin kdykoliv znovu tlačítkem v administraci)
2. Sleduje `AUDIO_ROOT` (vč. podadresářů) a reaguje na nové soubory v reálném čase
3. Čeká, dokud se soubor nepřestane měnit (kopírování dokončeno) – kontrola každé 3 s, stabilita 10 s
4. Extrahuje metadata z audio tagů a vloží knihu do databáze

**Podporované formáty:** `.mp3`, `.m4a`, `.m4b`, `.ogg`, `.flac`, `.opus`, `.aac`, `.wav`

**Pořadí kapitol** (`chapters.position`) se určuje v tomto pořadí priority:

1. **Ruční pořadí** nastavené editorem v rozhraní – přežije i opravu kapitol
2. **Číslo disku a tracku z tagů** (`Disc`, `Track`); každý disk má vlastní
   rozsah tisícovek, takže si CD1 a CD2 nesahají na pozice
3. **Přirozené pořadí názvu souboru** mezi audio soubory adresáře – čísla se
   porovnávají jako čísla, takže `2.mp3` je před `10.mp3`
4. Jinak další v řadě

Obsazenou pozici scanner nikdy nepřepíše, jen posune na první volnou. Mezery
v číslování ničemu nevadí, kapitoly se řadí podle relativního pořadí.

**Priorita metadat z tagů:**

| Pole | Pořadí čtení |
|------|-------------|
| Název knihy | `Album` → `Title` tag → název souboru |
| Autoři | `AlbumArtist` → `Composer` → `Artist` → název adresáře |
| Vypravěč | `Artist` (pokud se liší od autorů) |
| Délka | `ffprobe` → `1 s` (placeholder, opravit přes API) |

**Autoři:** kniha jich může mít víc. Tag se rozdělí na jednotlivá jména podle
oddělovačů `;` `/` `&` `|`, slov „a“ / „and“ a podle čárky (ta se ale bere jako
oddělovač jen tehdy, nejde-li o tvar `Příjmení, Křestní`). Každé jméno se pak
rozdělí na **křestní / prostřední / příjmení**:

| Tag | Křestní | Prostřední | Příjmení |
|-----|---------|------------|----------|
| `Jan Amos Komenský` | Jan | Amos | Komenský |
| `Komenský, Jan Amos` | Jan | Amos | Komenský |
| `Karel Čapek` | Karel | – | Čapek |
| `Homér` | – | – | Homér |

Jednoslovné jméno se ukládá jako příjmení – podle něj se autoři řadí i hledají.
Autor je v databázi jednoznačně určen trojicí jmen, takže stejný autor ze dvou
různých souborů vznikne jen jednou. Pořadí autorů z tagu se zachovává, první je
hlavní autor (podle něj scanner páruje soubory ke knize).

**Obálky:** při ingestu knihy scanner hledá obálku v tomto pořadí:

1. **Obrázek v adresáři s audio soubory** (`.jpg`, `.jpeg`, `.png`, `.webp`, `.gif`, `.bmp`) –
   pokud jich je víc, použije se **největší** (podle velikosti souboru)
2. **Obrázek vložený v tagu** audio souboru (`APIC` / `covr`)

Nalezená obálka se zkopíruje do `COVER_ROOT` jako `<book_id>.<přípona>` a relativní
cesta se uloží do `books.cover_path`. Kniha, která už obálku má (a soubor v
`COVER_ROOT` existuje), se znovu nepřepisuje. Obrázky nad 20 MB se ignorují.

---

## API přehled

Základní URL: `http://localhost:8080/api/v1`

### Autentikace

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `GET` | `/auth/config` | Je registrace zapnutá a s jakou rolí | veřejné |
| `POST` | `/auth/register` | Registrace nového uživatele | veřejné |
| `POST` | `/auth/login` | Přihlášení, vrátí JWT token | veřejné |
| `GET` | `/auth/stream-token` | Krátkodobý token pro adresu audia (24 h) | přihlášený |

Při vypnuté registraci vrací `POST /auth/register` `403`; přepínač je
v administraci (viz níže).

Všechny ostatní endpointy vyžadují hlavičku:
```
Authorization: Bearer <token>
```

Výjimkou je `GET /books/{id}/cover` – obálky se načítají přes `<img>`, které
hlavičku `Authorization` poslat neumí. Ochranou je neuhodnutelné UUID knihy.

Druhou výjimkou je `GET /chapters/{id}/audio`: `<audio>` hlavičku poslat také
neumí, ale audio je proti obálce citlivější, takže se ověřuje tokenem v adrese.
Není to přihlašovací token – `GET /auth/stream-token` vydá samostatný token
platný 24 hodin, který **umí jen streamovat**. Opačně to platí taky: stream
token API nikam jinam nepustí.

### Uživatelé

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `GET` | `/users` | Seznam uživatelů | admin |
| `GET` | `/users/{id}` | Detail uživatele | admin / vlastní profil |
| `PUT` | `/users/{id}` | Aktualizace jména a emailu | admin / vlastní profil |
| `PUT` | `/users/{id}/password` | Změna hesla | admin / vlastní profil |
| `PUT` | `/users/{id}/appearance` | Uložení vzhledu (`color_scheme`, `theme_mode`) | vlastní profil |
| `PUT` | `/users/{id}/role` | Nastavení role | admin |
| `DELETE` | `/users/{id}` | Smazání uživatele | admin |

Vlastní účet vrátí na `DELETE` `400` a poslední administrátor `409` (platí
i pro snížení jeho role).

### Administrace

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `POST` | `/admin/users` | Založení účtu s libovolnou rolí | admin |
| `GET` | `/admin/settings/metadata` | Zdroje metadat, jejich pořadí a stav | admin |
| `PUT` | `/admin/settings/metadata` | Uložení zdrojů (platí okamžitě) | admin |
| `GET` | `/admin/settings/registration` | Nastavení registrace | admin |
| `PUT` | `/admin/settings/registration` | Uložení nastavení registrace | admin |
| `GET` | `/admin/scanner` | Stav scanneru | admin |
| `POST` | `/admin/scanner/rescan` | Spuštění průchodu knihovnou | admin |
| `GET` | `/admin/library/repair` | Náhled opravy kapitol (nic nemění) | admin |
| `POST` | `/admin/library/repair` | Provedení opravy kapitol | admin |
| `GET` | `/admin/library/merge` | Náhled sloučení rozdělených knih (nic nemění) | admin |
| `POST` | `/admin/library/merge` | Sloučení rozdělených knih | admin |
| `GET` | `/admin/stats` | Statistiky knihovny | admin |
| `GET` | `/admin/system` | Verze, cesty, ffprobe, místo na disku | admin |
| `GET` | `/admin/audit?limit=&before=` | Výpis administrativních akcí | admin |
| `GET` | `/admin/listening` | Přehled poslechu všech uživatelů | admin |
| `GET` | `/admin/listening/{id}` | Poslechy, stav knih a deník posledních 90 dní jednoho uživatele | admin |

Zápis zdrojů metadat nahrazuje celý seznam a jeho pořadí je pořadí, ve kterém
se zdroje zkoušejí:

```jsonc
// PUT /admin/settings/metadata
{ "providers": [ { "name": "databazeknih", "enabled": true },
                 { "name": "googlebooks",  "enabled": false } ],
  "google_books_api_key": "" }
```

Průchod knihovnou i oprava kapitol běží na pozadí; druhý souběžný požadavek
skončí `409`. Plán opravy si server vždy sestaví sám, klient mu seznam knih ke
smazání neposílá. Audit se stránkuje kurzorem: `next_before` z odpovědi se
pošle jako `before` v dalším požadavku.

### Knihy

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `GET` | `/books` | Seznam knih | reader+ |
| `GET` | `/books/{id}` | Detail knihy | reader+ |
| `GET` | `/books/{id}/cover` | Obrázek obálky (soubor z `COVER_ROOT`) | veřejné |
| `GET` | `/books/{id}/chapters` | Kapitoly (audio soubory) v pořadí přehrávání | reader+ |
| `GET` | `/books/progress` | Stav knih přihlášeného uživatele (rozposlouchané, doposlechnuté) | reader+ |
| `PUT` | `/books/{id}/progress` | Ruční označení knihy za doposlechnutou (`{"finished": true}`) i jeho zrušení | reader+ |
| `DELETE` | `/books/{id}/progress` | Návrat knihy mezi neposlechnuté | reader+ |
| `POST` | `/books` | Přidání knihy | editor+ |
| `PUT` | `/books/{id}` | Aktualizace knihy (úplná náhrada) | editor+ |
| `PATCH` | `/books/{id}` | Aktualizace jen poslaných polí | editor+ |
| `PUT` | `/books/{id}/chapters/order` | Ruční pořadí kapitol | editor+ |
| `DELETE` | `/books/{id}` | Smazání knihy | admin |

Kniha má autory ve vazbě M:N. Při zápisu se posílá `author_ids` (alespoň jedno
ID, pořadí určuje hlavního autora), ve čtení se vrací pole `authors` s celými
záznamy autorů:

```jsonc
// POST /books
{ "author_ids": ["<uuid>", "<uuid>"], "title": "Ze života hmyzu",
  "duration_seconds": 7200, "file_path": "capek/ze-zivota-hmyzu" }

// GET /books/{id}
{ "id": "<uuid>", "title": "Ze života hmyzu",
  "authors": [ { "id": "<uuid>", "first_name": "Karel", "middle_name": "",
                 "last_name": "Čapek", "name": "Karel Čapek" }, ... ] }
```

Kapitola je jeden audio soubor. Celá cesta k souboru se nevystavuje (adresář
knihy je v `file_path` knihy), klient dostane jen název souboru – podle něj se
pozná, jestli pořadí sedí. `start_offset_seconds` je začátek kapitoly v rámci
celé knihy, počítá se ze součtu délek předchozích kapitol.

```jsonc
// GET /books/{id}/chapters
[ { "id": "<uuid>", "position": 1, "title": "Kapitola 1", "file_name": "01.mp3",
    "start_offset_seconds": 0, "duration_seconds": 1834 }, ... ]

// PUT /books/{id}/chapters/order – všechny kapitoly knihy, každá právě jednou
{ "chapter_ids": ["<uuid>", "<uuid>", "<uuid>"] }
```

**`PUT` vs. `PATCH`:** `PUT` je úplná náhrada a vyžaduje i `file_path`. Ten se
ale přes API nikdy nevrací (cesty k audiu jsou v režii scanneru), takže klient,
který knihu jen četl, nemá co poslat a `PUT` by cestu přepsal. Na úpravy proto
slouží `PATCH`, který mění výhradně pole obsažená v těle:

```jsonc
// PATCH /books/{id} – vynechané pole zůstane beze změny, null sloupec vyprázdní
{ "title": "Nový název", "narrator": null, "published_year": 1936 }
```

`PATCH` pole `file_path` nepřijímá vůbec (skončí `400`, stejně jako každé jiné
neznámé pole). Ověřuje se jen to, co klient poslal: `title` nesmí být prázdný,
`duration_seconds` musí být kladné, `internal_rating` 1–5 nebo `null`,
`published_year` (rok prvního vydání) 1000 až příští rok nebo `null` a `author_ids`
musí obsahovat alespoň jednoho autora.

### Autoři

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `GET` | `/authors` | Seznam autorů | reader+ |
| `GET` | `/authors/{id}` | Detail autora | reader+ |
| `GET` | `/authors/{id}/image` | Fotka autora (soubor z `AUTHOR_IMAGE_ROOT`) | veřejné |
| `POST` | `/authors` | Přidání autora | editor+ |
| `PUT` | `/authors/{id}` | Aktualizace autora | editor+ |
| `PUT` | `/authors/{id}/image` | Stažení fotky ze zdroje metadat | editor+ |
| `DELETE` | `/authors/{id}/image` | Smazání fotky | editor+ |
| `DELETE` | `/authors/{id}` | Smazání autora | admin |

Jméno se posílá po částech (`first_name`, `middle_name`, `last_name`); povinné
je `last_name`. Místo částí lze poslat celé jméno v `name` – rozdělí se stejnou
logikou jako tagy (`"Komenský, Jan Amos"` i `"Jan Amos Komenský"`). Odpověď
obsahuje části i složené `name`.

Volitelně lze poslat `birth_year` a `death_year` (1000 až letošní rok, úmrtí
nesmí předcházet narození – jinak `400`).

Autor se stejnou trojicí jmen vrátí `409 Conflict`; stejně dopadne mazání
autora, který má v knihovně knihy.

**Fotka autora** se nenahrává souborem, ale stáhne se ze zdroje metadat:

```jsonc
// PUT /authors/{id}/image
{ "url": "https://www.databazeknih.cz/img/authors/10_/101/karel-capek-z04-101.jpg" }
```

Adresu určuje klient, proto se pouští jen hostitelé **zapnutých** zdrojů
metadat (viz níže) – cizí nebo vnitřní adresa skončí `400`, takže přes tenhle
endpoint nejde server donutit sáhnout kamkoliv. Odpověď musí být obrázek
(`Content-Type: image/*`, max 20 MB), jinak `502`. Soubor se uloží do
`AUTHOR_IMAGE_ROOT` jako `<author_id>.<přípona>` a jeho název jde do
`authors.image_path`.

`GET /authors/{id}/image` je veřejný ze stejného důvodu jako obálky knih –
`<img>` v prohlížeči neumí poslat hlavičku `Authorization`.

### Série

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `GET` | `/series` | Seznam sérií | reader+ |
| `GET` | `/series/{id}` | Detail série | reader+ |
| `POST` | `/series` | Přidání série | editor+ |
| `PUT` | `/series/{id}` | Aktualizace série | editor+ |
| `DELETE` | `/series/{id}` | Smazání série | admin |

### Poslech

Poslech (session) drží, co uživatel právě poslouchá: jednu knihu, celou sérii,
nebo ručně poskládaný seznam. Rozposlouchaných může být víc naráz. Session vidí
a mění jen její vlastník – cizí ID vrací `404`, aby o cizím účtu nic neprozradilo.

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `GET` | `/sessions` | Poslechy uživatele (nedoposlechnuté první) | reader+ |
| `POST` | `/sessions` | Založení nebo pokračování poslechu | reader+ |
| `GET` | `/sessions/{id}` | Detail poslechu | reader+ |
| `PUT` | `/sessions/{id}/position` | Uložení kapitoly a pozice | reader+ |
| `POST` | `/sessions/{id}/items` | Přidání knih a sérií na konec | reader+ |
| `DELETE` | `/sessions/{id}` | Smazání poslechu | reader+ |

Tělo `POST /sessions` určuje `kind`:

```jsonc
{ "kind": "book",   "book_id": "<uuid>" }
{ "kind": "series", "series_id": "<uuid>" }
{ "kind": "list",   "title": "Na cesty", "book_ids": ["<uuid>"], "series_ids": ["<uuid>"] }
```

U knihy a série vrací `200` s **existujícím** rozposlouchaným poslechem a `201`
jen u opravdu nového – druhé „Přehrát“ u téže knihy tak pokračuje místo
zakládání duplicity. Kniha se přitom hledá i uvnitř sérií a seznamů. Série se
rozbalí na díly podle `series_position` (díl bez pořadí jde na konec), seznam
vzniká vždy nový a duplicitní knihy v něm padají. Prázdný výběr vrací `400`.

`PUT /sessions/{id}/position` je zápis, který posílá přehrávač každých 10 sekund
poslechu a při každé změně:

```jsonc
{
  "book_id": "<uuid>",
  "chapter_id": "<uuid>",      // pozice se měří v kapitole, ne v celé knize
  "position_seconds": 124,
  "playback_speed": 1.25,      // 0.5 až 3.0
  "finished": false,           // true = doposlechnuto; další zápis příznak zruší
  "listened_seconds": 10,      // sekundy obsahu od minulého zápisu; server ořízne na 0–600
  "book_finished": false       // doposlechnutá poslední kapitola této knihy
}
```

`listened_seconds` jde do deníku poslechu: přehrávač sčítá jen plynulý posun
přehrávání, takže převíjení ani výměna kapitoly se nepočítají. Nesmyslná
hodnota se ořízne, pozice se uloží tak jako tak. `book_finished` označí knihu
za doposlechnutou – posílá se na konci její poslední kapitoly, dřív než poslech
přejde na další knihu. Doposlechnutí knihy pak už další poslech neruší, na
rozdíl od `finished` celého poslechu.

Každá kniha poslechu si nese vlastní kapitolu a pozici, takže skok na jiný díl
série nic neztratí. Kniha mimo poslech vrací `400`, stejně jako kapitola cizí
knihy nebo rychlost mimo rozsah.

### Audio

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `GET`, `HEAD` | `/chapters/{id}/audio?t=<token>` | Stream audio souboru kapitoly | stream token |

Odpovídá `http.ServeContent`, takže umí `Range` a vrací `206 Partial Content` –
přetáčení nestahuje soubor od začátku. Token do `t` vydá
`GET /auth/stream-token` (platnost 24 h, pouze streamování).

Cesta z `chapters.file_path` se před otevřením ověřuje proti `AUDIO_ROOT`, takže
ani ručně upravený záznam v databázi nepustí ven z knihovny.

Za reverzní proxy patří audio vlastní `location` s dlouhým čtecím timeoutem:
v pauze přestane prohlížeč číst a proxy by spojení jinak po pár minutách shodila.
Hotová konfigurace je v `deploy/nginx/`.

### Metadata knih

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `GET` | `/metadata/sources` | Zdroje v pořadí, ve kterém se zkoušejí | editor+ |
| `GET` | `/metadata/search?q=<dotaz>` | Vyhledání knihy | editor+ |
| `GET` | `/metadata/book?url=<url>` | Metadata knihy dle URL | editor+ |
| `GET` | `/metadata/book/{id}` | Metadata knihy dle ID databazeknih.cz | editor+ |
| `GET` | `/metadata/author/search?q=<dotaz>` | Vyhledání autora | editor+ |
| `GET` | `/metadata/author?url=<url>` | Metadata autora dle URL | editor+ |

Typický workflow editora:
```
GET /metadata/search?q=Sapkowski+Zaklínač
→ [ { "id": 1234, "title": "Zaklínač", "author": "", "year": 0,
      "url": "https://www.databazeknih.cz/prehled-knihy/...",
      "source": "databazeknih" }, ... ]

GET /metadata/book?url=https://www.databazeknih.cz/...
→ { "id": 160, "title": "...", "author": "...", "author_id": 101,
    "description": "...", "genres": [...], "cover_url": "...", "rating": 100,
    "publisher": "...", "year": 1936, "original_title": "", "source_url": "...",
    "source": "databazeknih" }

PATCH /books/{id}   (s daty z metadat)
```

Odpovědi jsou v `snake_case` jako zbytek API. `rating` je hodnocení zdroje
v procentech (0–100) – **není** to `internal_rating` knihy (1–5). `year` je
rok **prvního vydání díla** – u překladů rok originálu (databazeknih.cz ho
bere ze sekce „Více info“, OpenLibrary z `first_publish_year`); Google Books
zná jen rok konkrétního vydání. `original_title` je název originálu
u překladů. Nevyplněná pole zůstávají nulová: ne každý zdroj dává autora a rok
už v seznamu výsledků a ne každý zná nakladatele.

#### Zdroje a jejich pořadí

Které zdroje se používají a v jakém pořadí, nastavuje **admin v administraci**
(Administrace → Zdroje metadat). Změna platí okamžitě, server se nerestartuje.
`METADATA_PROVIDERS` a `GOOGLE_BOOKS_API_KEY` z `.env` slouží jen jako výchozí
hodnota, dokud nastavení nikdo neuložil; potom vyhrává databáze.

Zkouší se odshora a vrátí se výsledky **prvního, který něco najde** – zdroj,
který spadne nebo nic nevrátí, se přeskočí. Rozbitý scraper tak funkci
nezablokuje. Vypnutý zdroj se nepoužije ani pro stahování fotek autorů;
s vypnutými všemi zdroji metadata nefungují vůbec.

| Zdroj | Typ | Poznámka |
|-------|-----|----------|
| `databazeknih` | scraper HTML | Nejlepší pokrytí českých titulů. Vrací i žánry, nakladatele, hodnocení a u překladů název a rok originálu (druhý požadavek na sekci „Více info“). |
| `cbdb` | scraper HTML | Česká databáze, dobrý doplněk. Nedává rok vydání ani nakladatele (patří konkrétnímu vydání). |
| `openlibrary` | oficiální JSON API | Zdarma, bez klíče a bez limitu. Česká beletrie je děravá. Rok ani nakladatel u díla nejsou. |
| `googlebooks` | oficiální JSON API | Bez klíče platí anonymní denní kvóta **sdílená pro celou IP** – snadno se vyčerpá (`429`). Vlastní klíč se zadá do `GOOGLE_BOOKS_API_KEY`. Autory jako samostatné záznamy nemá, hledá jen knihy. |

`GET /metadata/book?url=` si zdroj vybere podle domény v adrese; tím zároveň
vzniká allowlist, protože cizí adresu neobslouží nikdo (`400`). Endpoint
`/metadata/book/{id}` pracuje s číselným ID specifickým pro databazeknih.cz
a vrací `404`, když tento zdroj není zapnutý.

`GET /metadata/sources` vrátí, které zdroje jsou zapnuté a v jakém pořadí –
zvlášť pro knihy a zvlášť pro autory:

```jsonc
{ "books":   ["databazeknih", "cbdb", "openlibrary", "googlebooks"],
  "authors": ["databazeknih", "cbdb", "openlibrary"] }
```

#### Metadata autorů

`GET /metadata/author/search?q=` a `GET /metadata/author?url=` fungují stejně
jako u knih, jen je neumí `googlebooks`. Vrací jméno (celé i rozdělené na
části stejně, jako se ukládá u autora), životopis, roky života a adresu fotky:

```jsonc
// GET /metadata/author?url=https://www.databazeknih.cz/autori/karel-capek-101
{ "id": 101, "name": "Karel Čapek",
  "first_name": "Karel", "middle_name": "", "last_name": "Čapek",
  "bio": "Český prozaik, dramatik…",
  "image_url": "https://www.databazeknih.cz/img/authors/…/karel-capek-z04-101.jpg",
  "birth_year": 1890, "death_year": 1938,
  "source_url": "https://www.databazeknih.cz/autori/karel-capek-101",
  "source": "databazeknih" }
```

Fotku stáhne až `PUT /authors/{id}/image` (viz Autoři výše).

Co který zdroj u autorů dá:

| Zdroj | Životopis | Roky | Fotka |
|-------|-----------|------|-------|
| `databazeknih` | ano (nejobsáhlejší) | ano | ano |
| `cbdb` | ano | ano | ano |
| `openlibrary` | často prázdný | ano | jen někdy |

Na přehledu autora má databazeknih.cz životopis zkrácený zhruba na 250 znaků
(a jen v JSON-LD), proto se celý text dotahuje ze stránky `/zivotopis/<slug>-<id>`
druhým požadavkem. U popisů knih se odřezává ovládací odkaz „… celý text“
a u cbdb.cz patička „(Založil/a: …)“ – text samotný zkrácený není.

U OpenLibrary je potřeba počítat s duplicitními záznamy téhož autora – většina
z nich je prázdná, proto je ve výchozím pořadí až za českými zdroji.

České zdroje čtou detail autora přednostně z JSON-LD (`schema.org/Person`),
které je proti změnám rozložení stránky odolnější než hledání v HTML.

Scrapery stojí na HTML cizích webů a ty se mění bez ohlášení – databazeknih.cz
si například přesunul vyhledávání z `/hledat` na `/search?in=books`. Když se
zdá, že import nefunguje, nejrychleji to prověří živý smoke test:

```bash
cd libriter-backend
LIBRITER_LIVE_METADATA=1 go test ./internal/metadata/ -run Live -v
```

Běžné `go test ./...` na cizí weby nechodí, tenhle test se bez proměnné
přeskakuje. Selhání zdroje se v API projeví jako `502` s konkrétním důvodem
v těle, vypršení časového limitu jako `504`.

### Zdraví serveru

```
GET /health
```

### Ostatní cesty

Vše mimo `/health` a `/api/v1/*` obsluhuje webové rozhraní: existující soubor se
odešle přímo, jinak se vrátí `index.html` a o cestu se postará routing v prohlížeči.
Neznámé cesty pod `/api/v1/` vracejí JSON `{"error":"endpoint nenalezen"}`.

---

## Role a oprávnění

| Role | Čtení knih | Editace knih | Mazání | Správa uživatelů | Administrace |
|------|:----------:|:------------:|:------:|:----------------:|:------------:|
| **reader** | ✓ | — | — | — | — |
| **editor** | ✓ | ✓ | — | — | — |
| **admin** | ✓ | ✓ | ✓ | ✓ | ✓ |

Roli nově registrovaného uživatele i to, jestli je registrace vůbec otevřená,
nastavuje admin (výchozí stav: registrace zapnutá, role **reader**). Role
existujícího účtu mění pouze admin – v administraci nebo přes
`PUT /users/{id}/role`.

Role se při ověření každého požadavku čte z databáze, ne z tokenu (s desetivteřinovou
cache), takže snížení role nebo smazání účtu platí hned i pro už vydané tokeny.
