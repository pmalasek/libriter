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
# Verzi vkládá linker; administrace ji ukazuje v přehledu systému. Release
# (`just deploy`) ji předává přes LIBRITER_VERSION, jinak platí `git describe`.
backend:
    mkdir -p bin
    cd {{backend_dir}} && CGO_ENABLED=0 go build \
        -ldflags "-X libriter/internal/version.Version=${LIBRITER_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}" \
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

# Závislosti mobilní aplikace. Instalují se v kořeni: mobil, web
# i libriter-shared jsou npm workspaces s jedním package-lock.json, takže
# `lucide-react-native` a spol. leží v kořenovém node_modules. Bez tohohle
# kroku padá bundling po každém `git pull`, který přidal závislost, na
# „Unable to resolve module …“.
[private]
mobile-deps:
    npm ci

# Metro bundler mobilní aplikace (dev build, ne Expo Go)
[working-directory('libriter-mobile')]
mobile-start: mobile-deps
    npx expo start --dev-client

# `pod install` musí předcházet buildu: podspec expo-sqlite si při instalaci
# kopíruje vendorované sqlite3.c a .h do node_modules/expo-sqlite/ios/ a
# `npm ci` (viz recept frontend) node_modules přeinstaluje, takže je smaže.
# Bez nich build padá na „cannot find 'exsqlite3_open' in scope“.
#
# Sestaví a spustí aplikaci na připojeném iPhonu (jen macOS + Xcode)
[working-directory('libriter-mobile')]
mobile-ios: mobile-deps mobile-node-env mobile-pods
    npx expo run:ios --device

# Řekne Xcode, kde je node. Build fáze Hermesu ho spouští, ale Xcode nemá
# v PATH nvm ani Homebrew, takže cestu bere z ios/.xcode.env.local. Ten
# soubor vygeneroval `expo prebuild` s absolutní cestou a po upgradu nvm
# ukazuje do prázdna – build pak padá na „node: No such file or directory“.
# Přepíše se proto tím node, který právě běží.
[working-directory('libriter-mobile')]
[private]
mobile-node-env:
    #!/usr/bin/env bash
    set -euo pipefail
    [ -d ios ] || exit 0
    echo "export NODE_BINARY=$(command -v node)" > ios/.xcode.env.local

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
mobile-android: mobile-deps
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -r "$HOME/.config/libriter/android-env.sh" ]; then
        . "$HOME/.config/libriter/android-env.sh"
    fi
    npx expo run:android --device

# Release APK k ruční instalaci → libriter-mobile/dist/libriter.apk
mobile-apk: (mobile-android-release "assembleRelease" "apk/release/app-release.apk" "libriter.apk")

# Release AAB pro Google Play → libriter-mobile/dist/libriter.aab
mobile-aab: (mobile-android-release "bundleRelease" "bundle/release/app-release.aab" "libriter.aab")

# Podepisuje klíčem z ~/.gradle/gradle.properties (just mobile-keystore, čte
# ho plugins/withAndroidReleaseSigning.js); bez něj debug klíčem.
[working-directory('libriter-mobile')]
[private]
mobile-android-release task artifact out: mobile-deps
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -r "$HOME/.config/libriter/android-env.sh" ]; then
        . "$HOME/.config/libriter/android-env.sh"
    fi
    if ! grep -qs '^LIBRITER_UPLOAD_STORE_FILE=' "$HOME/.gradle/gradle.properties"; then
        echo "⚠ Release klíč nenastavený (just mobile-keystore) – podepisuji debug klíčem, do Google Play to neprojde." >&2
    fi
    npx expo prebuild --platform android
    (cd android && ./gradlew {{task}})
    mkdir -p dist
    cp "android/app/build/outputs/{{artifact}}" "dist/{{out}}"
    echo "→ {{mobile_dir}}/dist/{{out}}"

# Stačí jednou. Klíč zálohuj – bez něj už nejde vydat aktualizaci aplikace
# v Google Play.
#
# Vytvoří podepisovací klíč pro release build Androidu
mobile-keystore:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -r "$HOME/.config/libriter/android-env.sh" ]; then
        . "$HOME/.config/libriter/android-env.sh"
    fi
    ks="$HOME/.config/libriter/android-upload.jks"
    props="$HOME/.gradle/gradle.properties"
    if [ -e "$ks" ]; then
        echo "Klíč už existuje: $ks – nový by znemožnil aktualizace už vydané aplikace." >&2
        exit 1
    fi
    read -rsp "Heslo ke klíči (aspoň 6 znaků): " pw; echo
    read -rsp "Heslo znovu: " pw2; echo
    [ "$pw" = "$pw2" ] || { echo "Hesla se neshodují." >&2; exit 1; }
    [ "${#pw}" -ge 6 ] || { echo "Heslo je kratší než 6 znaků." >&2; exit 1; }
    mkdir -p "$(dirname "$ks")" "$(dirname "$props")"
    KS_PASS="$pw" keytool -genkeypair -storetype PKCS12 -keystore "$ks" \
        -alias libriter -keyalg RSA -keysize 2048 -validity 10000 -dname "CN=Libriter" \
        -storepass:env KS_PASS -keypass:env KS_PASS
    chmod 600 "$ks"
    touch "$props" && chmod 600 "$props"
    sed -i.bak '/^LIBRITER_UPLOAD_/d' "$props" && rm -f "$props.bak"
    {
        echo "LIBRITER_UPLOAD_STORE_FILE=$ks"
        echo "LIBRITER_UPLOAD_STORE_PASSWORD=$pw"
        echo "LIBRITER_UPLOAD_KEY_ALIAS=libriter"
        echo "LIBRITER_UPLOAD_KEY_PASSWORD=$pw"
    } >> "$props"
    echo "✓ Klíč: $ks, hesla v $props"

# Jen macOS; potřebuje placený Apple Developer účet a ios.appleTeamId
# v app.json (nebo LIBRITER_APPLE_TEAM). method: app-store-connect (TestFlight
# a App Store) nebo release-testing (ad hoc na registrovaná zařízení).
#
# Release IPA → libriter-mobile/dist/Libriter.ipa
[working-directory('libriter-mobile')]
mobile-ipa method="app-store-connect": mobile-deps
    #!/usr/bin/env bash
    set -euo pipefail
    team="${LIBRITER_APPLE_TEAM:-$(node -p "require('./app.json').expo.ios.appleTeamId ?? ''")}"
    if [ -z "$team" ]; then
        echo "Chybí Apple Team ID: doplň ios.appleTeamId do app.json nebo nastav LIBRITER_APPLE_TEAM." >&2
        exit 1
    fi
    npx expo prebuild --platform ios
    {{just_executable()}} mobile-node-env mobile-pods
    out="$PWD/dist"
    archive="$PWD/ios/build/Libriter.xcarchive"
    mkdir -p "$out"
    xcodebuild -workspace ios/Libriter.xcworkspace -scheme Libriter \
        -configuration Release -destination 'generic/platform=iOS' \
        -archivePath "$archive" -allowProvisioningUpdates \
        DEVELOPMENT_TEAM="$team" CODE_SIGN_STYLE=Automatic archive
    tmp="$(mktemp -d)"
    opts="$tmp/ExportOptions.plist"
    cat > "$opts" <<EOF
    <?xml version="1.0" encoding="UTF-8"?>
    <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
    <plist version="1.0"><dict>
      <key>method</key><string>{{method}}</string>
      <key>teamID</key><string>$team</string>
      <key>signingStyle</key><string>automatic</string>
      <key>destination</key><string>export</string>
    </dict></plist>
    EOF
    xcodebuild -exportArchive -archivePath "$archive" -exportPath "$out" \
        -exportOptionsPlist "$opts" -allowProvisioningUpdates
    rm -rf "$tmp"
    echo "→ {{mobile_dir}}/dist/Libriter.ipa"

# Nový release: zvýší verzi (patch/minor/major), sestaví .deb a vystaví ho na GitHubu
deploy:
    bash _scripts/release.sh

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
