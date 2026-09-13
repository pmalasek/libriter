# Libriter – build orchestrace
#
# Frontend se sestavuje přímo do libriter-backend/internal/web/dist a odtud
# se přes go:embed vkompiluje do binárky. `just backend` funguje i bez
# sestaveného frontendu – server pak na / vrací stránku s návodem (503).

bin          := "bin/libriter"
frontend_dir := "libriter-frontend"
backend_dir  := "libriter-backend"
dist         := backend_dir / "internal/web/dist"

[private]
default:
    @just --list

# Sestaví frontend i backend do bin/libriter
build: frontend backend

# npm ci && npm run build (výstup do internal/web/dist)
[working-directory('libriter-frontend')]
frontend:
    npm ci
    npm run build

# go build -o bin/libriter (vkompiluje aktuální dist)
backend:
    mkdir -p bin
    cd {{backend_dir}} && CGO_ENABLED=0 go build -o ../{{bin}} ./cmd/server

# Vývojový server na :8080
[working-directory('libriter-backend')]
dev-backend:
    go run ./cmd/server

# Vite na :5173, proxy /api na :8080
[working-directory('libriter-frontend')]
dev-frontend:
    npm run dev

# go vet ./...
[working-directory('libriter-backend')]
vet:
    go vet ./...

# Smaže bin/ a obsah dist/
clean:
    rm -rf bin
    find {{dist}} -mindepth 1 ! -name .gitkeep -delete
