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

# Freeze the Debian package repo to a 2026-07-04 snapshot. `apt install chromium`
# is version-unpinned, so a rebuild on 2026-07-07 pulled a NEWER bookworm chromium
# that fails to launch under headless Puppeteer on Railway (renders had worked for
# weeks on the prior image). The broken chromium 150.0.7871.46-1~deb12u1 entered
# bookworm-security on 2026-07-05, so the snapshot must be pinned to 2026-07-04 or
# earlier to get chromium 149.0.7827.196-1~deb12u1 (the previous security build,
# accepted 2026-06-26). The exact last-good version is unknown (it came from an
# old Docker layer cache), so if 149 also fails to launch, move this date earlier.
# Snapshot repos carry an old Valid-Until, so disable that check.
RUN rm -f /etc/apt/sources.list.d/debian.sources \
 && printf '%s\n' \
    'deb http://snapshot.debian.org/archive/debian/20260704T000000Z/ bookworm main' \
    'deb http://snapshot.debian.org/archive/debian/20260704T000000Z/ bookworm-updates main' \
    'deb http://snapshot.debian.org/archive/debian-security/20260704T000000Z/ bookworm-security main' \
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

# hyperframes ไม่เคยอ่าน PUPPETEER_EXECUTABLE_PATH เลยสักเวอร์ชัน (grep บันเดิลทั้ง 0.6.70
# และ 0.7.90: ศูนย์ครั้งทั้งคู่) — 0.6.70 รอดมาตลอดเพราะมีทางสำรอง whichBinary("chromium")
# ค้นหา /usr/bin/chromium เอง แต่ 0.7.90 ตัดทางสำรองนั้นทิ้ง เปลี่ยนมาไล่ลำดับ
# PRODUCER_HEADLESS_SHELL_PATH → HYPERFRAMES_BROWSER_PATH → แคชในโฮม → ดาวน์โหลดเอง ·
# อิมเมจนี้ไม่มี unzip การดาวน์โหลดจึงตาย ("Extraction failed: no zip archiver is
# available") และเรนเดอร์ล้มใน 9 วินาที · ตัวแปรนี้จึงเป็นตัวเดียวที่กันไม่ให้ hyperframes
# ออกเน็ตตอนเรนเดอร์
ENV HYPERFRAMES_BROWSER_PATH=/usr/bin/chromium

# Warm the npx cache with the pinned Hyperframes CLI instead of a global install, then run 3
# build-time checks against hyperframes/chromium's real behavior — ตายตอน build ดีกว่าไปตาย
# กลางการเรนเดอร์บน prod
#
# ทำไมไม่ install -g: `npm install -g hyperframes` ลง bin แต่ไม่ลง core runtime manifest
# (core/dist/hyperframe.manifest.json) เรนเดอร์พังด้วย "Missing manifest …" แล้วตกไปทำ static
# FFmpeg image เงียบ ๆ · แพ็กเกจที่ warm ผ่าน npx สมบูรณ์กว่า และ Go renderer เองก็เรียก
# `npx hyperframes@<ver>` เป็นทางหลักอยู่แล้วเมื่อไม่เจอ binary บน PATH
#
# ทำไมไม่พิมพ์เวอร์ชันซ้ำ: HFV อ่านจาก internal/producer/hyperframes.go จุดเดียวที่ตัดสินเวอร์ชัน
# จริง พิมพ์ซ้ำในไฟล์นี้จะเกิด drift ได้ — Go renderer ขอ npx ด้วยเลขของมันเอง มิสแมตช์แล้วพลาด
# แคชที่ warm ไว้ ไปดึงจาก npm registry กลางการเรนเดอร์บน prod แทน · เวอร์ชันอ่านไม่ได้/มั่วทำให้
# build พังตรงนี้แทน
#
# 3 ด่าน (แต่ละด่านจับสิ่งที่ด่านก่อนจับไม่ได้):
#  1) `npx hyperframes@$HFV --version` — แคชอุ่นสำเร็จ + เวอร์ชันอ่านได้จริง
#  2) `"$HYPERFRAMES_BROWSER_PATH" --version` — ไบนารีที่ชี้ไว้รันได้จริง + shared-lib ครบ
#     (--no-install-recommends ด้านบนต้องไล่ลง lib เอง พลาดตัวเดียวก็ล้มตอน runtime) — แต่ไม่ได้
#     พิสูจน์ว่า hyperframes จะ *เลือก* ไบนารีตัวนี้จริง
#  3) เทียบ `npx hyperframes browser path` กับ HYPERFRAMES_BROWSER_PATH — คำสั่งนี้ใช้ตัวจัดการ
#     เบราว์เซอร์ของ CLI (findBrowser) ซึ่ง grep บันเดิล 0.7.90 พบว่าเป็นคนละฟังก์ชันกับตัวที่คุม
#     การเรนเดอร์จริง (resolveHeadlessShellPath) — สอดคล้องกันตอนนี้เพราะทั้งคู่พึ่ง
#     HYPERFRAMES_BROWSER_PATH ตัวเดียว (ไม่ได้ตั้ง PRODUCER_HEADLESS_SHELL_PATH ที่มีสิทธิ์
#     เหนือกว่าฝั่งเรนเดอร์) · ทดสอบแล้วว่ามันไม่ error เมื่อ path ผิด (ตกไปใช้แคช/ค้นระบบเงียบ ๆ
#     แทน) จึงต้องเทียบค่าที่มันตอบเอง ห้ามพึ่ง exit code · ปิด update-check/telemetry กันบรรทัด
#     ปน stdout, tail -1 กันเหนียวอีกชั้น · ด่านนี้จับสิ่งที่ 1-2 จับไม่ได้: ถ้าวันหน้า hyperframes
#     เปลี่ยนชื่อ env var หรือลำดับค้นหาเบราว์เซอร์อีก (แบบที่ 0.6.70→0.7.90 เพิ่งทำ) ค่าที่มันตอบ
#     จะไม่ตรงกับที่เราตั้ง
COPY internal/producer/hyperframes.go /tmp/hf.go
RUN HFV="$(sed -n 's/.*hyperframesVersion = "\([^"]*\)".*/\1/p' /tmp/hf.go)" \
 && echo "hyperframes version from hyperframes.go: ${HFV:?not found in hyperframes.go}" \
 && npx --yes "hyperframes@$HFV" --version \
 && "$HYPERFRAMES_BROWSER_PATH" --version \
 && FOUND="$(HYPERFRAMES_NO_UPDATE_CHECK=1 HYPERFRAMES_NO_TELEMETRY=1 npx --yes "hyperframes@$HFV" browser path | tail -1)" \
 && if [ "$FOUND" != "$HYPERFRAMES_BROWSER_PATH" ]; then echo "hyperframes จะใช้ '$FOUND' ไม่ใช่ '$HYPERFRAMES_BROWSER_PATH'"; exit 1; fi \
 && rm /tmp/hf.go

COPY --from=builder /server /server
COPY migrations/ /migrations/
# Sarabun Thai fonts the composition builder copies into each render project.
COPY internal/producer/assets/fonts/ /app/assets/fonts/

ENV PORT=8080
# Absolute path main.go's EnableHyperframes passes to the composition builder.
ENV FONTS_DIR=/app/assets/fonts

# ค่าพวกนี้ **ทับ** ค่าที่ CLI คำนวณเองจาก cgroup · ตั้งไว้ตอนสมมติฐาน ~8GB ยังไม่ได้พิสูจน์
# บนของจริง คอนเทนเนอร์จริงใหญ่กว่านั้นมาก (ตัวเลขวัดจริง ดูคอมเมนต์ renderWorkers ใน
# internal/producer/hyperframes.go) ค่านี้จึงบีบแคชเกินจำเป็น ถอดได้ถ้าอยากได้ความเร็วเพิ่ม
# แต่ยังไม่มีใครวัดผลหลังถอด
ENV PRODUCER_FRAME_DATA_URI_CACHE_BYTES_MB=256
ENV PRODUCER_FRAME_DATA_URI_CACHE_LIMIT=64
# 10 นาทีต่อหนึ่งคำสั่ง CDP: การจับภาพที่อืดแต่ยังเดินอยู่จะได้ไปต่อจนจบ แทนที่จะ
# ตายที่ 5 นาทีแบบเดิม · เพดานรวมยังคุมด้วย HyperframesRenderer.timeout (20 นาที)
ENV PRODUCER_PUPPETEER_PROTOCOL_TIMEOUT_MS=600000

EXPOSE 8080
CMD ["/server"]
