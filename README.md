# Libriter

Osobní správce a přehrávač audioknih s webovým rozhraním a mobilní aplikací.

## Architektura

```
libriter/
├── libriter-backend/   # REST API (Go)
├── libriter-frontend/  # Webové rozhraní
├── libriter-mobile/    # Mobilní aplikace
├── _scripts/           # Pomocné skripty
└── data/
    ├── libriter.db     # SQLite databáze (DB_PATH) – vytvoří se automaticky
    ├── audio/          # Audio soubory (AUDIO_ROOT)
    └── covers/         # Obálky knih (COVER_ROOT)
```

Databázové schéma je v `libriter-backend/internal/db/migrations/` a aplikuje se
automaticky při startu backendu.

---

## Požadavky

### Systémové závislosti

| Nástroj | Verze | Účel | Instalace |
|---------|-------|------|-----------|
| **Go** | ≥ 1.21 | Backend | viz níže |
| **ffprobe** | libovolná | Délka audio souborů (scanner) | součást balíčku `ffmpeg` |

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

#### Developer tools

```bash
sudo apt install build-essential git cmake pkg-config
```

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

---

## Instalace a spuštění backendu

### 1. Vytvoření adresářů pro data

```bash
mkdir -p data/audio data/covers
```

### 2. Stažení Go závislostí a build

```bash
cd libriter-backend
go mod download
go build ./...
```

### 3. Spuštění serveru

```bash
cd libriter-backend
go run ./cmd/server
```

Server se spustí na portu nastaveném v `SERVER_PORT` (výchozí `8080`).

Při prvním startu backend automaticky:
- vytvoří soubor databáze na cestě `DB_PATH` (včetně adresáře)
- aplikuje migrace z `internal/db/migrations/` – všechny tabulky
  (authors, series, books, chapters, tags, users, roles, permissions, ratings, ...)
- založí výchozí role **admin**, **editor**, **reader** a jejich oprávnění

Aplikované migrace se evidují v tabulce `schema_migrations`; při dalších
startech se spustí jen nové. Zálohu databáze pořídíte prostým zkopírováním
souboru `libriter.db` (při běžícím serveru i souborů `-wal` a `-shm`).

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

---

## Role a oprávnění

| Role | Čtení knih | Editace knih | Mazání | Správa uživatelů |
|------|:----------:|:------------:|:------:|:----------------:|
| **reader** | ✓ | — | — | — |
| **editor** | ✓ | ✓ | — | — |
| **admin** | ✓ | ✓ | ✓ | ✓ |

Nový uživatel dostane automaticky roli **reader**.
Role mění pouze admin přes `PUT /users/{id}/role`.
