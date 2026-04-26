# Ads Vance Content Factory — Design Spec

## Overview

Automated content pipeline for YouTube channel **@adsvance** that produces 1 short video per day (30-60 seconds) about Facebook Ads account tips/tricks in Thai language. Goal: remove channel owner from production loop **completely** — zero hours per month.

**Output:** 1 clip/day, uploaded at midnight, Thai language only.
**Owner involvement:** 0 hours/month (fully automated).
**Video format:** 1 AI-generated image + AI voiceover narration per clip.

---

## Architecture

```
WEEKLY PRODUCE (fully automated)                    DAILY PUBLISH (fully automated)
┌─────────────────────────────────────────┐    ┌────────────────────────────┐
│ ① Claude: generate 7 topics + scripts   │    │ Automated QC checks        │
│ ② GPT Image 2: 1 infographic per clip   │───>│ YouTube scheduled upload    │
│ ③ ElevenLabs V3: Thai voiceover         │    │ at 00:00 every day         │
│ ④ FFmpeg: assemble video                │    │                            │
└─────────────────────────────────────────┘    └────────────────────────────┘

Owner involvement: NONE — fully hands-off after initial setup.
```

---

## Video Format

Each clip is a single AI-generated infographic image with AI voiceover narration on top.
To keep viewers engaged despite a single image, the video includes motion effects:

```
┌──────────────────────────────────────┐
│                                      │
│   1 AI-generated infographic         │
│   (explains the tip visually)        │
│                                      │
│   + Slow zoom/pan (Ken Burns)        │
│   + Text overlays appear over time   │
│   + Highlight circles/arrows         │
│                                      │
│   + AI voiceover narrating           │
│                                      │
│   Duration: 30-60 seconds            │
│                                      │
└──────────────────────────────────────┘
```

**Why 1 image works for this content:**
- Clips are very short (30-60s) — viewers don't need scene changes
- The VALUE is in the spoken tip/solution — the image supports, not leads
- Infographic format is common and trusted in the Facebook Ads tutorial space
- Ken Burns effect + text overlays prevent static-image boredom

---

## Pipeline Steps

### Step ① Script Generation (Claude API)

**Trigger:** Weekly cron job (every Monday)
**Input:** Topic category rotation + previous topics (dedup)
**Output:** 7 structured scripts in JSON format

Each script contains:

```json
{
  "id": "clip-2026-04-28",
  "title": "แอดถูก reject เพราะ Special Ad Category แก้ยังไง?",
  "youtube_title": "แอดโดน Reject! แก้ปัญหา Special Ad Category ใน 1 นาที {Ads Vance}",
  "youtube_description": "วิธีแก้ปัญหาแอดถูก reject เพราะ Special Ad Category\n\nติดต่อทีมงาน line id : @adsvance\nเข้ากลุ่มเทเลแกรม: https://t.me/adsvancech",
  "youtube_tags": ["facebook ads", "โฆษณาเฟสบุ๊ค", "แอดถูก reject", "special ad category"],
  "duration_target_seconds": 50,
  "voice_script": "[confident] แอดคุณถูก reject เพราะ Special Ad Category ใช่ไหม? แก้ได้ง่ายมาก... เข้า Ads Manager แล้วกดที่แคมเปญที่มีปัญหา— ดูตรง Special Ad Categories... ปัญหาคือคุณเลือก category ผิด หรือไม่ได้เลือกทั้งที่ต้องเลือก สินค้าที่เกี่ยวกับ สินเชื่อ การจ้างงาน ที่อยู่อาศัย หรือประเด็นสังคม ต้องเลือก category ให้ถูก... แก้แล้วกด publish ใหม่ได้เลย! [friendly] ถ้ายังติดปัญหา ทักมาที่ไลน์ @adsvance",
  "image_prompt": "Clean professional infographic in Thai language explaining how to fix Facebook Ad rejection due to Special Ad Category. Show 4 categories with icons: สินเชื่อ (Credit - dollar icon), การจ้างงาน (Employment - briefcase icon), ที่อยู่อาศัย (Housing - house icon), ประเด็นสังคม (Social Issues - people icon). Include step arrows: ① เปิด Ads Manager → ② เลือก Campaign → ③ ตรวจ Special Ad Category → ④ แก้ไขแล้ว Publish. Dark gradient background (#1a1a2e to #16213e), accent color #e94560, modern flat design. Brand text 'Ads Vance' bottom-right corner. 16:9 ratio.",
  "thumbnail_prompt": "Eye-catching YouTube thumbnail. Large bold Thai text 'แอดถูก Reject!' in white with red glow effect. Facebook Ads Manager icon with red X mark. Dark dramatic background. Professional, click-worthy. 16:9 ratio.",
  "text_overlays": [
    {"text": "แอดถูก Reject!", "appear_at_seconds": 0, "duration_seconds": 5, "position": "top"},
    {"text": "① เปิด Ads Manager", "appear_at_seconds": 5, "duration_seconds": 10, "position": "bottom"},
    {"text": "② เช็ค Special Ad Category", "appear_at_seconds": 15, "duration_seconds": 15, "position": "bottom"},
    {"text": "③ แก้ไข → Publish ใหม่", "appear_at_seconds": 30, "duration_seconds": 10, "position": "bottom"},
    {"text": "ติดต่อ @adsvance", "appear_at_seconds": 40, "duration_seconds": 10, "position": "center"}
  ]
}
```

**Topic generation strategy:**
- Rotate through categories weekly (account, payment, campaign, pixel)
- Analyze common Facebook Ads problems from communities
- Avoid duplicate topics within 60 days
- Prioritize trending issues (Facebook platform updates, policy changes)

### Step ② Image Generation (GPT Image 2 via Kie.ai)

**API:** `POST https://api.kie.ai/api/v1/jobs/createTask`
**Model:** `gpt-image-2-text-to-image`

Generates 2 images per clip:

1. **Main infographic** (shown in video): 16:9, 2K resolution
   - Uses `image_prompt` from script
   - Style: dark gradient background, Thai text, icons, step-by-step arrows
   - Consistent brand colors (#1a1a2e, #e94560) and "Ads Vance" watermark

2. **Thumbnail** (for YouTube): 16:9, 2K resolution
   - Uses `thumbnail_prompt` from script
   - Bold, eye-catching, optimized for click-through

**API call example:**
```json
{
  "model": "gpt-image-2-text-to-image",
  "input": {
    "prompt": "{image_prompt from script}",
    "aspect_ratio": "16:9",
    "resolution": "2K"
  }
}
```

### Step ③ Voice Generation (ElevenLabs V3 via Kie.ai)

**API:** `POST https://api.kie.ai/api/v1/jobs/createTask`
**Model:** `elevenlabs/text-to-dialogue-v3`

Takes the `voice_script` field and generates Thai voiceover.

**API call example:**
```json
{
  "model": "elevenlabs/text-to-dialogue-v3",
  "input": {
    "dialogue": [
      {
        "text": "{voice_script from script}",
        "voice": "Adam"
      }
    ],
    "language_code": "th",
    "stability": 0.5
  }
}
```

**Script formatting conventions for natural Thai delivery:**
- `...` (ellipses) = natural pauses between steps
- `—` (dash) = emphasis or direction change
- `[confident]`, `[friendly]`, `[excited]`, `[serious]` = audio tags for emotion
- Audio tags only at key emotional beats, not every sentence

**Voice selection:** Test Adam, Brian, Roger initially. Pick the one that sounds most natural in Thai. Use that same voice for all clips (channel identity).

**Max 5000 characters per API call.** For scripts under 60 seconds, this is more than sufficient.

### Step ④ Video Assembly (FFmpeg + Python)

**Input per clip:**
- 1 infographic image (from Step ②)
- 1 voice audio file (from Step ③)
- Script JSON with text overlay timings

**Assembly process:**
1. **Intro** (2-3 seconds): Branded "Ads Vance" animation (made once, reused forever)
2. **Main content:** Single infographic image displayed for full duration with:
   - **Ken Burns effect:** Slow zoom-in from 100% to 110% over the clip duration
   - **Text overlays:** Appear/disappear at specified times with fade-in animation
   - **Highlight effects:** Subtle glow/pulse on relevant parts of the infographic
3. **Outro** (2-3 seconds): "ติดต่อ @adsvance" + Line ID + Telegram link
4. **Export:** MP4, 1080p, H.264, AAC audio

**Text overlay style:**
- Font: Noto Sans Thai (bold) — free, supports Thai fully
- Position: bottom third or top, per script specification
- Background: semi-transparent dark bar (rgba 0,0,0,0.6)
- Color: white text, accent color (#e94560) for keywords
- Animation: fade-in 0.3s

**Python + FFmpeg wrapper** handles:
- Reading script JSON
- Constructing FFmpeg filter_complex for zoom, text overlays, timing
- Outputting final MP4

---

## Daily Publish (Automated)

### QC Process

Automated checks before upload:
- Video duration within 25-120 seconds
- Audio track present and not silent
- Resolution is 1080p
- File size between 1MB and 50MB

Human QC is optional but recommended for the first 2 weeks. After confirming quality is consistent, switch to fully automated.

### YouTube Upload

**Method:** YouTube Data API v3
**Schedule:** Every day at 00:00 Bangkok time (UTC+7)

Upload metadata pulled directly from script JSON:
- Title: `youtube_title`
- Description: `youtube_description`
- Tags: `youtube_tags`
- Category: Education (27)
- Thumbnail: generated thumbnail from Step ②
- Visibility: Scheduled public at midnight

---

## Tech Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| Script Generation | Claude API (Sonnet 4.6) | Generate topics + structured scripts |
| Voice Generation | ElevenLabs V3 via Kie.ai | Thai AI voiceover |
| Image Generation | GPT Image 2 via Kie.ai | Infographics + thumbnails |
| Video Assembly | Python + FFmpeg | Combine image + audio + text overlays |
| Scheduling | Cron (VPS) or GitHub Actions | Trigger weekly production + daily upload |
| Upload | YouTube Data API v3 | Scheduled midnight upload |
| Orchestration | Python script | Pipeline coordinator |

---

## Cost Estimate (30 clips/month)

| Item | Est. Monthly Cost |
|------|------------------|
| Kie.ai Voice (30 clips x ~1 min each) | ~$5-15 |
| Kie.ai Image (30 infographics + 30 thumbnails) | ~$5-15 |
| Claude API (script generation) | ~$2-5 |
| VPS for automation | ~$5-10 |
| YouTube API | Free |
| **Total** | **~$17-45/month** |
| **Owner time** | **0 hours/month** |

---

## Modular Voice Design

Voice module is swappable without changing the rest of the pipeline:

| Option | When to use |
|--------|------------|
| ElevenLabs V3 (default) | Primary — best quality AI Thai voice |
| Human VO freelancer | If AI voice doesn't test well (~50-200 THB/clip) |
| Owner batch recording | Fallback — read 30 scripts in ~1 hour/month |

Script JSON output is the same regardless — only the voice generation step changes.

---

## Content Categories & Rotation

Rotate through categories weekly for variety:

| Week | Focus Category | Example Topics |
|------|---------------|---------------|
| 1 | Account & Access | บัญชีถูกจำกัด, ยืนยันตัวตน, 2FA, เฟสปลิว |
| 2 | Payment & Billing | ชำระเงินไม่ผ่าน, เติมเงินไม่ขึ้น, เปลี่ยนบัตร |
| 3 | Campaign & Ads | แอดไม่วิ่ง, ถูก reject, targeting ผิด, ad review |
| 4 | Pixel & Tracking | ติดตั้ง pixel, custom conversion, event setup |

Repeat cycle monthly with new specific topics each time.

---

## Risk & Mitigation

| Risk | Impact | Mitigation |
|------|--------|-----------|
| AI voice sounds unnatural in Thai | Viewers leave | Test first 7 clips, measure retention. Swap to human VO if needed. |
| AI-generated infographic has errors | Misleading content | Automated QC + human review first 2 weeks. Adjust prompts. |
| Kie.ai API downtime | Production stops | Queue-based with retry. 7-day buffer (produce weekly, publish daily). |
| YouTube API quota limits | Upload fails | Daily quota 10,000 units. Upload ~1,600 units. Well within limit. |
| Content becomes repetitive | Audience boredom | Topic dedup within 60 days. Category rotation. Monitor engagement. |
| GPT Image 2 generates incorrect Thai text | Unreadable text on image | Include English fallback for critical text. Review first batch. |

---

## Success Metrics

| Metric | Target (3-month) |
|--------|-----------------|
| Upload consistency | 30/30 days per month |
| Average view duration | >50% of clip length |
| Subscriber growth | 10% month-over-month |
| Owner time per month | 0 hours (after setup) |
| Cost per clip | <$1.50 |

---

## Initial Setup (One-time)

What needs to happen before the pipeline runs:

1. Set up Kie.ai account + credits
2. Set up Claude API key
3. Set up YouTube Data API credentials + OAuth
4. Create branded intro/outro video (2-3 seconds each)
5. Choose and lock ElevenLabs voice (test 3 voices, pick best Thai)
6. Deploy Python pipeline to VPS or GitHub Actions
7. Configure cron jobs: weekly production (Monday), daily upload (23:30 UTC)
8. Run first 7 clips manually, QC, then switch to auto
