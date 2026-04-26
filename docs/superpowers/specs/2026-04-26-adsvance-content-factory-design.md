# Ads Vance Content Factory — Design Spec

## Overview

Automated content pipeline for YouTube channel **@adsvance** that produces 1 short video per day (30-60 seconds) about Facebook Ads account tips/tricks in Thai language. Goal: remove channel owner from daily production loop entirely.

**Output:** 1 clip/day, uploaded at midnight, Thai language only.
**Owner involvement:** ~2-3 hours/month (batch screenshot capture only).

---

## Architecture

```
MONTHLY PREP (owner ~2 hr)        WEEKLY PRODUCE (automated)         DAILY PUBLISH (automated)
┌──────────────────────┐    ┌──────────────────────────────┐    ┌────────────────────────┐
│ Screenshot Library   │    │ ① Claude: 7 scripts          │    │ QC check (human/AI)    │
│ + Screenshot Index   │───>│ ② GPT Image 2: thumbnails    │───>│ YouTube scheduled      │
│                      │    │ ③ ElevenLabs V3: Thai voice   │    │ upload at 00:00        │
│                      │    │ ④ FFmpeg: assemble video      │    │                        │
└──────────────────────┘    └──────────────────────────────┘    └────────────────────────┘
```

---

## Phase 1: Monthly Prep

### 1.1 Screenshot Library

Owner opens Facebook Ads Manager / Business Manager and captures screenshots of all key screens, organized by category:

```
screenshots/
├── account/
│   ├── account-overview.png
│   ├── account-quality.png
│   ├── account-settings.png
│   └── account-error-restricted.png
├── payment/
│   ├── payment-settings.png
│   ├── billing-summary.png
│   ├── add-payment-method.png
│   └── payment-failed-error.png
├── campaign/
│   ├── campaign-creation.png
│   ├── ad-set-settings.png
│   ├── ad-level.png
│   ├── ad-rejected.png
│   └── special-ad-category.png
├── pixel/
│   ├── events-manager.png
│   ├── pixel-setup.png
│   ├── custom-conversions.png
│   └── pixel-not-found-error.png
├── business-manager/
│   ├── bm-settings.png
│   ├── bm-people.png
│   ├── bm-pages.png
│   └── bm-ad-accounts.png
├── verification/
│   ├── identity-verification.png
│   ├── two-factor-auth.png
│   └── business-verification.png
└── errors/
    ├── common-error-1.png
    └── common-error-2.png
```

**Naming convention:** `{category}/{descriptive-name}.png`
**Storage:** Google Drive shared folder or local project directory.
**Update cadence:** Monthly. Only add new screenshots when Facebook updates UI.

### 1.2 Screenshot Index

A JSON file mapping screenshot paths to descriptions and tags for AI script generation:

```json
{
  "screenshots": [
    {
      "path": "account/account-quality.png",
      "description": "Facebook Account Quality page showing account health score",
      "tags": ["account", "quality", "health", "restriction"],
      "added": "2026-04-26"
    }
  ]
}
```

---

## Phase 2: Weekly Produce (Automated)

### Step ① Script Generation (Claude API)

**Trigger:** Weekly cron job (every Monday)
**Input:** Topic Bank + Screenshot Index
**Output:** 7 structured scripts in JSON format

Each script contains:

```json
{
  "id": "clip-2026-04-28",
  "title": "แอดถูก reject เพราะ Special Ad Category แก้ยังไง?",
  "youtube_title": "แอดโดน Reject! แก้ปัญหา Special Ad Category ใน 1 นาที {Ads Vance}",
  "youtube_description": "วิธีแก้ปัญหาแอดถูก reject...\n\nติดต่อทีมงาน line id : @adsvance\nเข้ากลุ่มเทเลแกรม: https://t.me/adsvancech",
  "youtube_tags": ["facebook ads", "โฆษณาเฟสบุ๊ค", "แอดถูก reject", "special ad category"],
  "duration_target_seconds": 50,
  "scenes": [
    {
      "scene": 1,
      "duration_seconds": 5,
      "voice_text": "แอดคุณถูก reject เพราะ Special Ad Category ใช่ไหม? แก้ได้ง่ายมาก",
      "audio_tags": "[confident]",
      "visual_type": "screenshot",
      "visual_source": "campaign/ad-rejected.png",
      "text_overlay": "แอดถูก Reject!",
      "zoom_target": null
    },
    {
      "scene": 2,
      "duration_seconds": 15,
      "voice_text": "เข้า Ads Manager... กดที่แคมเปญที่มีปัญหา... แล้วดูตรง Special Ad Categories",
      "audio_tags": "",
      "visual_type": "screenshot",
      "visual_source": "campaign/special-ad-category.png",
      "text_overlay": "ขั้นตอนที่ 1: เข้า Campaign Settings",
      "zoom_target": { "x": 200, "y": 300, "w": 400, "h": 200 }
    },
    {
      "scene": 3,
      "duration_seconds": 20,
      "voice_text": "ปัญหาคือคุณเลือก category ผิด หรือไม่ได้เลือกทั้งที่ต้องเลือก",
      "audio_tags": "",
      "visual_type": "generated_infographic",
      "visual_prompt": "Clean infographic showing Facebook Special Ad Categories: Credit, Employment, Housing, Social Issues. Thai language labels. Modern flat design, dark background, colorful icons.",
      "text_overlay": "เช็คหมวดหมู่ให้ถูกต้อง",
      "zoom_target": null
    },
    {
      "scene": 4,
      "duration_seconds": 10,
      "voice_text": "แก้แล้วกด publish ใหม่ได้เลย ถ้ายังติดปัญหา ทักมาที่ไลน์ @adsvance",
      "audio_tags": "[friendly]",
      "visual_type": "screenshot",
      "visual_source": "campaign/publish-button.png",
      "text_overlay": "ติดต่อ @adsvance",
      "zoom_target": null
    }
  ]
}
```

**Topic generation strategy:**
- Analyze common Facebook Ads problems from communities/groups
- Rotate categories: account issues, payment, pixel, campaign errors, verification, BM settings
- Avoid duplicate topics within 60 days
- Prioritize trending issues (Facebook platform updates, policy changes)

### Step ② Image Generation (GPT Image 2 via Kie.ai)

**API:** `POST https://api.kie.ai/api/v1/jobs/createTask`
**Model:** `gpt-image-2-text-to-image`

Used for:
- **Thumbnails** (1 per clip): 16:9 ratio, 2K resolution, bold Thai text, eye-catching
- **Infographics** (when script specifies `visual_type: "generated_infographic"`): diagrams, flowcharts, comparison graphics

Not used for: Facebook Ads Manager UI screenshots (use real screenshots instead).

**Thumbnail prompt template:**
```
YouTube thumbnail for Facebook Ads tutorial video.
Topic: {topic_thai}
Style: Bold Thai text "{short_title}" in center, {accent_color} accent,
dark gradient background, relevant icon ({icon_description}).
Professional, clean, 16:9 ratio.
Brand: "Ads Vance" small logo bottom-right.
```

### Step ③ Voice Generation (ElevenLabs V3 via Kie.ai)

**API:** `POST https://api.kie.ai/api/v1/jobs/createTask`
**Model:** `elevenlabs/text-to-dialogue-v3`

Configuration:
- `language_code`: `"th"` (Thai)
- `stability`: `0.5` (balanced)
- Single speaker per clip (consistent voice identity)
- Audio tags for expressiveness: `[confident]`, `[friendly]`, `[excited]`, `[serious]`

**Voice selection:** Choose one consistent voice for channel identity. Test multiple voices initially (Adam, Brian, Roger) and pick the one that sounds most natural in Thai.

**Script preprocessing:**
- Add ellipses (`...`) for natural pauses between steps
- Add dashes (`—`) for emphasis/interruptions
- Audio tags at emotional beats only (not every line)
- Max 5000 characters per API call (split if needed)

**Output:** One MP3/WAV file per clip, with all scenes concatenated.

### Step ④ Video Assembly (FFmpeg)

**Input per clip:**
- Voice audio file (from Step ③)
- Screenshot images + generated images (from Step ②)
- Script JSON (timing, text overlays, zoom targets)

**Assembly process:**
1. Create intro (2s branded animation — made once, reused)
2. For each scene:
   - Display image (screenshot or infographic)
   - Apply zoom/pan effect if `zoom_target` specified
   - Overlay text with consistent font/style
   - Sync with voice audio timing
3. Add outro (2s — Line ID, Telegram link)
4. Export: MP4, 1080p, H.264

**Text overlay style:**
- Font: TH Sarabun or Noto Sans Thai (bold)
- Position: bottom third or top, depending on image content
- Background: semi-transparent dark bar
- Color: white text, accent color for keywords

**FFmpeg command structure (conceptual):**
```bash
ffmpeg -i voice.mp3 \
  -loop 1 -i scene1.png -t {duration1} \
  -loop 1 -i scene2.png -t {duration2} \
  ... \
  -filter_complex "[scene assembly + text overlays + zoom effects]" \
  -c:v libx264 -c:a aac -shortest output.mp4
```

Actual implementation will use a Python script wrapping FFmpeg for complex scene assembly.

---

## Phase 3: Daily Publish

### QC Process

Before upload, each clip goes through:

1. **Automated checks:**
   - Video duration within 30-120 seconds
   - Audio present and synced
   - Resolution is 1080p
   - File size reasonable

2. **Human QC (optional but recommended initially):**
   - Watch 1-minute clip
   - Check voice sounds natural
   - Check screenshots match topic
   - Approve or flag for regeneration

### YouTube Upload

**Method:** YouTube Data API v3 (scheduled upload)
**Schedule:** Every day at 00:00 (midnight) Bangkok time (UTC+7)

Upload metadata from script:
- Title: `youtube_title` from script
- Description: `youtube_description` (includes Line ID, Telegram link)
- Tags: `youtube_tags`
- Category: Education or People & Blogs
- Thumbnail: generated thumbnail from Step ②
- Visibility: Scheduled (public at midnight)

---

## Tech Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| Script Generation | Claude API (Opus/Sonnet) | Generate topics + structured scripts |
| Voice Generation | ElevenLabs V3 via Kie.ai | Thai AI voiceover |
| Image Generation | GPT Image 2 via Kie.ai | Thumbnails + infographics |
| Video Assembly | Python + FFmpeg | Combine images + audio + text |
| Scheduling | Cron (VPS) or GitHub Actions | Trigger weekly production |
| Upload | YouTube Data API v3 | Scheduled midnight upload |
| Storage | Google Drive or local | Screenshot library |
| Orchestration | Python script | Pipeline coordinator |

---

## Cost Estimate (30 clips/month)

| Item | Est. Monthly Cost |
|------|------------------|
| Kie.ai Voice (30 clips x ~1 min each) | ~$5-15 |
| Kie.ai Image (30 thumbnails + ~15 infographics) | ~$5-10 |
| Claude API (script generation) | ~$2-5 |
| VPS for automation | ~$5-10 |
| YouTube API | Free |
| **Total** | **~$17-40/month** |
| Owner time | ~2-3 hours/month |

---

## Modular Voice Design

Voice module is swappable without changing the rest of the pipeline:

| Option | When to use |
|--------|------------|
| ElevenLabs V3 (default) | Primary — best quality AI Thai voice |
| Human VO freelancer | If AI voice doesn't test well with audience |
| Owner batch recording | Fallback — owner reads 30 scripts in ~1 hour/month |

Script JSON output is the same regardless — only the voice generation step changes.

---

## Content Categories & Rotation

To maintain variety, rotate through these categories weekly:

| Week | Focus Category | Example Topics |
|------|---------------|---------------|
| 1 | Account & Access | บัญชีถูกจำกัด, ยืนยันตัวตน, 2FA |
| 2 | Payment & Billing | ชำระเงินไม่ผ่าน, เติมเงินไม่ขึ้น, เปลี่ยนบัตร |
| 3 | Campaign & Ads | แอดไม่วิ่ง, ถูก reject, targeting ผิด |
| 4 | Pixel & Tracking | ติดตั้ง pixel, custom conversion, event setup |

Repeat cycle monthly, with new specific topics each time.

---

## Risk & Mitigation

| Risk | Impact | Mitigation |
|------|--------|-----------|
| AI voice sounds unnatural in Thai | Viewers leave | Test first 7 clips, measure retention. Swap to human VO if needed. |
| Facebook UI changes frequently | Screenshots outdated | Monthly refresh cycle. AI detects when screenshot doesn't match topic context. |
| Kie.ai API downtime | Production stops | Queue-based system with retry. 7-day buffer (produce weekly, publish daily). |
| YouTube API quota limits | Upload fails | YouTube daily quota is 10,000 units. Upload costs ~1,600 units. Well within limit. |
| Content becomes repetitive | Audience boredom | Topic dedup within 60 days. Monitor engagement metrics. |

---

## Success Metrics

| Metric | Target (3-month) |
|--------|-----------------|
| Upload consistency | 30/30 days per month |
| Average view duration | >50% of clip length |
| Subscriber growth | 10% month-over-month |
| Owner time per month | <3 hours |
| Cost per clip | <$1.50 |
