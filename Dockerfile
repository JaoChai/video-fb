# ── Go binary builder ────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server cmd/server/main.go

# ── Runtime: Debian (not Alpine) because headless Chromium needs glibc ─────────
# Carries Node 22 + Chromium + FFmpeg so the Hyperframes CLI can render the
# 9:16 multi-scene videos (Approach A — single image bundling the toolchain).
FROM node:22-bookworm-slim

# Freeze the Debian package repo to a 2026-07-06 snapshot. `apt install chromium`
# is version-unpinned, so a rebuild on 2026-07-07 pulled a NEWER bookworm chromium
# that fails to launch under headless Puppeteer on Railway (renders had worked for
# weeks on the prior image). Pinning apt to the snapshot from BEFORE that break
# restores the exact chromium that rendered fine AND makes future rebuilds
# reproducible. Snapshot repos carry an old Valid-Until, so disable that check.
RUN rm -f /etc/apt/sources.list.d/debian.sources \
 && printf '%s\n' \
    'deb http://snapshot.debian.org/archive/debian/20260706T000000Z/ bookworm main' \
    'deb http://snapshot.debian.org/archive/debian/20260706T000000Z/ bookworm-updates main' \
    'deb http://snapshot.debian.org/archive/debian-security/20260706T000000Z/ bookworm-security main' \
    > /etc/apt/sources.list \
 && apt-get -o Acquire::Check-Valid-Until=false update \
 && apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    tzdata \
    chromium \
    fonts-thai-tlwg \
    # Headless-Chromium shared libs. --no-install-recommends skips chromium's
    # recommended deps, so the ones Puppeteer needs to launch are listed
    # explicitly — otherwise the render fails at runtime (silent fallback).
    libnss3 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libxkbcommon0 \
    libxcomposite1 libxdamage1 libxfixes3 libxrandr2 libgbm1 libasound2 \
    libpangocairo-1.0-0 libcairo2 libxshmfence1 \
 && rm -rf /var/lib/apt/lists/*

# Warm the npx cache with the pinned Hyperframes CLI instead of a global install.
# `npm install -g hyperframes` lays down the bin but NOT the core runtime manifest
# (core/dist/hyperframe.manifest.json), so renders fail with "Missing manifest …"
# and silently fall back to a static FFmpeg image. The npx-cached package is
# complete (manifest included), and the Go renderer prefers `npx hyperframes@<ver>`
# when no global binary is on PATH — so this fixes the manifest AND stays offline
# at render time. Keep this version in sync with hyperframesVersion in
# internal/producer/hyperframes.go (0.6.70).
RUN npx --yes hyperframes@0.6.70 --version

COPY --from=builder /server /server
COPY migrations/ /migrations/
# Sarabun Thai fonts the composition builder copies into each render project.
COPY internal/producer/assets/fonts/ /app/assets/fonts/

ENV PORT=8080
# Tell Puppeteer (used by Hyperframes) to use the system Chromium, not download one.
ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium
ENV PUPPETEER_SKIP_DOWNLOAD=true
# Absolute path main.go's EnableHyperframes passes to the composition builder.
ENV FONTS_DIR=/app/assets/fonts

EXPOSE 8080
CMD ["/server"]
