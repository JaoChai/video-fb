# Dockerfile + Railway Deploy (Plan 2b-6) — Runbook

> This is a **deploy runbook**, not a TDD code plan. The code change is one Dockerfile (+ .dockerignore); the rest is an operational sequence the human runs against production. The hard, irreversible, outward-facing step (the prod deploy) is gated behind explicit confirmation.

**Goal:** Ship a Docker image that bundles Node 22 + headless Chromium + FFmpeg + the pinned Hyperframes CLI + Sarabun fonts so the Go server can render the 9:16 multi-scene videos, then deploy it to Railway and verify the **first real end-to-end MP4**.

**Status of the code:** Done on branch `feat/hyperframes-dockerfile` — `Dockerfile` (rewritten) + `.dockerignore` (new). The Dockerfile is a faithful port of the **proven** image from the rolled-back `redesign/hyperframes-video-engine` branch (the one that actually rendered videos), adapted to master's env contract (`FONTS_DIR`, fonts at `internal/producer/assets/fonts`, no mascot).

---

## What the new Dockerfile changes (vs the old alpine+ffmpeg-only image)

| Aspect | Old | New |
|---|---|---|
| Runtime base | `alpine:3.21` | `node:22-bookworm-slim` (Debian — Chromium needs glibc) |
| Browser | none | `chromium` apt pkg + the explicit shared-lib list Puppeteer needs |
| Render CLI | none | `npx --yes hyperframes@0.6.70` cache warmed at build (NOT global install — global misses the core manifest → silent fallback) |
| Thai fonts | none | `fonts-thai-tlwg` (system) + Sarabun TTFs copied to `/app/assets/fonts` |
| Env | `PORT` | `+ PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium`, `PUPPETEER_SKIP_DOWNLOAD=true`, `FONTS_DIR=/app/assets/fonts` |
| FFmpeg | `apk ffmpeg` | `apt ffmpeg` |

**Battle-tested details preserved from the proven image** (do not "simplify" these — each is a hard-won fix): Debian base, the explicit Chromium shared-lib list, the npx-cache-warm-instead-of-global-install, and the `PUPPETEER_*` env. Removing any one reintroduces a silent render failure that falls back to a broken/static output.

---

## ⚠️ PROD-SAFETY — read before deploying

**The pipeline has never rendered an MP4 end-to-end.** All prior verification is `hyperframes lint`/`inspect` (composition is valid) + unit tests (logic is correct) — but a full TTS → image → render → upload run has not happened.

**The scheduler auto-runs.** `cmd/server/main.go` always calls `sched.Start()` (no flag gates it); it loads cron schedules from the DB. So **deploying the new image means the next scheduled `ProduceWeekly` fire renders the new hyperframes pipeline unattended — and the publisher may auto-publish the result to the live YouTube / Zernio channel.** If the first real render is broken (font boxes, scene-freeze, wrong timing), unverified video could go public on the channel that attracts ad-account buyers.

**There is no static fallback in the live path.** Plan 2b-5a routed the orchestrator entirely to `ProduceHyperframes916`; the old static `Produce` is dead code. If the render fails, the clip fails (no degraded image) — which is safe (won't publish garbage) but means a broken deploy produces zero clips until fixed.

**Therefore the deploy must be staged + observed, not fire-and-forget.** See the gated sequence below.

---

## Step 0 — Local image build (verification, no prod impact) ✅ in progress

```bash
docker build -t adsvance-hyperframes:test .
```
Confirms the image builds (apt + Chromium + npx hyperframes resolve). A successful local build de-risks the Railway build. **A local build does NOT prove the render works** — that needs the DB keys + a real run (Step 3). If `docker` or network is unavailable locally, skip to building on Railway, but then watch the Railway build logs closely.

Optional deeper local check (needs a DB URL with kie/openrouter keys in `settings`):
```bash
docker run --rm -e DATABASE_URL=<neon-url> adsvance-hyperframes:test /server -produce 1
# Watch for: question → script → scene → assembly (hyperframes render, no [Browser:PAGEERROR]) → upload → "Clip ready (hyperframes)".
```

---

## Step 1 — Commit the Dockerfile (branch) ✅

```bash
git add Dockerfile .dockerignore
git commit -m "feat(deploy): Docker image with Node+Chromium+FFmpeg+hyperframes for 9:16 render"
```
Then merge to master per the project pattern (the deploy reads from master / the Railway-connected branch).

---

## Step 2 — 🚦 STOP: confirm deploy strategy with the human

Do **not** deploy without deciding these (they are the human's call — they own the live channel):

1. **Pause unattended production first?** Before deploying, pause/disable the active production schedule rows (so the cron doesn't auto-fire the new pipeline + auto-publish). Re-enable only after Step 3 verifies a good MP4. *(Recommended.)* — inspect/disable via the schedules table or the app's schedule UI.
2. **Deploy target** — overwrite the existing Railway backend service (fastest, but the active service flips to hyperframes) vs. a separate service/environment first. Railway keeps prior deployments, so a bad deploy can be rolled back instantly to the previous image.
3. **Who triggers the first clip** — a manual `-produce 1` one-off (observed), not the scheduler.

**Railway deploy (only after the above is agreed):** via the Railway MCP (`mcp__plugin_railway_railway__*`) `list_projects` → `list_services` → `deploy`, or `railway up` from the CLI. Infra IDs are in the `reference_adsvance_infra` memory. **This is the irreversible outward-facing step — the assistant will not run it without explicit go-ahead.**

---

## Step 3 — First real render, observed (the binding proof)

After deploy, with the scheduler still paused:

```bash
# trigger ONE clip via the produce API (or a one-off `-produce 1` run)
curl -X POST <railway-url>/api/v1/orchestrator/produce -H 'Authorization: Bearer <API_KEY>' -H 'Content-Type: application/json' -d '{"count":1}'
```
Then verify:
- Railway logs show: `question → script → scene → assembly → upload → "Clip ready (hyperframes)"`, and **no** `[Browser:PAGEERROR]` / `Failed to download CDN script` / `is not defined` lines (the `scanBrowserIssues` guard logs these on a silent render failure).
- The clip row has `Video916URL` + `ThumbnailURL` set; open the MP4: **9:16, 60–90 s, 6–10 animated scenes, Thai captions (not boxes), brand royal-blue/amber, no scene-freeze.**

If broken: read the logs for the browser-issue lines, check `FONTS_DIR`/Chromium env, and roll back the Railway deployment to the previous image while fixing. **Do not re-enable the schedule until a clip renders correctly.**

---

## Step 4 — Re-enable unattended production

Once Step 3 produces a good MP4, re-enable the production schedule. Monitor the first unattended run.

---

## Rollback

Railway retains previous deployments — redeploy the prior (alpine) image from the Railway dashboard/MCP to revert instantly. The DB/migrations are forward-compatible (2b-1's migration 030 columns are additive), so a rollback of the binary does not require a DB rollback.

---

## Notes / deferred

- This deploys the **2b-5a** pipeline (Q&A topic → multi-scene video). The topic-driven purify + `image`→`imageprompt` + dead-code removal is **Plan 2b-5b** — independent of this deploy.
- `fonts-thai-tlwg` (system) is a safety net; the composition actually uses the copied Sarabun TTFs via the template's relative `assets/fonts/` paths.
- Keep `hyperframes@0.6.70` in the Dockerfile in sync with `hyperframesVersion` in `internal/producer/hyperframes.go`.
