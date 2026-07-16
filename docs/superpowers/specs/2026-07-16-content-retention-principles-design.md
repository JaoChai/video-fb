# Content Retention Principles — Design Spec

**Date:** 2026-07-16
**Status:** Approved (brainstorming), pending implementation plan
**Author:** JaoChai + Claude
**Related:** migration 055 (TikTok engagement), [[project_tiktok_engagement_fix]], content_brain_v2 (052/053/054)

## 1. Problem & Goal

TikTok/Shorts clips get weak algorithmic distribution. Deep research (2026-07-16,
16 verified claims from primary sources: TikTok Newsroom/Transparency, YouTube
Help; secondary: Buffer, Hootsuite, Socialinsider) established that **viewer
retention behavior — hook + watch-to-completion — is the single strongest ranking
lever on both platforms**, NOT engagement/comments (that premise was an
overstatement; comments are a positive-but-secondary predicted-interaction input).

**Goal:** Encode the highest-leverage, source-backed retention principles into the
content-generation agents' prompts so clips are built for the first-3-second hook
and watch-through by construction.

### Credible principles this spec acts on (verified, high-confidence)

- **P1 — First ~3s is the hook window.** Low avg watch time (e.g. 3s on a 60s clip)
  is a direct diagnostic of a missing hook. (Buffer/Hootsuite, citing TikTok data)
- **P2 — Completion/watch-through is the strongest signal.** Write so viewers stay
  to the end. (TikTok Newsroom, primary)
- **P3 — Platform-endorsed hook tactics:** open with a question the video answers,
  a countdown/timer, or flash the end result up front. (Hootsuite, citing TikTok
  Creator Academy)
- **P4 — Skips count against ranking; every scene must earn the next.** (TikTok
  Transparency, primary)

### Explicitly OUT of scope (already satisfied or refuted)

- **Thai caption burn-in is ALREADY DONE** — `internal/producer/captions.go` builds
  synced Thai captions from ground-truth VoiceText. No work needed.
- **Comment-driving CTA — already shipped** in migration 055. Not re-touched.
- **Do NOT encode refuted numbers:** "sub-30s always best", "hook 2s = +19%
  retention", "80% watch muted", "75% retention = 3x reach", fixed completion-rate
  benchmark tables. All were adversarially refuted (0-3) and must not become rules.

## 2. Scope

Encode **3 retention principles** into 4 agent prompts, all via a single migration
(056), plus a critic text-level gate. **No Go code changes. No JSON output-shape
changes** (hard constraint — see Risks).

1. Hook spoken + visual within first ~3 seconds.
2. Open loop / write-for-completion.
3. Tight, fast-cut scenes with a mid-clip re-hook (anti-skip).

## 3. Approach (chosen: A — Prompt-rule + critic-check)

Alternatives considered:
- **B — add a structured `hook_line` output field** enforced by producer as scene 0.
  Rejected: changing the script agent's JSON output shape is exactly what caused the
  052 regression (narration went blank on every clip). High regression risk.
- **C — dedicated "hook" agent.** Rejected: overkill; new pipeline step + per-clip
  LLM cost for a rule that fits in existing prompts.

Approach A matches how 052–055 shipped (prompt edits via migration), keeps blast
radius to prompt text, and uses the existing critic agent to compensate for A's only
weakness (non-deterministic hook placement).

## 4. Detailed Design — changes per agent

All changes are `UPDATE agent_configs` in `migrations/056_*.sql`, using `REPLACE()`
on an exact existing anchor substring (idempotent; no-op once applied), mirroring 055.

### 4.1 `script` agent — `prompt_template`
Add to the JSON output rules (near the `answer_script`/`voice_script` bullets):
- The FIRST sentence of `voice_script` (and `answer_script`) must be a **hook that
  lands within ~3 seconds** (≤ ~15 Thai words), chosen from one of three
  platform-endorsed patterns: (a) a question the clip immediately answers,
  (b) a shock number / rejected-status, (c) flash the end result/payoff first.
- **Forbid** opening with: restating the question, a greeting ("สวัสดีครับ"), or a
  long preamble.
- Include an **open loop** early (promise the payoff/method comes later) to pull
  viewers to the end.
- Clip arc: hook (scene 1, ~3s) → raise the stakes/problem → concrete step → payoff + CTA.

### 4.2 `question` agent — `skills`
- Prefer questions with a **clear single-clip payoff and a curiosity gap** — not
  open-ended questions with no resolvable answer in one clip.

### 4.3 `scene` / analyzer agent — `skills`
- Scene 1 = the full hook.
- Every scene **short, fast-cut, one idea per scene**.
- Insert a **mid-clip re-hook / curiosity bump** to defend the middle (the common
  skip point). Avoid long, information-dense scenes.

### 4.4 `critic` agent — `skills`
Critic edits text (scenes/metadata); it cannot watch the video, so the gate is a
text-level heuristic:
- If `scene[0]` VoiceText/on-screen text does NOT open with a hook pattern
  (question / shock-number / result-first) → **rewrite the hook** (fix, do not block).
- If there is no open loop → add one.
- Keep the existing 053 guard: do not push hooks into clickbait that overstates the
  truth or teaches policy evasion.
- Do NOT touch the comment CTA (owned by 055).

## 5. Verification (regression-guarded)

The 052 regression (prompt output change → blank narration on every clip) is the
top risk. Mitigations + checks:

1. **Pre-apply:** run the migration's `REPLACE()` as a read-only `SELECT` on prod
   and confirm each anchor substring matches (the 055 pattern) before applying.
2. **Post-deploy, produce 1 clip** and confirm:
   - `scene[0]` opens with a hook pattern
   - an open loop is present
   - **narration is NOT empty** (direct 052-regression check)
   - the clip renders and publishes normally
3. **Monitor 5–7 days:** avg watch time / completion trend and count of 0-view
   TikTok clips (using the now-failed-filtered analytics from 055).

## 6. Rollout

- Single global migration 056 (prompt changes are global by nature, like 055).
- No new feature flag; `content_brain_v2_enabled` still gates the v2 pipeline.
- **Rollback:** revert migration 056 (each UPDATE is a REPLACE of the new text back
  to the old; a paired down-migration or a manual revert restores prior prompts).

## 7. Risks

- **R1 — Blank-narration regression (052 class).** Mitigation: no JSON field
  added/renamed; only content rules added inside existing `answer_script`/
  `voice_script`/skills text. Verification step 2 explicitly checks narration.
- **R2 — Over-aggressive hooks → clickbait/policy risk.** Mitigation: keep the 053
  critic guard against overstating truth / teaching evasion.
- **R3 — Critic over-rewrites good hooks (false positives).** Mitigation: gate is
  "fix if clearly missing", not "block"; critic already reconciles conservatively.

## 8. Success Criteria

- 1 freshly-produced clip demonstrably opens with a ≤3s hook + has an open loop, with
  intact narration and normal publish.
- Over 5–7 days: no increase in blank/failed clips; directional improvement in avg
  watch time / reduction in 0-view TikTok clips.
