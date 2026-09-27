#!/usr/bin/env bash
# =============================================================================
#  Libriter – nový release
#
#  Spuštění: just deploy        (nebo bash _scripts/release.sh)
#
#  1. najde poslední verzi (tag vX.Y.Z; bez tagu v0.0.0),
#  2. zeptá se, jestli jde o patch, minor, nebo major,
#  3. sestaví web i backend s novou verzí vloženou do binárky,
#  4. zabalí dist/libriter_X.Y.Z_amd64.deb (balíček: packaging/deb/),
#  5. vytvoří a pushne tag a GitHub release se seznamem změn a .deb přílohou.
#
#  Předpoklady: gh (přihlášené přes `gh auth login`), dpkg-deb, čistý strom
#  na main, shodný s origin/main.
#
#  Proměnné: DRY_RUN=1 jen sestaví balíček – bez tagu, pushe a releasu
#            (a bez kontroly větve a čistého stromu).
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib.sh"
cd "$REPO_DIR"
umask 022   # práva adresářů v balíčku (jinak 775 z uživatelské umask)

DRY_RUN="${DRY_RUN:-0}"
ARCH=amd64
PKG_SRC="$REPO_DIR/packaging/deb"
DIST="$REPO_DIR/dist"

# -----------------------------------------------------------------------------
#  1. Předpoklady
# -----------------------------------------------------------------------------
info "Kontrola předpokladů"
have dpkg-deb || die "Chybí dpkg-deb (balíček dpkg)."
have just || die "Chybí just – just setup."
if [[ "$DRY_RUN" != 1 ]]; then
  have gh || die "Chybí gh (GitHub CLI) – just setup."
  gh auth status >/dev/null 2>&1 || die "gh není přihlášené – gh auth login."

  branch="$(git rev-parse --abbrev-ref HEAD)"
  [[ "$branch" == main ]] || die "Release jde jen z větve main (teď: $branch)."
  [[ -z "$(git status --porcelain)" ]] || die "Pracovní strom není čistý – nejdřív commit."

  git fetch --quiet --tags origin main
  [[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] \
    || die "main se liší od origin/main – nejdřív push/pull."
fi
ok "vše připraveno"

# -----------------------------------------------------------------------------
#  2. Poslední verze a volba nové
# -----------------------------------------------------------------------------
prev="$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n1)"
current="${prev:-v0.0.0}"
IFS=. read -r major minor patch <<<"${current#v}"

next_patch="v$major.$minor.$((patch + 1))"
next_minor="v$major.$((minor + 1)).0"
next_major="v$((major + 1)).0.0"

echo
info "Poslední verze: ${prev:-žádná (začíná se od v0.0.0)}"
echo "  1) patch  → $next_patch"
echo "  2) minor  → $next_minor"
echo "  3) major  → $next_major"
read -rp "Jaký release? [1-3] " choice
case "$choice" in
  1|patch) version="$next_patch" ;;
  2|minor) version="$next_minor" ;;
  3|major) version="$next_major" ;;
  *) die "Neplatná volba: $choice" ;;
esac

git rev-parse -q --verify "refs/tags/$version" >/dev/null \
  && die "Tag $version už existuje."

# -----------------------------------------------------------------------------
#  3. Seznam změn
# -----------------------------------------------------------------------------
range="${prev:+$prev..}HEAD"
changes="$(git log --no-merges --pretty='- %s (%h)' "$range")"
[[ -n "$changes" ]] || die "Od $prev se nic nezměnilo."

mkdir -p "$DIST"
notes="$DIST/release-notes-$version.md"
{
  echo "## Změny"
  echo
  echo "$changes"
  echo
  echo "## Instalace"
  echo
  echo '```bash'
  echo "sudo apt install ./libriter_${version#v}_${ARCH}.deb"
  echo '```'
} >"$notes"

echo
info "Změny v $version:"
echo "$changes"
echo
read -rp "Pokračovat s releasem $version? [y/N] " confirm
[[ "$confirm" =~ ^[yYaA]$ ]] || die "Zrušeno."

# -----------------------------------------------------------------------------
#  4. Build
# -----------------------------------------------------------------------------
info "Build $version"
LIBRITER_VERSION="$version" GOOS=linux GOARCH="$ARCH" just build

# -----------------------------------------------------------------------------
#  5. Balíček .deb
# -----------------------------------------------------------------------------
deb_version="${version#v}"
deb_name="libriter_${deb_version}_${ARCH}"
root="$DIST/deb/$deb_name"
deb="$DIST/$deb_name.deb"

info "Balím $deb_name.deb"
rm -rf "$root"
install -d "$root/DEBIAN" "$root/usr/bin" "$root/lib/systemd/system" "$root/etc/libriter"

sed -e "s/@VERSION@/$deb_version/" -e "s/@ARCH@/$ARCH/" \
  "$PKG_SRC/control.in" >"$root/DEBIAN/control"
install -m 644 "$PKG_SRC/conffiles" "$root/DEBIAN/conffiles"
install -m 755 "$PKG_SRC/postinst" "$PKG_SRC/prerm" "$PKG_SRC/postrm" "$root/DEBIAN/"

install -m 755 bin/libriter "$root/usr/bin/libriter"
install -m 644 "$PKG_SRC/libriter.service" "$root/lib/systemd/system/libriter.service"
install -m 640 "$PKG_SRC/libriter.env" "$root/etc/libriter/libriter.env"

dpkg-deb --root-owner-group --build "$root" "$deb" >/dev/null
ok "$deb"

if [[ "$DRY_RUN" == 1 ]]; then
  warn "DRY_RUN=1 – tag ani GitHub release se nevytváří."
  exit 0
fi

# -----------------------------------------------------------------------------
#  6. Tag a GitHub release
# -----------------------------------------------------------------------------
info "Tag $version"
git tag -a "$version" -m "Libriter $version"
git push origin "$version"

info "GitHub release $version"
if ! gh release create "$version" "$deb" --title "Libriter $version" --notes-file "$notes"; then
  fail "Release se nepodařilo vytvořit; tag $version už je na GitHubu. Dokončení ručně:"
  echo "  gh release create $version $deb --title \"Libriter $version\" --notes-file $notes"
  exit 1
fi
ok "hotovo: $(gh release view "$version" --json url -q .url)"
