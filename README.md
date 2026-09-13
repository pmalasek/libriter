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
    └── covers/         # Obálky knih (COVER_ROOT)
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
COVER_ROOT=./data/covers        # kořenový adresář obrázků
MAX_UPLOAD_MB=500
```

> **Bezpečnost:** `JWT_SECRET` musí být v produkci silný náhodný řetězec.
> Vygenerujte ho např. pomocí: `openssl rand -hex 32`

Webové rozhraní ani CLI žádné další proměnné nepotřebují – `.env` zůstává stejný.
Vývojový server Vite přebírá adresu backendu z `BACKEND_URL` (výchozí
`http://localhost:8080`), což je proměnná prostředí, ne součást `.env`.

**Jak se `.env` hledá:** v aktuálním adresáři, pak v jeho podadresáři
`libriter-backend/`, a takto dál v nadřazených adresářích. Cestu lze vynutit
proměnnou `LIBRITER_ENV_FILE`. Skutečné proměnné prostředí mají vždy přednost
před hodnotami z `.env`.

**Relativní cesty** (`DB_PATH`, `AUDIO_ROOT`, `COVER_ROOT`) zapsané v `.env` se
vztahují k adresáři toho `.env` – ne k aktuálnímu adresáři. `bin/libriter` tak
míří na stejná data, ať ho spustíte odkudkoli. Cesta předaná proměnnou prostředí
se ponechává tak, jak je.

---

## Instalace a spuštění

### 1. Vytvoření adresářů pro data

```bash
mkdir -p data/audio data/covers
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
  (authors, series, books, chapters, tags, users, roles, permissions, ratings, ...)
- založí výchozí role **admin**, **editor**, **reader** a jejich oprávnění

Aplikované migrace se evidují v tabulce `schema_migrations`; při dalších
startech se spustí jen nové. Zálohu databáze pořídíte prostým zkopírováním
souboru `libriter.db` (při běžícím serveru i souborů `-wal` a `-shm`).

---

## Webové rozhraní

React + TypeScript + Tailwind v4, komponenty shadcn/ui (Radix), routing
`react-router`, serverový stav `@tanstack/react-query`. Vše v češtině.

**Co první verze umí:** přihlášení a registraci, seznam a detail knih, autory,
série, profil (změna jména, e-mailu a hesla), hledání v knihách, světlý i tmavý
režim podle systému.

**Co ještě ne:** přehrávání audia, obálky knih a editační formuláře – backend pro
ně zatím nemá endpointy (chybí kapitoly, streamování a servírování `COVER_ROOT`).
Knihy, autory a série proto zakládá scanner nebo přímé volání API.

Přihlášený uživatel se drží v `localStorage` (JWT + profil). Role se obnoví až
při dalším přihlášení, takže po změně role adminem je nutné se odhlásit a
přihlásit znovu.

---

## Správa uživatelů z příkazové řádky

Registrace přes API dává vždy roli **reader**, takže prvního administrátora
vytvořte přes CLI stejné binárky:

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

---

## Audio scanner

Scanner se spustí **automaticky při startu backendu** a:

1. Při startu projde `AUDIO_ROOT` a přidá do databáze všechny audio soubory, které v ní ještě nejsou
2. Sleduje `AUDIO_ROOT` (vč. podadresářů) a reaguje na nové soubory v reálném čase
3. Čeká, dokud se soubor nepřestane měnit (kopírování dokončeno) – kontrola každé 3 s, stabilita 10 s
4. Extrahuje metadata z audio tagů a vloží knihu do databáze

**Podporované formáty:** `.mp3`, `.m4a`, `.m4b`, `.ogg`, `.flac`, `.opus`, `.aac`, `.wav`

**Priorita metadat z tagů:**

| Pole | Pořadí čtení |
|------|-------------|
| Název knihy | `Album` → `Title` tag → název souboru |
| Autor | `AlbumArtist` → `Composer` → `Artist` → název adresáře |
| Vypravěč | `Artist` (pokud se liší od autora) |
| Délka | `ffprobe` → `1 s` (placeholder, opravit přes API) |

---

## API přehled

Základní URL: `http://localhost:8080/api/v1`

### Autentikace

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `POST` | `/auth/register` | Registrace nového uživatele | veřejné |
| `POST` | `/auth/login` | Přihlášení, vrátí JWT token | veřejné |

Všechny ostatní endpointy vyžadují hlavičku:
```
Authorization: Bearer <token>
```

### Uživatelé

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `GET` | `/users` | Seznam uživatelů | admin |
| `GET` | `/users/{id}` | Detail uživatele | admin / vlastní profil |
| `PUT` | `/users/{id}` | Aktualizace jména a emailu | admin / vlastní profil |
| `PUT` | `/users/{id}/password` | Změna hesla | admin / vlastní profil |
| `PUT` | `/users/{id}/role` | Nastavení role | admin |
| `DELETE` | `/users/{id}` | Smazání uživatele | admin |

### Knihy

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `GET` | `/books` | Seznam knih | reader+ |
| `GET` | `/books/{id}` | Detail knihy | reader+ |
| `POST` | `/books` | Přidání knihy | editor+ |
| `PUT` | `/books/{id}` | Aktualizace knihy | editor+ |
| `DELETE` | `/books/{id}` | Smazání knihy | admin |

### Autoři

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `GET` | `/authors` | Seznam autorů | reader+ |
| `GET` | `/authors/{id}` | Detail autora | reader+ |
| `POST` | `/authors` | Přidání autora | editor+ |
| `PUT` | `/authors/{id}` | Aktualizace autora | editor+ |
| `DELETE` | `/authors/{id}` | Smazání autora | admin |

### Série

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `GET` | `/series` | Seznam sérií | reader+ |
| `GET` | `/series/{id}` | Detail série | reader+ |
| `POST` | `/series` | Přidání série | editor+ |
| `PUT` | `/series/{id}` | Aktualizace série | editor+ |
| `DELETE` | `/series/{id}` | Smazání série | admin |

### Metadata (databazeknih.cz)

| Metoda | Endpoint | Popis | Přístup |
|--------|----------|-------|---------|
| `GET` | `/metadata/search?q=<dotaz>` | Vyhledání knihy | editor+ |
| `GET` | `/metadata/book/{id}` | Metadata dle DK ID | editor+ |
| `GET` | `/metadata/book?url=<url>` | Metadata dle URL | editor+ |

Typický workflow editora:
```
GET /metadata/search?q=Sapkowski+Zaklínač
→ [ { "id": 1234, "title": "Zaklínač", "url": "https://..." }, ... ]

GET /metadata/book?url=https://www.databazeknih.cz/...
→ { "title": "...", "author": "...", "description": "...", "cover_url": "..." }

POST /books   (s daty z metadat)
```

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

| Role | Čtení knih | Editace knih | Mazání | Správa uživatelů |
|------|:----------:|:------------:|:------:|:----------------:|
| **reader** | ✓ | — | — | — |
| **editor** | ✓ | ✓ | — | — |
| **admin** | ✓ | ✓ | ✓ | ✓ |

Nový uživatel dostane automaticky roli **reader**.
Role mění pouze admin přes `PUT /users/{id}/role`.
