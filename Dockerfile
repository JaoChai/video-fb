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
    && rm -rf /var/lib/apt/lists/*

# Pre-install the pinned Hyperframes CLI so renders don't download it each run.
RUN npm install -g hyperframes@0.6.70

COPY --from=builder /server /server
COPY migrations/ /migrations/
# Sarabun fonts the composition builder copies into each project.
COPY hyperframes-poc/poc-video/assets/fonts/ /app/assets/fonts/

ENV PORT=8080
# Tell Puppeteer (used by Hyperframes) to use the system Chromium, not download one.
ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium
ENV PUPPETEER_SKIP_DOWNLOAD=true
ENV HYPERFRAMES_FONTS_DIR=/app/assets/fonts
# HYPERFRAMES_ENABLED stays unset (off) — flip to "true" in Railway when ready.

EXPOSE 8080
CMD ["/server"]
