#!/usr/bin/env bash
# Společné funkce pro setup.sh a doctor.sh: detekce systému, barvy, verze.
# Načítá se přes `source`, samostatně nic nedělá.

# --- barvy (jen když jde o terminál) ---------------------------------------
if [[ -t 1 ]]; then
  C_OK=$'\033[32m'; C_WARN=$'\033[33m'; C_ERR=$'\033[31m'; C_DIM=$'\033[2m'; C_BOLD=$'\033[1m'; C_OFF=$'\033[0m'
else
  C_OK=""; C_WARN=""; C_ERR=""; C_DIM=""; C_BOLD=""; C_OFF=""
fi

info()  { printf '%s→%s %s\n' "$C_BOLD" "$C_OFF" "$*"; }
ok()    { printf '  %s✓%s %s\n' "$C_OK" "$C_OFF" "$*"; }
warn()  { printf '  %s!%s %s\n' "$C_WARN" "$C_OFF" "$*"; }
fail()  { printf '  %s✗%s %s\n' "$C_ERR" "$C_OFF" "$*"; }
die()   { printf '%s✗ %s%s\n' "$C_ERR" "$*" "$C_OFF" >&2; exit 1; }

# --- systém -----------------------------------------------------------------
# OS      = macos | ubuntu | fedora | linux (neznámá distribuce)
# PKG     = brew | apt | dnf | ""
# ARCH    = arm64 | x86_64
detect_os() {
  ARCH="$(uname -m)"
  [[ "$ARCH" == "aarch64" ]] && ARCH="arm64"
  case "$(uname -s)" in
    Darwin) OS="macos"; PKG="brew" ;;
    Linux)
      OS="linux"; PKG=""
      if [[ -r /etc/os-release ]]; then
        # shellcheck disable=SC1091
        . /etc/os-release
        case "${ID:-} ${ID_LIKE:-}" in
          *ubuntu*|*debian*) OS="ubuntu"; PKG="apt" ;;
          *fedora*|*rhel*)   OS="fedora"; PKG="dnf" ;;
        esac
        OS_PRETTY="${PRETTY_NAME:-Linux}"
      fi
      ;;
    *) die "Nepodporovaný systém: $(uname -s)" ;;
  esac
  [[ "$OS" == "macos" ]] && OS_PRETTY="macOS $(sw_vers -productVersion 2>/dev/null)"
  export OS PKG ARCH OS_PRETTY
}

# sudo jen když nejsme root
SUDO=""
[[ "$(id -u)" -ne 0 ]] && SUDO="sudo"

# --- Android SDK cesty ------------------------------------------------------
android_home_default() {
  if [[ "$OS" == "macos" ]]; then
    echo "$HOME/Library/Android/sdk"
  else
    echo "$HOME/Android/Sdk"
  fi
}

# Verze SDK balíčků. Gradle si při buildu Expo appky stáhne přesné verze,
# které projekt vyžaduje (licence musí být přijaté); tohle je jen rozumný základ.
ANDROID_API="${ANDROID_API:-36}"
ANDROID_BUILD_TOOLS="${ANDROID_BUILD_TOOLS:-36.0.0}"
# Build id z https://developer.android.com/studio#command-line-tools-only
CMDLINE_TOOLS_BUILD="${CMDLINE_TOOLS_BUILD:-15859902}"

# Minimální verze
NODE_MIN=22
GO_MIN="1.22"
JAVA_WANT=17

# --- pomocné ----------------------------------------------------------------
have() { command -v "$1" >/dev/null 2>&1; }

# porovnání verzí "a >= b" pro tečkované číslice
version_ge() { [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]; }

node_major() { node -v 2>/dev/null | sed -E 's/^v([0-9]+).*/\1/'; }
go_version()  { go version 2>/dev/null | sed -E 's/.*go([0-9]+\.[0-9]+).*/\1/'; }
java_major()  { java -version 2>&1 | head -n1 | sed -E 's/.*"([0-9]+)(\.[0-9]+)*.*/\1/'; }

# JAVA_HOME pro JDK 17 podle systému
find_java17_home() {
  if [[ "$OS" == "macos" ]]; then
    /usr/libexec/java_home -v 17 2>/dev/null && return 0
  fi
  local d
  for d in /usr/lib/jvm/java-17-openjdk-* /usr/lib/jvm/java-17-openjdk /usr/lib/jvm/java-17 /usr/lib/jvm/temurin-17*; do
    [[ -x "$d/bin/javac" ]] && { echo "$d"; return 0; }
  done
  return 1
}

# Načte prostředí pro Android, pokud ho setup zapsal
load_android_env() {
  local env_file="$HOME/.config/libriter/android-env.sh"
  # shellcheck disable=SC1090
  [[ -r "$env_file" ]] && . "$env_file"
  : "${ANDROID_HOME:=$(android_home_default)}"
  export ANDROID_HOME
  export PATH="$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools:$PATH"
}
