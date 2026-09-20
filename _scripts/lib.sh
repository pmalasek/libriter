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
# JDK: Gradle/AGP v Expo projektu běží spolehlivě na 17–24. Novější JDK (Fedora 44
# už v repozitářích nabízí jen 25+) Gradle odmítne s "Unsupported class file major
# version", proto má horní mez. JAVA_PREFERRED se instaluje, když nic vhodného není.
JAVA_WANT=17
JAVA_MAX=24
JAVA_PREFERRED=21
# JDK stažené setupem (bez sudo, mimo balíčkovač distribuce)
LIBRITER_JDK_DIR="${LIBRITER_JDK_DIR:-$HOME/.local/share/libriter/jdk}"

# --- pomocné ----------------------------------------------------------------
have() { command -v "$1" >/dev/null 2>&1; }

# porovnání verzí "a >= b" pro tečkované číslice
version_ge() { [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]; }

node_major() { node -v 2>/dev/null | sed -E 's/^v([0-9]+).*/\1/'; }
go_version()  { go version 2>/dev/null | sed -E 's/.*go([0-9]+\.[0-9]+).*/\1/'; }
java_major()  { java -version 2>&1 | head -n1 | sed -E 's/.*"([0-9]+)(\.[0-9]+)*.*/\1/'; }

# Major verze JDK v adresáři ($1); selže, když tam JDK není
jdk_major_at() {
  local d="$1" v=""
  [[ -x "$d/bin/javac" ]] || return 1
  [[ -r "$d/release" ]] && v="$(sed -nE 's/^JAVA_VERSION="?([0-9._]+).*/\1/p' "$d/release" | head -n1)"
  [[ -z "$v" ]] && v="$("$d/bin/javac" -version 2>&1 | sed -nE 's/^javac ([0-9._]+).*/\1/p' | head -n1)"
  [[ -z "$v" ]] && return 1
  [[ "$v" == 1.* ]] && v="${v#1.}"   # 1.8.0 -> 8
  printf '%s\n' "${v%%.*}"
}

# JAVA_HOME s JDK v rozsahu JAVA_WANT–JAVA_MAX. Preferuje JAVA_PREFERRED,
# jinak nejnovější podporované. Prohledá i JDK stažené setupem.
find_java_home() {
  local d major best="" best_major=0 candidates=()
  [[ -n "${JAVA_HOME:-}" ]] && candidates+=("$JAVA_HOME")
  candidates+=("$LIBRITER_JDK_DIR"/*)
  if [[ "${OS:-}" == "macos" ]]; then
    candidates+=(/Library/Java/JavaVirtualMachines/*/Contents/Home "$HOME/Library/Java/JavaVirtualMachines"/*/Contents/Home)
  else
    candidates+=(/usr/lib/jvm/* /usr/java/*)
  fi
  for d in "${candidates[@]}"; do
    [[ -d "$d" ]] || continue
    major="$(jdk_major_at "$d")" || continue
    (( major >= JAVA_WANT && major <= JAVA_MAX )) || continue
    (( major == JAVA_PREFERRED )) && { echo "$d"; return 0; }
    (( major > best_major )) && { best_major=$major; best="$d"; }
  done
  [[ -n "$best" ]] && { echo "$best"; return 0; }
  return 1
}

# Stáhne Eclipse Temurin JDK do $LIBRITER_JDK_DIR (bez sudo). $1 = major verze.
# Fallback pro distribuce, které vhodné JDK v repozitářích nemají (Fedora 44).
install_temurin_jdk() {
  local ver="${1:-$JAVA_PREFERRED}" os_tag arch_tag url tmp dir target
  case "${OS:-}" in macos) os_tag="mac" ;; *) os_tag="linux" ;; esac
  case "${ARCH:-}" in arm64) arch_tag="aarch64" ;; *) arch_tag="x64" ;; esac
  url="https://api.adoptium.net/v3/binary/latest/${ver}/ga/${os_tag}/${arch_tag}/jdk/hotspot/normal/eclipse"
  tmp="$(mktemp -d)"
  info "Stahuji Temurin JDK $ver ($os_tag/$arch_tag) do $LIBRITER_JDK_DIR"
  if ! curl -fL --progress-bar -o "$tmp/jdk.tar.gz" "$url" || ! tar -xzf "$tmp/jdk.tar.gz" -C "$tmp"; then
    rm -rf "$tmp"; return 1
  fi
  dir="$(find "$tmp" -mindepth 1 -maxdepth 1 -type d | head -n1)"
  [[ -n "$dir" ]] || { rm -rf "$tmp"; return 1; }
  [[ -d "$dir/Contents/Home" ]] && dir="$dir/Contents/Home"
  target="$LIBRITER_JDK_DIR/temurin-$ver"
  mkdir -p "$LIBRITER_JDK_DIR"
  rm -rf "$target"
  mv "$dir" "$target"
  rm -rf "$tmp"
  [[ -x "$target/bin/javac" ]] || return 1
  ok "Temurin JDK $ver v $target"
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
