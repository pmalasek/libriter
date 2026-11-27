# Mediater (Libriter + Sceniter) – build orchestrace
#
# Weby se sestavují přímo do server/internal/<produkt>/web/dist a odtud se
# přes go:embed vkompilují do binárky. `just backend` funguje i bez
# sestavených webů – server pak na / vrací stránku s návodem (503).

bin          := "bin/mediater"
frontend_dir := "libriter-frontend"
sceniter_dir := "sceniter-frontend"
backend_dir  := "server"
mobile_dir   := "libriter-mobile"
tv_dir       := "sceniter-tv"
dist         := backend_dir / "internal/libriter/web/dist"
sceniter_dist := backend_dir / "internal/sceniter/web/dist"

[private]
default:
    @just --list

# Sestaví oba weby, backend do bin/mediater a pluginy metadat do bin/plugins
build: frontend backend plugins

# Závislosti se instalují v kořeni: weby, mobil i sdílené balíčky jsou npm
# workspaces s jedním společným package-lock.json.
#
# npm ci && build obou webů (výstup do internal/{libriter,sceniter}/web/dist)
frontend:
    npm ci
    npm run build -w {{frontend_dir}}
    npm run build -w {{sceniter_dir}}

# go build -o bin/mediater (vkompiluje aktuální dist)
# Verzi vkládá linker; administrace ji ukazuje v přehledu systému. Release
# (`just deploy`) ji předává přes MEDIATER_VERSION, jinak platí `git describe`.
backend:
    mkdir -p bin
    cd {{backend_dir}} && CGO_ENABLED=0 go build \
        -ldflags "-X mediater/internal/core/version.Version=${MEDIATER_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}" \
        -o ../{{bin}} ./cmd/server

# Server najde pluginy v plugins/ vedle binárky (nebo v PLUGIN_DIR). chmod 755:
# soubor zapisovatelný pro skupinu (umask 002) server nespustí.
#
# Pluginy metadat (server/cmd/plugins/*) do bin/plugins/mediater-meta-<jméno>
plugins:
    #!/usr/bin/env bash
    set -euo pipefail
    version="${MEDIATER_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
    mkdir -p bin/plugins
    cd {{backend_dir}}
    for dir in cmd/plugins/*/; do
        name=$(basename "$dir")
        out="../bin/plugins/mediater-meta-$name"
        CGO_ENABLED=0 go build -ldflags "-X mediater/internal/core/version.Version=$version" -o "$out" "./$dir"
        chmod 755 "$out"
    done
    ls ../bin/plugins

# Vývojový server na :8080 (pluginy metadat z bin/plugins)
[working-directory('server')]
dev-backend: plugins
    PLUGIN_DIR="${PLUGIN_DIR:-../bin/plugins}" go run ./cmd/server

# Vite na :5173, proxy /api na :8080
[working-directory('libriter-frontend')]
dev-frontend:
    npm run dev

# Web Sceniteru: Vite na :5174, proxy /api na :8080
[working-directory('sceniter-frontend')]
dev-sceniter:
    npm run dev

# Kontrola úplnosti překladů (jádro, Libriter, Sceniter)
i18n-check:
    npm run i18n:check -w core-shared
    npm run i18n:check -w libriter-shared
    npm run i18n:check -w sceniter-shared

# go vet ./...
[working-directory('server')]
vet:
    go vet ./...

# Hlásí jen zranitelnosti v kódu, který server opravdu volá. Nálezy ve
# standardní knihovně opraví novější Go toolchain, ne go get.
#
# Známé zranitelnosti v Go závislostech (govulncheck)
[working-directory('server')]
vulncheck:
    go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# tsc ve všech npm workspaces (sdílené balíčky, weby, mobil) a v TV aplikaci
typecheck:
    npm run typecheck
    cd {{tv_dir}} && npx tsc --noEmit

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
mobile-aab: mobile-version (mobile-android-release "bundleRelease" "bundle/release/app-release.aab" "libriter.aab")

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
# v Google Play. Na dalším stroji ho nevytvářej znovu, ale naimportuj
# (just mobile-keystore-import).
#
# Vytvoří podepisovací klíč pro release build Androidu
mobile-keystore:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -r "$HOME/.config/libriter/android-env.sh" ]; then
        . "$HOME/.config/libriter/android-env.sh"
    fi
    ks="$HOME/.config/libriter/android-upload.jks"
    if [ -e "$ks" ]; then
        echo "Klíč už existuje: $ks – nový by znemožnil aktualizace už vydané aplikace." >&2
        exit 1
    fi
    echo "Máš-li už klíč z jiného stroje, přenes ho: just mobile-keystore-import <soubor.jks>"
    read -rp "Opravdu vytvořit NOVÝ klíč? [a/N] " yn
    [[ "$yn" =~ ^[aAyY]$ ]] || exit 1
    read -rsp "Heslo ke klíči (aspoň 6 znaků): " pw; echo
    read -rsp "Heslo znovu: " pw2; echo
    [ "$pw" = "$pw2" ] || { echo "Hesla se neshodují." >&2; exit 1; }
    [ "${#pw}" -ge 6 ] || { echo "Heslo je kratší než 6 znaků." >&2; exit 1; }
    mkdir -p "$(dirname "$ks")"
    KS_PASS="$pw" keytool -genkeypair -storetype PKCS12 -keystore "$ks" \
        -alias libriter -keyalg RSA -keysize 2048 -validity 10000 -dname "CN=Libriter" \
        -storepass:env KS_PASS -keypass:env KS_PASS
    chmod 600 "$ks"
    KS_PASS="$pw" KEY_PASS="$pw" {{just_executable()}} mobile-keystore-props libriter
    echo "✓ Klíč: $ks"

# Hesla jsou v $KS_PASS a $KEY_PASS, ne v argumentech (ty vidí `ps`).
#
# Zapíše LIBRITER_UPLOAD_* do ~/.gradle/gradle.properties
[private]
mobile-keystore-props alias:
    #!/usr/bin/env bash
    set -euo pipefail
    props="$HOME/.gradle/gradle.properties"
    mkdir -p "$(dirname "$props")"
    touch "$props" && chmod 600 "$props"
    sed -i.bak '/^LIBRITER_UPLOAD_/d' "$props" && rm -f "$props.bak"
    {
        echo "LIBRITER_UPLOAD_STORE_FILE=$HOME/.config/libriter/android-upload.jks"
        echo "LIBRITER_UPLOAD_STORE_PASSWORD=$KS_PASS"
        echo "LIBRITER_UPLOAD_KEY_ALIAS={{alias}}"
        echo "LIBRITER_UPLOAD_KEY_PASSWORD=$KEY_PASS"
    } >> "$props"
    echo "✓ Hesla v $props"

# Klíč z jiného stroje nebo ze zálohy: ověří heslo, zkopíruje ho do
# ~/.config/libriter/android-upload.jks a zapíše gradle.properties. alias je
# potřeba jen u úložiště s víc klíči.
#
# Naimportuje existující podepisovací klíč pro release build Androidu
mobile-keystore-import file alias="":
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -r "$HOME/.config/libriter/android-env.sh" ]; then
        . "$HOME/.config/libriter/android-env.sh"
    fi
    src={{quote(file)}}
    case "$src" in /*) ;; *) src={{quote(invocation_directory())}}/"$src" ;; esac
    [ -r "$src" ] || { echo "Soubor nejde přečíst: $src" >&2; exit 1; }
    ks="$HOME/.config/libriter/android-upload.jks"
    if [ -e "$ks" ] && ! cmp -s "$src" "$ks"; then
        echo "Na $ks už je jiný klíč. Pokud ho opravdu chceš nahradit, nejdřív ho zazálohuj a smaž." >&2
        exit 1
    fi
    # anglický výstup keytoolu, ať jde parsovat
    kt() { keytool -J-Duser.language=en "$@"; }
    read -rsp "Heslo k úložišti klíčů: " pw; echo
    export KS_PASS="$pw"
    entries="$(kt -list -keystore "$src" -storepass:env KS_PASS 2>&1)" \
        || { echo "Úložiště nejde otevřít (špatné heslo?): $entries" >&2; exit 1; }
    alias={{quote(alias)}}
    if [ -z "$alias" ]; then
        aliases="$(printf '%s\n' "$entries" | grep 'PrivateKeyEntry' | cut -d, -f1)"
        if [ "$(printf '%s\n' "$aliases" | grep -c .)" -ne 1 ]; then
            echo "Úložiště nemá právě jeden klíč, zadej alias: just mobile-keystore-import <soubor> <alias>" >&2
            printf '%s\n' "$aliases" >&2
            exit 1
        fi
        alias="$aliases"
    fi
    # certreq potřebuje privátní klíč, takže ověří i heslo ke klíči
    key_ok() { KEY_PASS="$1" kt -certreq -keystore "$src" -alias "$alias" \
        -storepass:env KS_PASS -keypass:env KEY_PASS -file /dev/null >/dev/null 2>&1; }
    export KEY_PASS="$pw"
    if ! key_ok "$KEY_PASS"; then
        read -rsp "Heslo ke klíči '$alias' (liší se od hesla úložiště): " KEY_PASS; echo
        key_ok "$KEY_PASS" || { echo "Heslo ke klíči nesedí." >&2; exit 1; }
    fi
    mkdir -p "$(dirname "$ks")"
    [ "$src" -ef "$ks" ] || cp "$src" "$ks"
    chmod 600 "$ks"
    {{just_executable()}} mobile-keystore-props "$alias"
    echo "✓ Klíč '$alias': $ks"

# Jen macOS; potřebuje placený Apple Developer účet a ios.appleTeamId
# v app.json (nebo LIBRITER_APPLE_TEAM). method: app-store-connect (TestFlight
# a App Store) nebo release-testing (ad hoc na registrovaná zařízení).
#
# Release IPA → libriter-mobile/dist/Libriter.ipa
mobile-ipa method="app-store-connect": mobile-version (mobile-ios-release method "export")

# Nahrává účtem přihlášeným v Xcode. Aplikace musí v App Store Connect už
# existovat.
#
# Sestaví release a nahraje ho do App Store Connect (TestFlight)
mobile-testflight: mobile-version (mobile-ios-release "app-store-connect" "upload")

# Jedno číslo buildu i verze pro obě platformy; na Linuxu jen Android. Jedno
# volání just, aby se `npm ci` (mobile-deps) pustilo jen jednou.
#
# Zvýší verzi a sestaví AAB i IPA → libriter-mobile/dist/
mobile-release: mobile-version
    #!/usr/bin/env bash
    set -euo pipefail
    recipes=(mobile-android-release bundleRelease bundle/release/app-release.aab libriter.aab)
    if [ "$(uname)" = Darwin ]; then
        recipes+=(mobile-ios-release app-store-connect export)
    else
        echo "iOS jde sestavit jen na macOS – vzniká jen AAB."
    fi
    {{just_executable()}} "${recipes[@]}"
    ls -lh {{mobile_dir}}/dist/libriter.aab {{mobile_dir}}/dist/Libriter.ipa 2>/dev/null || true  # na Linuxu IPA chybí

# Zeptá se na zvýšení verze (patch/minor/major) a zvýší číslo buildu v app.json
[private]
mobile-version:
    node _scripts/app-version.mjs {{mobile_dir}}

# destination: export (IPA do dist/) nebo upload (rovnou do App Store Connect)
[working-directory('libriter-mobile')]
[private]
mobile-ios-release method destination: mobile-deps
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
      <key>destination</key><string>{{destination}}</string>
    </dict></plist>
    EOF
    # export do $tmp: vedle IPA tam xcodebuild dává i logy a plisty
    xcodebuild -exportArchive -archivePath "$archive" -exportPath "$tmp" \
        -exportOptionsPlist "$opts" -allowProvisioningUpdates
    if [ -f "$tmp/Libriter.ipa" ]; then
        # Cloudový distribuční certifikát (bez lokálního klíče) zapíše jméno
        # s diakritikou do podpisu v jiném tvaru Unicode, než má certifikát,
        # a App Store Connect pak IPA odmítne („Invalid Signature“, 90035).
        # Lokálně to poznat jde: podpis nesplní vlastní designated requirement
        # (kontroluje se jen s -v).
        unzip -q "$tmp/Libriter.ipa" -d "$tmp/check"
        if ! codesign --verify --deep --strict -v "$tmp/check/Payload/Libriter.app" 2>"$tmp/codesign.log"; then
            cat "$tmp/codesign.log" >&2
            echo "✗ IPA má neplatný podpis a App Store Connect ho odmítne. Nejčastější příčina: chybí lokální" >&2
            echo "  certifikát Apple Distribution – Xcode → Settings → Accounts → Manage Certificates → + → Apple Distribution." >&2
            rm -rf "$tmp"
            exit 1
        fi
        mv "$tmp/Libriter.ipa" "$out/"
    fi
    rm -rf "$tmp"
    if [ "{{destination}}" = upload ]; then
        echo "✓ Nahráno do App Store Connect – v TestFlightu se build objeví po zpracování (obvykle 10–30 min)."
    else
        echo "→ {{mobile_dir}}/dist/Libriter.ipa"
    fi

# ---- Sceniter TV (Apple TV + Google TV) ----
#
# TV aplikace běží na forku react-native-tvos, proto není npm workspace:
# má vlastní node_modules a package-lock.json v sceniter-tv/ (dvě verze
# React Native v jednom stromu by se pobily). Sdílené balíčky bere Metro
# přímo ze zdrojů (viz sceniter-tv/metro.config.js).

# Závislosti TV aplikace (vlastní lockfile v sceniter-tv/). Pod rootem
# (sudo) by node_modules, android/ i Gradle cache patřily rootovi.
[working-directory('sceniter-tv')]
[private]
tv-deps:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ "$(id -u)" = 0 ]; then
        echo "TV aplikaci sestavuj jako běžný uživatel, ne přes sudo." >&2
        exit 1
    fi
    npm ci

# Metro bundler TV aplikace (pro už nainstalovaný debug build)
[working-directory('sceniter-tv')]
tv-start: tv-deps
    EXPO_TV=1 npx expo start

# Na připojené Android TV / Google TV (adb connect <ip>) nebo v emulátoru Android TV
[working-directory('sceniter-tv')]
tv-android: tv-deps
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -r "$HOME/.config/libriter/android-env.sh" ]; then
        . "$HOME/.config/libriter/android-env.sh"
    fi
    EXPO_TV=1 npx expo run:android --device

# V simulátoru Apple TV (jen macOS + Xcode s tvOS SDK)
[working-directory('sceniter-tv')]
tv-ios: tv-deps
    #!/usr/bin/env bash
    set -euo pipefail
    EXPO_TV=1 npx expo prebuild --platform ios
    echo "export NODE_BINARY=$(command -v node)" > ios/.xcode.env.local
    (cd ios && pod install)
    EXPO_TV=1 npx expo run:ios

# Release APK k ruční instalaci (sideload na Google TV) → sceniter-tv/dist/sceniter-tv.apk
tv-apk: (tv-android-release "assembleRelease" "apk/release/app-release.apk" "sceniter-tv.apk")

# Release AAB pro Google Play (Android TV) → sceniter-tv/dist/sceniter-tv.aab
tv-aab: tv-version (tv-android-release "bundleRelease" "bundle/release/app-release.aab" "sceniter-tv.aab")

# Jen macOS + Xcode s tvOS SDK. method: app-store-connect (TestFlight a App
# Store) nebo release-testing (ad hoc na registrovaná zařízení).
#
# Release IPA pro Apple TV → sceniter-tv/dist/Sceniter.ipa
tv-ipa method="app-store-connect": tv-version (tv-ios-release method "export")

# Jedno číslo buildu i verze pro obě platformy; na Linuxu jen Android.
# bump: patch|minor|major|build|none, bez něj se zeptá.
#
# Zvýší verzi a sestaví AAB i IPA → sceniter-tv/dist/
tv-release bump="":
    #!/usr/bin/env bash
    set -euo pipefail
    LIBRITER_BUMP="{{bump}}" {{just_executable()}} tv-version
    recipes=(tv-android-release bundleRelease bundle/release/app-release.aab sceniter-tv.aab)
    if [ "$(uname)" = Darwin ]; then
        recipes+=(tv-ios-release app-store-connect export)
    else
        echo "Apple TV jde sestavit jen na macOS – vzniká jen AAB."
    fi
    {{just_executable()}} "${recipes[@]}"
    ls -lh {{tv_dir}}/dist/sceniter-tv.aab {{tv_dir}}/dist/Sceniter.ipa 2>/dev/null || true  # na Linuxu IPA chybí

# Zeptá se na zvýšení verze (patch/minor/major) a zvýší číslo buildu v app.json
[private]
tv-version:
    node _scripts/app-version.mjs {{tv_dir}}

# Podepisuje stejným upload klíčem jako mobil (just mobile-keystore).
[working-directory('sceniter-tv')]
[private]
tv-android-release task artifact out: tv-deps
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -r "$HOME/.config/libriter/android-env.sh" ]; then
        . "$HOME/.config/libriter/android-env.sh"
    fi
    if ! grep -qs '^LIBRITER_UPLOAD_STORE_FILE=' "$HOME/.gradle/gradle.properties"; then
        echo "⚠ Release klíč nenastavený (just mobile-keystore) – podepisuji debug klíčem, do Google Play to neprojde." >&2
    fi
    EXPO_TV=1 npx expo prebuild --platform android
    # Televize a TV boxy jsou ARM; x86 je jen pro emulátor a build zdvojnásobí.
    (cd android && ./gradlew {{task}} -PreactNativeArchitectures=armeabi-v7a,arm64-v8a)
    mkdir -p dist
    cp "android/app/build/outputs/{{artifact}}" "dist/{{out}}"
    echo "→ {{tv_dir}}/dist/{{out}}"

# Sestaví release pro Apple TV a nahraje ho do App Store Connect (TestFlight)
tv-testflight: tv-version (tv-ios-release "app-store-connect" "upload")

# destination: export (IPA do dist/) nebo upload (rovnou do App Store Connect)
[working-directory('sceniter-tv')]
[private]
tv-ios-release method destination: tv-deps
    #!/usr/bin/env bash
    set -euo pipefail
    team="${LIBRITER_APPLE_TEAM:-$(node -p "require('./app.json').expo.ios.appleTeamId ?? ''")}"
    if [ -z "$team" ]; then
        echo "Chybí Apple Team ID: doplň ios.appleTeamId do app.json nebo nastav LIBRITER_APPLE_TEAM." >&2
        exit 1
    fi
    EXPO_TV=1 npx expo prebuild --platform ios
    echo "export NODE_BINARY=$(command -v node)" > ios/.xcode.env.local
    (cd ios && pod install)
    out="$PWD/dist"
    archive="$PWD/ios/build/Sceniter.xcarchive"
    mkdir -p "$out"
    xcodebuild -workspace ios/Sceniter.xcworkspace -scheme Sceniter \
        -configuration Release -destination 'generic/platform=tvOS' \
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
      <key>destination</key><string>{{destination}}</string>
    </dict></plist>
    EOF
    # export do $tmp: vedle IPA tam xcodebuild dává i logy a plisty
    xcodebuild -exportArchive -archivePath "$archive" -exportPath "$tmp" \
        -exportOptionsPlist "$opts" -allowProvisioningUpdates
    if [ -f "$tmp/Sceniter.ipa" ]; then
        # Stejná kontrola podpisu jako u mobilu (viz mobile-ios-release).
        unzip -q "$tmp/Sceniter.ipa" -d "$tmp/check"
        if ! codesign --verify --deep --strict -v "$tmp/check/Payload/Sceniter.app" 2>"$tmp/codesign.log"; then
            cat "$tmp/codesign.log" >&2
            echo "✗ IPA má neplatný podpis a App Store Connect ho odmítne. Nejčastější příčina: chybí lokální" >&2
            echo "  certifikát Apple Distribution – Xcode → Settings → Accounts → Manage Certificates → + → Apple Distribution." >&2
            rm -rf "$tmp"
            exit 1
        fi
        mv "$tmp/Sceniter.ipa" "$out/"
    fi
    rm -rf "$tmp"
    if [ "{{destination}}" = upload ]; then
        echo "✓ Nahráno do App Store Connect – v TestFlightu se build objeví po zpracování (obvykle 10–30 min)."
    else
        echo "→ {{tv_dir}}/dist/Sceniter.ipa"
    fi

# Ikony, bannery a Top Shelf z sceniter-tv/assets/brand/ (po změně loga)
[working-directory('sceniter-tv')]
tv-brand-assets:
    sh brand-assets.sh

# Nový release: zvýší verzi (patch/minor/major), sestaví .deb a vystaví ho na GitHubu
deploy:
    bash _scripts/release.sh

# ---- Docker ----
# Server, na kterém běží kontejnery (compose v /opt/libriter a /opt/libriter-demo).
docker_host := env("MEDIATER_DOCKER_HOST", "172.24.0.46")
# Kolik starších verzí image nechat na serveru pro návrat zpět (kromě těch,
# na kterých běží demo a produkce).
docker_keep_images := env("MEDIATER_KEEP_IMAGES", "2")

# Docker image mediater:<verze> + mediater:latest (verze: MEDIATER_VERSION, jinak `git describe`)
docker-build:
    #!/usr/bin/env bash
    set -euo pipefail
    version="${MEDIATER_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
    # --provenance=false: bez atestace má opakovaný build z cache stejné ID image,
    # takže docker-deploy pozná, že image na serveru už je, a kontejner nevyměňuje.
    docker build --provenance=false --build-arg MEDIATER_VERSION="$version" \
        -t "mediater:$version" -t mediater:latest .
    echo "Image: mediater:$version"

# Image se na server posílá přes `docker save | ssh docker load` (bez registry)
# a jen když tam ještě není – `just docker-deploy prod` po `demo` tak už nic
# nepřenáší. Kontejner se vymění přes `docker compose up -d` a čeká se, až
# bude healthy. Nakonec se na serveru smažou staré image (viz docker_keep_images).
#
# Sestaví image, nahraje ho na server a restartuje instanci: just docker-deploy demo | prod
docker-deploy target:
    #!/usr/bin/env bash
    set -euo pipefail
    case "{{target}}" in
      demo) dir=/opt/libriter-demo; name=libriter-demo ;;
      prod) dir=/opt/libriter;      name=libriter ;;
      *) echo "Neznámý cíl '{{target}}' – použij: just docker-deploy demo | prod" >&2; exit 1 ;;
    esac
    host="{{docker_host}}"
    version="${MEDIATER_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"

    if [[ "{{target}}" == prod ]]; then
      read -rp "Nasadit $version na PRODUKCI ($host:$dir)? [y/N] " confirm
      [[ "$confirm" =~ ^[yYaA]$ ]] || { echo "Zrušeno."; exit 1; }
    fi

    MEDIATER_VERSION="$version" {{just_executable()}} docker-build

    local_id="$(docker images -q --no-trunc "mediater:$version")"
    remote_ids="$(ssh "$host" "sudo docker images -q --no-trunc mediater:$version; echo /; sudo docker images -q --no-trunc mediater:latest")"
    if [[ "$remote_ids" == "$local_id"$'\n/\n'"$local_id" ]]; then
      echo "Image mediater:$version už na $host je – přenos se přeskakuje."
    else
      echo "Přenáším mediater:$version na $host…"
      docker save "mediater:$version" mediater:latest | gzip -1 | ssh "$host" 'sudo docker load'
    fi

    echo "Restartuji $name…"
    ssh "$host" "sudo sh -s -- $dir $name {{docker_keep_images}}" <<'EOF'
    set -eu
    dir=$1 name=$2 keep=$3
    cd "$dir"
    docker compose up -d
    i=0
    while [ "$(docker inspect -f '{{{{.State.Health.Status}}' "$name")" != healthy ]; do
      i=$((i + 1))
      if [ $i -gt 45 ]; then
        echo "Kontejner $name není ani po 90 s healthy:" >&2
        docker logs --tail 40 "$name" >&2
        exit 1
      fi
      sleep 2
    done
    docker logs "$name" 2>&1 | grep -m1 'verze=' || true
    docker compose ps --format '{{{{.Name}}  {{{{.Image}}  {{{{.Status}}'

    # Úklid starých image. Zůstává: co používá některý kontejner (demo i
    # produkce), aktuální latest a $keep nejnovějších dalších verzí pro
    # návrat zpět. `docker images` řadí od nejnovějšího.
    used="$(docker ps -aq | xargs -r docker inspect -f '{{{{.Image}}' | sort -u)"
    latest="$(docker images -q --no-trunc mediater:latest)"
    n=0
    docker images --no-trunc --format '{{{{.ID}} {{{{.Tag}}' mediater | while read -r id tag; do
      [ "$tag" = latest ] && continue
      [ "$id" = "$latest" ] && continue
      printf '%s\n' "$used" | grep -qx "$id" && continue
      n=$((n + 1))
      [ $n -le "$keep" ] && continue
      echo "Mažu starý image mediater:$tag"
      docker rmi "mediater:$tag" >/dev/null
    done
    docker image prune -f >/dev/null
    echo "Image na serveru:"
    docker images --format '  {{{{.Repository}}:{{{{.Tag}}  {{{{.CreatedSince}}' mediater
    EOF
    echo "Hotovo: {{target}} běží na $version."

# Nainstaluje vývojové nástroje: Go, Node, Android SDK; na macOS i iOS (Xcode CLT, CocoaPods)
setup:
    bash _scripts/setup.sh

# Zkontroluje, že jsou dostupné nástroje pro backend, web, Android a iOS
doctor:
    bash _scripts/doctor.sh

# Smaže bin/ a obsah dist/
clean:
    rm -rf bin
    find {{dist}} {{sceniter_dist}} -mindepth 1 ! -name .gitkeep -delete
