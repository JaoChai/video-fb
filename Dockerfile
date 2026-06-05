# Go binary builder.
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server cmd/server/main.go

# Runtime: Debian-based (not Alpine) because headless Chromium needs glibc.
# Carries Node 22 + Chromium + FFmpeg so the Hyperframes CLI can render videos.
FROM node:22-bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    tzdata \
    chromium \
    fonts-thai-tlwg \
    # Headless-Chromium shared libs. --no-install-recommends skips chromium's
    # recommended deps, so list the ones Puppeteer needs to launch explicitly —
    # otherwise the render fails at runtime and silently falls back to FFmpeg.
    libnss3 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libxkbcommon0 \
    libxcomposite1 libxdamage1 libxfixes3 libxrandr2 libgbm1 libasound2 \
    libpangocairo-1.0-0 libcairo2 libxshmfence1 \
    && rm -rf /var/lib/apt/lists/*

# Warm the npx cache with the pinned Hyperframes CLI instead of a global install.
# A global `npm install -g hyperframes` lays down the bin but NOT the core runtime
# manifest (/usr/local/lib/core/dist/hyperframe.manifest.json), so renders fail with
# "Missing manifest ... Build core runtime artifacts before rendering" and silently
# fall back to a static FFmpeg image. The npx-cached package is complete (manifest
# included), and the Go renderer falls back to `npx hyperframes@<ver>` when no global
# binary is on PATH — so this both fixes the manifest and stays offline at render time.
RUN npx --yes hyperframes@0.6.70 --version

COPY --from=builder /server /server
COPY migrations/ /migrations/
# Sarabun fonts the composition builder copies into each project.
COPY hyperframes-poc/poc-video/assets/fonts/ /app/assets/fonts/
# Mascot poses the composition builder copies into each project (intro/outro/per-scene).
COPY assets/mascot/ /app/assets/mascot/

ENV PORT=8080
# Tell Puppeteer (used by Hyperframes) to use the system Chromium, not download one.
ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium
ENV PUPPETEER_SKIP_DOWNLOAD=true
ENV HYPERFRAMES_FONTS_DIR=/app/assets/fonts
# HYPERFRAMES_ENABLED stays unset (off) — flip to "true" in Railway when ready.

EXPOSE 8080
CMD ["/server"]
