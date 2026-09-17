#!/usr/bin/env bash
# =============================================================================
#  Libriter – instalace vývojového prostředí
#
#  Spuštění: just setup        (nebo bash _scripts/setup.sh)
#
#  Nainstaluje nástroje pro backend (Go, ffprobe), web (Node) a mobilní vývoj:
#    - Android všude: JDK 17, Android command-line tools, platform-tools,
#      build-tools, watchman, udev pravidla pro telefon (Linux)
#    - iOS jen na macOS: Xcode CLT, CocoaPods, kontrola Xcode
#
#  Podporované systémy: macOS (Homebrew), Ubuntu/Debian (apt), Fedora (dnf).
#  Skript je idempotentní – co už je, přeskočí. Stav ověří `just doctor`.
#
#  Proměnné: ANDROID_API, ANDROID_BUILD_TOOLS, CMDLINE_TOOLS_BUILD (viz lib.sh),
#            LIBRITER_SKIP_IOS=1 přeskočí iOS část na macOS.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib.sh"
detect_os

info "Systém: $OS_PRETTY ($ARCH), správce balíčků: ${PKG:-žádný}"
[[ -z "$PKG" ]] && die "Neznámá distribuce. Podporováno: macOS, Ubuntu/Debian, Fedora."

ENV_DIR="$HOME/.config/libriter"
ENV_FILE="$ENV_DIR/android-env.sh"
ANDROID_HOME="${ANDROID_HOME:-$(android_home_default)}"
export ANDROID_HOME

# -----------------------------------------------------------------------------
#  Instalace balíčků podle správce
# -----------------------------------------------------------------------------
pkg_install() {
  case "$PKG" in
    apt) $SUDO env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "$@" ;;
    dnf) $SUDO dnf install -y "$@" ;;
    brew) brew install "$@" ;;
  esac
}

pkg_update_once() {
  [[ -n "${_PKG_UPDATED:-}" ]] && return
  _PKG_UPDATED=1
  case "$PKG" in
    apt) info "apt-get update"; $SUDO apt-get update -qq ;;
    brew) info "brew update"; brew update --quiet ;;
  esac
}

# -----------------------------------------------------------------------------
#  0. Homebrew (macOS)
# -----------------------------------------------------------------------------
if [[ "$OS" == "macos" ]] && ! have brew; then
  info "Instaluji Homebrew"
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  if [[ -x /opt/homebrew/bin/brew ]]; then eval "$(/opt/homebrew/bin/brew shellenv)"; fi
  if [[ -x /usr/local/bin/brew ]]; then eval "$(/usr/local/bin/brew shellenv)"; fi
fi

# -----------------------------------------------------------------------------
#  1. Základ: git, curl, unzip, just
# -----------------------------------------------------------------------------
info "Základní nástroje"
base=()
have git   || base+=(git)
have curl  || base+=(curl)
have unzip || base+=(unzip)
have just  || base+=(just)
if ((${#base[@]})); then
  pkg_update_once
  pkg_install "${base[@]}"
else
  ok "git, curl, unzip, just"
fi

# -----------------------------------------------------------------------------
#  2. Backend: Go a ffprobe
# -----------------------------------------------------------------------------
info "Backend (Go ≥ $GO_MIN, ffprobe)"
if have go && version_ge "$(go_version)" "$GO_MIN"; then
  ok "go $(go_version)"
else
  pkg_update_once
  case "$PKG" in
    apt)  pkg_install golang-go ;;
    dnf)  pkg_install golang ;;
    brew) pkg_install go ;;
  esac
fi
if have ffprobe; then
  ok "ffprobe"
else
  pkg_update_once
  case "$PKG" in
    apt)  pkg_install ffmpeg ;;
    dnf)  pkg_install ffmpeg-free ;;   # ffprobe bez RPM Fusion
    brew) pkg_install ffmpeg ;;
  esac
fi

# -----------------------------------------------------------------------------
#  3. Node ≥ 22 (web + Expo)
# -----------------------------------------------------------------------------
info "Node.js ≥ $NODE_MIN"
if have node && (( $(node_major) >= NODE_MIN )); then
  ok "node $(node -v)"
elif [[ "$OS" == "macos" ]]; then
  pkg_install node
else
  # Distribuční node bývá starý; nvm dá aktuální LTS bez roota.
  export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
  if [[ ! -s "$NVM_DIR/nvm.sh" ]]; then
    info "Instaluji nvm do $NVM_DIR"
    curl -fsSL https://raw.githubusercontent.com/nvm-sh/nvm/master/install.sh | bash
  fi
  # shellcheck disable=SC1091
  . "$NVM_DIR/nvm.sh"
  nvm install --lts
  nvm alias default 'lts/*'
fi

# -----------------------------------------------------------------------------
#  4. Android: JDK 17
# -----------------------------------------------------------------------------
info "Android: JDK $JAVA_WANT"
if JAVA17_HOME="$(find_java17_home)"; then
  ok "JDK 17 v $JAVA17_HOME"
else
  pkg_update_once
  case "$PKG" in
    apt)  pkg_install openjdk-17-jdk-headless ;;
    dnf)  pkg_install java-17-openjdk-devel ;;
    brew) brew install --cask zulu@17 ;;
  esac
  JAVA17_HOME="$(find_java17_home)" || die "JDK 17 se nepodařilo najít po instalaci."
fi

# -----------------------------------------------------------------------------
#  5. Android: command-line tools + SDK balíčky
# -----------------------------------------------------------------------------
info "Android SDK v $ANDROID_HOME"
CMDLINE_DIR="$ANDROID_HOME/cmdline-tools/latest"
if [[ -x "$CMDLINE_DIR/bin/sdkmanager" ]]; then
  ok "cmdline-tools"
else
  mkdir -p "$ANDROID_HOME/cmdline-tools"
  case "$OS" in
    macos) [[ "$ARCH" == "arm64" ]] && plat="mac_arm64" || plat="mac_x86_64" ;;
    *)     plat="linux" ;;
  esac
  zip_url="https://dl.google.com/android/repository/commandlinetools-${plat}-${CMDLINE_TOOLS_BUILD}_latest.zip"
  tmp="$(mktemp -d)"
  info "Stahuji $zip_url"
  curl -fL --progress-bar -o "$tmp/cmdline-tools.zip" "$zip_url"
  unzip -q "$tmp/cmdline-tools.zip" -d "$tmp"
  rm -rf "$CMDLINE_DIR"
  mv "$tmp/cmdline-tools" "$CMDLINE_DIR"
  rm -rf "$tmp"
  ok "cmdline-tools nainstalovány"
fi

export JAVA_HOME="$JAVA17_HOME"
export PATH="$CMDLINE_DIR/bin:$ANDROID_HOME/platform-tools:$JAVA_HOME/bin:$PATH"

info "Přijímám licence SDK a instaluji balíčky (může trvat několik minut)"
yes | sdkmanager --sdk_root="$ANDROID_HOME" --licenses >/dev/null || true
sdkmanager --sdk_root="$ANDROID_HOME" \
  "platform-tools" \
  "platforms;android-${ANDROID_API}" \
  "build-tools;${ANDROID_BUILD_TOOLS}" \
  "cmdline-tools;latest" \
  || warn "Některý SDK balíček se nepodařilo nainstalovat. Zkontroluj ANDROID_API / ANDROID_BUILD_TOOLS v _scripts/lib.sh."

# -----------------------------------------------------------------------------
#  6. Android: watchman, přístup k telefonu přes USB (Linux)
# -----------------------------------------------------------------------------
info "Android: watchman a USB ladění"
if have watchman; then
  ok "watchman"
else
  pkg_update_once
  pkg_install watchman || warn "watchman není k dispozici; Metro funguje i bez něj, jen pomaleji sleduje soubory."
fi
case "$PKG" in
  apt)
    pkg_install android-sdk-platform-tools-common || true   # udev pravidla + skupina plugdev
    if [[ -z "$SUDO" ]]; then
      warn "Běžíš jako root; pro USB ladění přidej svého uživatele do skupiny plugdev."
    elif ! id -nG | grep -qw plugdev; then
      $SUDO usermod -aG plugdev "$USER" && warn "Přidán do skupiny plugdev, projeví se po odhlášení."
    fi
    ;;
  dnf)
    pkg_install android-tools || true   # obsahuje udev pravidla pro adb
    ;;
esac

# -----------------------------------------------------------------------------
#  7. Prostředí: ~/.config/libriter/android-env.sh + načtení v shellu
# -----------------------------------------------------------------------------
info "Zapisuji $ENV_FILE"
mkdir -p "$ENV_DIR"
cat > "$ENV_FILE" <<EOF
# Generováno _scripts/setup.sh (Libriter). Načítá se z shell rc souboru.
export ANDROID_HOME="$ANDROID_HOME"
export ANDROID_SDK_ROOT="\$ANDROID_HOME"
export JAVA_HOME="$JAVA17_HOME"
export PATH="\$ANDROID_HOME/cmdline-tools/latest/bin:\$ANDROID_HOME/platform-tools:\$ANDROID_HOME/emulator:\$JAVA_HOME/bin:\$PATH"
EOF

marker="# libriter: Android SDK"
line="[ -r \"\$HOME/.config/libriter/android-env.sh\" ] && . \"\$HOME/.config/libriter/android-env.sh\"  $marker"
rc_files=()
[[ "$OS" == "macos" ]] && rc_files+=("$HOME/.zprofile")
[[ -f "$HOME/.zshrc" ]]  && rc_files+=("$HOME/.zshrc")
[[ -f "$HOME/.bashrc" ]] && rc_files+=("$HOME/.bashrc")
((${#rc_files[@]})) || rc_files+=("$HOME/.profile")
for rc in "${rc_files[@]}"; do
  if ! grep -qsF "$marker" "$rc"; then
    printf '\n%s\n' "$line" >> "$rc"
    ok "přidáno načtení do $rc"
  fi
done

# -----------------------------------------------------------------------------
#  8. iOS (jen macOS)
# -----------------------------------------------------------------------------
if [[ "$OS" == "macos" && -z "${LIBRITER_SKIP_IOS:-}" ]]; then
  info "iOS: Xcode, Command Line Tools, CocoaPods"
  if ! xcode-select -p >/dev/null 2>&1; then
    warn "Spouštím instalaci Xcode Command Line Tools (dialog macOS); po dokončení spusť setup znovu."
    xcode-select --install || true
  else
    ok "Command Line Tools: $(xcode-select -p)"
  fi
  if [[ -d /Applications/Xcode.app ]]; then
    ok "Xcode $(xcodebuild -version 2>/dev/null | head -n1 | awk '{print $2}')"
    if ! xcodebuild -checkFirstLaunchStatus >/dev/null 2>&1; then
      info "Přijímám licenci Xcode a dokončuji první spuštění (sudo)"
      $SUDO xcodebuild -license accept || true
      $SUDO xcodebuild -runFirstLaunch || true
    fi
    if ! xcrun simctl list runtimes 2>/dev/null | grep -q "iOS"; then
      info "Stahuji iOS platformu pro Xcode (simulátor + build na zařízení; velké stahování)"
      xcodebuild -downloadPlatform iOS || warn "Stažení iOS platformy selhalo, spusť ručně: xcodebuild -downloadPlatform iOS"
    else
      ok "iOS runtime pro simulátor"
    fi
  else
    fail "Xcode není v /Applications. Nainstaluj ho z App Store (nebo 'mas install 497799835') a spusť setup znovu."
  fi
  if have pod; then ok "CocoaPods $(pod --version)"; else pkg_install cocoapods; fi
fi

# -----------------------------------------------------------------------------
#  Hotovo
# -----------------------------------------------------------------------------
echo
info "Hotovo. Otevři nový terminál (nebo: . $ENV_FILE) a spusť: just doctor"
[[ "$OS" != "macos" ]] && warn "iOS vývoj vyžaduje macOS s Xcode; na tomto systému je dostupný jen Android."
exit 0
