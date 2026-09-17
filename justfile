# Libriter – build orchestrace
#
# Frontend se sestavuje přímo do libriter-backend/internal/web/dist a odtud
# se přes go:embed vkompiluje do binárky. `just backend` funguje i bez
# sestaveného frontendu – server pak na / vrací stránku s návodem (503).

bin          := "bin/libriter"
frontend_dir := "libriter-frontend"
backend_dir  := "libriter-backend"
mobile_dir   := "libriter-mobile"
dist         := backend_dir / "internal/web/dist"

[private]
default:
    @just --list

# Sestaví frontend i backend do bin/libriter
build: frontend backend

# Závislosti se instalují v kořeni: web, mobil i libriter-shared jsou npm
# workspaces s jedním společným package-lock.json.
#
# npm ci && npm run build (výstup do internal/web/dist)
frontend:
    npm ci
    npm run build -w {{frontend_dir}}

# go build -o bin/libriter (vkompiluje aktuální dist)
# Verzi vkládá linker; administrace ji ukazuje v přehledu systému.
backend:
    mkdir -p bin
    cd {{backend_dir}} && CGO_ENABLED=0 go build \
        -ldflags "-X libriter/internal/version.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
        -o ../{{bin}} ./cmd/server

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

# tsc ve všech npm workspaces (shared, web, mobil)
typecheck:
    npm run typecheck

# Metro bundler mobilní aplikace (dev build, ne Expo Go)
[working-directory('libriter-mobile')]
mobile-start:
    npx expo start --dev-client

# `pod install` musí předcházet buildu: podspec expo-sqlite si při instalaci
# kopíruje vendorované sqlite3.c a .h do node_modules/expo-sqlite/ios/ a
# `npm ci` (viz recept frontend) node_modules přeinstaluje, takže je smaže.
# Bez nich build padá na „cannot find 'exsqlite3_open' in scope“.
#
# Sestaví a spustí aplikaci na připojeném iPhonu (jen macOS + Xcode)
[working-directory('libriter-mobile')]
mobile-ios: mobile-pods
    npx expo run:ios --device

# Doinstaluje CocoaPods, pokud už existuje vygenerovaný projekt ios/
[working-directory('libriter-mobile')]
[private]
mobile-pods:
    #!/usr/bin/env bash
    set -euo pipefail
    [ -d ios ] || exit 0
    cd ios && pod install

# Sestaví a spustí aplikaci na připojeném Androidu
[working-directory('libriter-mobile')]
mobile-android:
    npx expo run:android --device

# Podepsané APK k ruční instalaci (android/app/build/outputs/apk/release/)
[working-directory('libriter-mobile')]
mobile-apk:
    npx expo prebuild --platform android
    cd android && ./gradlew assembleRelease

# Nainstaluje vývojové nástroje: Go, Node, Android SDK; na macOS i iOS (Xcode CLT, CocoaPods)
setup:
    bash _scripts/setup.sh

# Zkontroluje, že jsou dostupné nástroje pro backend, web, Android a iOS
doctor:
    bash _scripts/doctor.sh

# Smaže bin/ a obsah dist/
clean:
    rm -rf bin
    find {{dist}} -mindepth 1 ! -name .gitkeep -delete
