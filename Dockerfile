# syntax=docker/dockerfile:1
# =============================================================================
#  Mediater (Libriter + Sceniter) – Docker image
#
#  docker build --build-arg MEDIATER_VERSION=$(git describe --tags --always) \
#      -t mediater:$(git describe --tags --always) .
#
#  Image obsahuje binárku /app/mediater (s vkompilovanými weby), pluginy
#  metadat v /app/plugins a ffmpeg. Pracovní adresář je /app, takže relativní
#  cesty z .env (DB_PATH=./data/libriter.db, AUDIO_ROOT=./data/...) míří do
#  /app/data – tam se připojuje volume s daty.
# =============================================================================

ARG MEDIATER_VERSION=dev

# -----------------------------------------------------------------------------
#  1. Weby (Libriter + Sceniter) → server/internal/{libriter,sceniter}/web/dist
# -----------------------------------------------------------------------------
FROM node:24-bookworm-slim AS web
WORKDIR /src

# Nejdřív jen manifesty – vrstva s node_modules se znovu použije, dokud se
# nezmění závislosti.
COPY package.json package-lock.json ./
COPY _scripts/restore-expo-sqlite-sources.mjs _scripts/patch-track-player-android.mjs _scripts/
COPY core-shared/package.json core-shared/
COPY libriter-shared/package.json libriter-shared/
COPY sceniter-shared/package.json sceniter-shared/
COPY core-web/package.json core-web/
COPY libriter-frontend/package.json libriter-frontend/
COPY sceniter-frontend/package.json sceniter-frontend/
COPY libriter-mobile/package.json libriter-mobile/

# Jen workspaces webů (a sdílené balíčky, na kterých závisí) – mobilní
# aplikace s Expo se do image neinstaluje. Postinstall skripty bez expo
# balíčků tiše skončí.
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund \
        --include-workspace-root \
        -w libriter-frontend -w sceniter-frontend

COPY _image/ _image/
COPY core-shared/ core-shared/
COPY libriter-shared/ libriter-shared/
COPY sceniter-shared/ sceniter-shared/
COPY core-web/ core-web/
COPY libriter-frontend/ libriter-frontend/
COPY sceniter-frontend/ sceniter-frontend/
RUN mkdir -p server/internal/libriter/web/dist server/internal/sceniter/web/dist \
 && npm run build -w libriter-frontend \
 && npm run build -w sceniter-frontend

# -----------------------------------------------------------------------------
#  2. Backend + pluginy metadat
# -----------------------------------------------------------------------------
FROM golang:1.26-bookworm AS go
ARG MEDIATER_VERSION
WORKDIR /src/server

COPY server/go.mod server/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY server/ ./
COPY --from=web /src/server/internal/libriter/web/dist internal/libriter/web/dist
COPY --from=web /src/server/internal/sceniter/web/dist internal/sceniter/web/dist

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    set -eu; \
    ldflags="-s -w -X mediater/internal/core/version.Version=${MEDIATER_VERSION}"; \
    mkdir -p /out/plugins; \
    go build -trimpath -ldflags "$ldflags" -o /out/mediater ./cmd/server; \
    for dir in cmd/plugins/*/; do \
        name=$(basename "$dir"); \
        go build -trimpath -ldflags "$ldflags" -o "/out/plugins/mediater-meta-$name" "./$dir"; \
    done; \
    # Host odmítne plugin zapisovatelný pro skupinu – vždy 755.
    chmod 755 /out/mediater /out/plugins/*

# -----------------------------------------------------------------------------
#  3. Runtime
# -----------------------------------------------------------------------------
FROM ubuntu:26.04
ARG MEDIATER_VERSION

RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        ffmpeg ca-certificates tzdata curl \
 && rm -rf /var/lib/apt/lists/*

# Stejné uid/gid jako dosavadní systémový uživatel `libriter` (1026:users),
# aby data v bind-mountu zůstala čitelná bez chown.
ARG APP_UID=1026
ARG APP_GID=100

WORKDIR /app
COPY --from=go /out/mediater /app/mediater
COPY --from=go /out/plugins/ /app/plugins/
COPY --chmod=755 docker/entrypoint.sh /usr/local/bin/entrypoint.sh

# Pracovní adresář převodů videa (ffmpeg/HLS). Potřebuje místo zhruba na
# velikost přehrávaného filmu, proto ne tmpfs v RAM – v compose se sem
# připojuje volume. Při startu ho aplikace sama čistí.
RUN mkdir -p /var/cache/mediater/transcode \
 && chown -R ${APP_UID}:${APP_GID} /var/cache/mediater

ENV TZ=Europe/Prague \
    SERVER_PORT=8080 \
    SERVER_ENV=production \
    TRANSCODE_ROOT=/var/cache/mediater/transcode

LABEL org.opencontainers.image.title="mediater" \
      org.opencontainers.image.description="Libriter + Sceniter" \
      org.opencontainers.image.version="${MEDIATER_VERSION}"

USER ${APP_UID}:${APP_GID}
VOLUME ["/app/data"]
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD curl -fsS "http://127.0.0.1:${SERVER_PORT}/health" >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["serve"]
