#!/usr/bin/env bash
# =============================================================================
#  Libriter – kontrola vývojového prostředí
#
#  Spuštění: just doctor        (nebo bash _scripts/doctor.sh)
#
#  Vypíše, co je a co chybí pro backend, web, Android a (na macOS) iOS.
#  Končí kódem 1, pokud chybí něco povinného. Chybějící věci doplní `just setup`.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib.sh"
detect_os
load_android_env

missing=0
# check <povinné:1|0> <název> <podmínka OK:0|1> <hodnota při OK> <rada při chybě>
check() {
  local required="$1" name="$2" status="$3" value="$4" hint="$5"
  if [[ "$status" -eq 0 ]]; then
    printf '  %s✓%s %-28s %s%s%s\n' "$C_OK" "$C_OFF" "$name" "$C_DIM" "$value" "$C_OFF"
  elif [[ "$required" -eq 1 ]]; then
    printf '  %s✗%s %-28s %s\n' "$C_ERR" "$C_OFF" "$name" "$hint"
    missing=1
  else
    printf '  %s-%s %-28s %s%s%s\n' "$C_WARN" "$C_OFF" "$name" "$C_DIM" "$hint" "$C_OFF"
  fi
}

echo "${C_BOLD}Libriter doctor${C_OFF}  ·  $OS_PRETTY ($ARCH)"

# -----------------------------------------------------------------------------
echo; echo "${C_BOLD}Základ${C_OFF}"
have git;  check 1 "git"  $? "$(git --version 2>/dev/null | awk '{print $3}')" "just setup"
have just; check 1 "just" $? "$(just --version 2>/dev/null | awk '{print $2}')" "just setup"
have curl; check 1 "curl" $? "" "just setup"
have unzip; check 1 "unzip" $? "" "just setup"

# -----------------------------------------------------------------------------
echo; echo "${C_BOLD}Backend (Go)${C_OFF}"
if have go && version_ge "$(go_version)" "$GO_MIN"; then s=0; else s=1; fi
check 1 "go >= $GO_MIN" $s "$(go version 2>/dev/null | awk '{print $3}')" "just setup (nebo https://go.dev/dl)"
have ffprobe; check 1 "ffprobe" $? "$(ffprobe -version 2>/dev/null | head -n1 | awk '{print $3}')" "just setup (balíček ffmpeg)"

# -----------------------------------------------------------------------------
echo; echo "${C_BOLD}Web (Node)${C_OFF}"
if have node && (( $(node_major) >= NODE_MIN )); then s=0; else s=1; fi
check 1 "node >= $NODE_MIN" $s "$(node -v 2>/dev/null)" "just setup (nvm install --lts)"
have npm; check 1 "npm" $? "$(npm -v 2>/dev/null)" "součást Node"
if [[ -d "$REPO_DIR/libriter-frontend/node_modules" ]]; then s=0; else s=1; fi
check 0 "frontend node_modules" $s "nainstalováno" "cd libriter-frontend && npm ci"

# -----------------------------------------------------------------------------
echo; echo "${C_BOLD}Android${C_OFF}"
if [[ -n "${JAVA_HOME:-}" && -x "$JAVA_HOME/bin/java" ]]; then
  jv="$("$JAVA_HOME/bin/java" -version 2>&1 | head -n1 | sed -E 's/.*"([0-9]+)(\.[0-9]+)*.*/\1/')"
  [[ "$jv" == "$JAVA_WANT" ]] && s=0 || s=1
  check 1 "JDK $JAVA_WANT (JAVA_HOME)" $s "$JAVA_HOME" "JAVA_HOME míří na JDK $jv, Gradle chce 17; just setup"
elif have java; then
  jv="$(java_major)"; [[ "$jv" == "$JAVA_WANT" ]] && s=0 || s=1
  check 1 "JDK $JAVA_WANT" $s "java $jv (JAVA_HOME nenastaveno)" "java $jv v PATH, JAVA_HOME nenastaveno; just setup"
else
  check 1 "JDK $JAVA_WANT" 1 "" "just setup"
fi
if [[ -d "$ANDROID_HOME" ]]; then s=0; else s=1; fi
check 1 "ANDROID_HOME" $s "$ANDROID_HOME" "adresář $ANDROID_HOME neexistuje; just setup"
have sdkmanager; check 1 "sdkmanager (cmdline-tools)" $? "$(sdkmanager --version 2>/dev/null | head -n1)" "just setup"
have adb; check 1 "adb (platform-tools)" $? "$(adb version 2>/dev/null | head -n1 | awk '{print $5}')" "just setup"
plats="$(ls -d "$ANDROID_HOME"/platforms/android-* 2>/dev/null | sed 's#.*/android-##' | sort -n | tr '\n' ' ')"
[[ -n "$plats" ]] && s=0 || s=1
check 1 "platforms" $s "android-$(echo "$plats" | tr ' ' ',' | sed 's/,$//;s/,/, android-/g')" "sdkmanager \"platforms;android-$ANDROID_API\""
bts="$(ls "$ANDROID_HOME"/build-tools 2>/dev/null | sort -V | tr '\n' ' ')"
[[ -n "$bts" ]] && s=0 || s=1
check 1 "build-tools" $s "${bts% }" "sdkmanager \"build-tools;$ANDROID_BUILD_TOOLS\""
if [[ -f "$ANDROID_HOME/licenses/android-sdk-license" ]]; then s=0; else s=1; fi
check 1 "licence SDK přijaté" $s "" "yes | sdkmanager --licenses"
[[ -d "$ANDROID_HOME/ndk" ]] && s=0 || s=1
check 0 "NDK" $s "$(ls "$ANDROID_HOME"/ndk 2>/dev/null | sort -V | tail -n1)" "volitelné, Gradle si stáhne verzi podle projektu"
have watchman; check 0 "watchman" $? "$(watchman --version 2>/dev/null)" "volitelné, zrychluje Metro"
if [[ "$OS" != "macos" ]]; then
  [[ -r /dev/kvm ]] && s=0 || s=1
  check 0 "KVM (emulátor)" $s "/dev/kvm" "volitelné, jen pro Android emulátor"
fi
if have adb; then
  devs="$(adb devices 2>/dev/null | awk 'NR>1 && $2=="device"{print $1}' | tr '\n' ' ')"
  unauth="$(adb devices 2>/dev/null | awk 'NR>1 && $2=="unauthorized"{print $1}' | tr '\n' ' ')"
  if [[ -n "$devs" ]]; then check 0 "připojený telefon" 0 "${devs% }" ""
  elif [[ -n "$unauth" ]]; then check 0 "připojený telefon" 1 "" "zařízení ${unauth% }: potvrď USB ladění na telefonu"
  else check 0 "připojený telefon" 1 "" "žádné zařízení (adb devices)"; fi
fi

# -----------------------------------------------------------------------------
if [[ "$OS" == "macos" ]]; then
  echo; echo "${C_BOLD}iOS${C_OFF}"
  xcode-select -p >/dev/null 2>&1; check 1 "Command Line Tools" $? "$(xcode-select -p 2>/dev/null)" "xcode-select --install"
  [[ -d /Applications/Xcode.app ]] && s=0 || s=1
  check 1 "Xcode" $s "$(xcodebuild -version 2>/dev/null | head -n1 | awk '{print $2}')" "App Store → Xcode"
  if [[ -d /Applications/Xcode.app ]]; then
    xcodebuild -checkFirstLaunchStatus >/dev/null 2>&1; check 1 "licence Xcode přijatá" $? "" "sudo xcodebuild -license accept && sudo xcodebuild -runFirstLaunch"
    xcrun simctl list runtimes 2>/dev/null | grep -q "iOS"; check 1 "iOS runtime (simulátor)" $? "$(xcrun simctl list runtimes 2>/dev/null | grep -m1 'iOS' | awk '{print $1, $2}')" "xcodebuild -downloadPlatform iOS"
    devs="$(xcrun xctrace list devices 2>/dev/null | grep -vi simulator | grep -E '\([0-9A-F-]{20,}\)' | grep -v "$(scutil --get ComputerName 2>/dev/null)" | head -n3 | tr '\n' ';')"
    [[ -n "$devs" ]] && s=0 || s=1
    check 0 "připojený iPhone/iPad" $s "$devs" "žádné zařízení (kabel + důvěřovat počítači)"
  fi
  have pod; check 1 "CocoaPods" $? "$(pod --version 2>/dev/null)" "brew install cocoapods"
else
  echo; echo "${C_BOLD}iOS${C_OFF}"
  check 0 "iOS toolchain" 1 "" "jen na macOS s Xcode"
fi

# -----------------------------------------------------------------------------
echo; echo "${C_BOLD}Mobilní projekt${C_OFF}"
if [[ -f "$REPO_DIR/libriter-mobile/package.json" ]]; then
  check 0 "libriter-mobile/" 0 "existuje" ""
  if [[ -d "$REPO_DIR/libriter-mobile/node_modules" ]]; then
    check 0 "expo-doctor" 0 "spusť: cd libriter-mobile && npx expo-doctor" ""
  else
    check 0 "mobile node_modules" 1 "" "npm ci v kořeni repa"
  fi
else
  check 0 "libriter-mobile/" 1 "" "zatím neexistuje (viz docs/mobile-app-plan.md, fáze 4)"
fi

echo
if [[ "$missing" -eq 0 ]]; then
  printf '%s✓ Vše povinné je k dispozici.%s\n' "$C_OK" "$C_OFF"
  exit 0
else
  printf '%s✗ Něco povinného chybí. Spusť: just setup%s\n' "$C_ERR" "$C_OFF"
  exit 1
fi
