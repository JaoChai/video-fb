# Ads Vance Content Factory V2 — Design Spec

## Overview

Fully autonomous content production system for YouTube channel **@adsvance**. The system generates Q&A-style videos answering simulated customer questions about Facebook Ads — specifically targeting advertisers who face account restrictions, payment issues, and verification problems.

**Zero human in the loop** after initial setup.

**Output:** 1 video/day posted across YouTube (16:9), TikTok, IG Reels, FB Reels (9:16) at midnight Bangkok time.

**Video format:** Multiple scenes per clip — question card with customer name → step-by-step answer images → CTA/summary. AI voiceover reads the question and explains the answer.

---

## Brand Identity

- **Name:** Ads Vance (แอดวานซ์)
- **Mascot:** Leopard astronaut riding a rocket, holding phone with Facebook icon
- **Primary color:** Navy blue (#1a3a8f)
- **Secondary:** White (#ffffff)
- **Accent:** Orange (#f5851f)
- **Font:** Noto Sans Thai (bold) for Thai text
- **Style:** Fun, energetic, tech-savvy, approachable
- **CTA:** "ติดต่อซื้อบัญชี Line: @adsvance"

---

## System Architecture

```
┌───────────────────────────────────────────────────────────────────────┐
│                     React Dashboard (SPA)                             │
│                                                                       │
│  Content     │  RAG        │  Agent      │  Schedule   │  Analytics   │
│  Manager     │  Manager    │  Config     │  Manager    │  Reports     │
└──────────────┴──────┬──────┴─────────────┴─────────────┴──────────────┘
                      │ REST API (JSON)
┌─────────────────────┴─────────────────────────────────────────────────┐
│                     Go Backend (Railway)                               │
│                                                                       │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │                    Agent Orchestrator                            │  │
│  │                                                                 │  │
│  │  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐      │  │
│  │  │ Question  │ │  Script   │ │  Image    │ │ Analytics │      │  │
│  │  │  Agent    │ │  Agent    │ │  Agent    │ │  Agent    │      │  │
│  │  └─────┬─────┘ └─────┬─────┘ └─────┬─────┘ └─────┬─────┘      │  │
│  │        │              │             │             │             │  │
│  │        └──────────────┴──────┬──────┴─────────────┘             │  │
│  │                              │                                  │  │
│  │                    ┌─────────┴─────────┐                        │  │
│  │                    │   RAG Engine      │                        │  │
│  │                    │   (pgvector)      │                        │  │
│  │                    └───────────────────┘                        │  │
│  └─────────────────────────────────────────────────────────────────┘  │
│                                                                       │
│  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐               │
│  │ Video         │ │ Knowledge     │ │ Scheduler     │               │
│  │ Producer      │ │ Crawler       │ │ (Cron)        │               │
│  └───────┬───────┘ └───────┬───────┘ └───────┬───────┘               │
│          │                 │                 │                        │
│  External APIs:            │                 │                        │
│  • Claude API (reasoning)  │                 │                        │
│  • Kie.ai (voice + image)  │                 │                        │
│  • Zernio (post + stats)   │                 │                        │
│  • FFmpeg (video assembly) │                 │                        │
└──────────┬─────────────────┴─────────────────┴────────────────────────┘
           │
    ┌──────┴──────┐
    │ Neon        │
    │ PostgreSQL  │
    │ + pgvector  │
    └─────────────┘
```

---

## Database Schema (Neon PostgreSQL)

```sql
-- Content
CREATE TABLE clips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    question TEXT NOT NULL,
    questioner_name TEXT NOT NULL,
    answer_script TEXT NOT NULL,
    voice_script TEXT NOT NULL,
    category TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft', -- draft, producing, ready, published, failed
    video_16_9_url TEXT,
    video_9_16_url TEXT,
    thumbnail_url TEXT,
    publish_date DATE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE scenes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id UUID REFERENCES clips(id) ON DELETE CASCADE,
    scene_number INT NOT NULL,
    scene_type TEXT NOT NULL, -- question, step, summary
    text_content TEXT NOT NULL,
    image_prompt TEXT NOT NULL,
    image_16_9_url TEXT,
    image_9_16_url TEXT,
    voice_text TEXT NOT NULL,
    duration_seconds FLOAT NOT NULL,
    text_overlays JSONB DEFAULT '[]'
);

CREATE TABLE clip_metadata (
    clip_id UUID PRIMARY KEY REFERENCES clips(id) ON DELETE CASCADE,
    youtube_title TEXT,
    youtube_description TEXT,
    youtube_tags TEXT[],
    zernio_post_id TEXT,
    youtube_video_id TEXT,
    tiktok_post_id TEXT,
    ig_post_id TEXT,
    fb_post_id TEXT
);

-- Analytics
CREATE TABLE clip_analytics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clip_id UUID REFERENCES clips(id) ON DELETE CASCADE,
    platform TEXT NOT NULL, -- youtube, tiktok, instagram, facebook
    views INT DEFAULT 0,
    likes INT DEFAULT 0,
    comments INT DEFAULT 0,
    shares INT DEFAULT 0,
    watch_time_seconds FLOAT DEFAULT 0,
    retention_rate FLOAT DEFAULT 0,
    fetched_at TIMESTAMPTZ DEFAULT NOW()
);

-- RAG Knowledge Base
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE knowledge_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    source_type TEXT NOT NULL, -- official, practitioner, community
    crawl_frequency TEXT DEFAULT 'weekly', -- daily, weekly, monthly
    last_crawled_at TIMESTAMPTZ,
    enabled BOOLEAN DEFAULT TRUE
);

CREATE TABLE knowledge_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID REFERENCES knowledge_sources(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    embedding VECTOR(1536),
    metadata JSONB DEFAULT '{}',
    url TEXT,
    crawled_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX ON knowledge_chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- Topic History (dedup)
CREATE TABLE topic_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    category TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Agent Configuration
CREATE TABLE agent_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_name TEXT UNIQUE NOT NULL, -- question, script, image, analytics
    system_prompt TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT 'claude-sonnet-4-6-20250514',
    temperature FLOAT DEFAULT 0.7,
    enabled BOOLEAN DEFAULT TRUE,
    config JSONB DEFAULT '{}'
);

-- Schedule
CREATE TABLE schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    cron_expression TEXT NOT NULL, -- e.g., "0 3 * * 1" (Monday 10am BKK)
    action TEXT NOT NULL, -- produce_weekly, publish_daily, crawl_knowledge, fetch_analytics
    enabled BOOLEAN DEFAULT TRUE,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ
);

-- Brand Theme
CREATE TABLE brand_themes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL DEFAULT 'default',
    primary_color TEXT NOT NULL DEFAULT '#1a3a8f',
    secondary_color TEXT NOT NULL DEFAULT '#ffffff',
    accent_color TEXT NOT NULL DEFAULT '#f5851f',
    font_name TEXT NOT NULL DEFAULT 'Noto Sans Thai',
    logo_url TEXT,
    mascot_description TEXT DEFAULT 'Leopard astronaut riding a rocket, holding phone with Facebook icon',
    image_style TEXT DEFAULT 'Modern flat design, dark gradient background, energetic, tech-savvy',
    active BOOLEAN DEFAULT TRUE
);
```

---

## Multi-Agent System

### Agent 1: Question Agent

**Purpose:** Generate realistic customer questions about Facebook Ads problems.

**Input:** Category rotation + topic history + RAG context
**Output:** 7 question objects per week

```json
{
  "question": "บัญชีโฆษณาถูกปิดกะทันหัน ทำยังไงให้กลับมาใช้ได้?",
  "questioner_name": "คุณ สมชาย",
  "category": "account",
  "pain_point": "account_banned",
  "difficulty": "common"
}
```

**Behavior:**
- Uses RAG to find real problems people face (from Reddit, communities)
- Generates Thai names for questioners
- Ensures questions feel natural and relatable
- Avoids questions about policy circumvention
- Dedup against 60-day topic history
- Rotates categories weekly: account → payment → campaign → pixel

### Agent 2: Script Agent

**Purpose:** Write the answer script for each question.

**Input:** Question + RAG knowledge
**Output:** Multi-scene script with voice text

```json
{
  "scenes": [
    {
      "scene_number": 1,
      "scene_type": "question",
      "text_content": "คุณ สมชาย ถามว่า:\n\"บัญชีโฆษณาถูกปิดกะทันหัน ทำยังไงให้กลับมาใช้ได้?\"",
      "voice_text": "วันนี้ คุณสมชาย ถามเข้ามาว่า... บัญชีโฆษณาถูกปิดกะทันหัน ทำยังไงให้กลับมาใช้ได้ มาดูกันเลย",
      "duration_seconds": 8
    },
    {
      "scene_number": 2,
      "scene_type": "step",
      "text_content": "ขั้นตอนที่ 1: เข้า Account Quality",
      "voice_text": "[confident] อันดับแรก... ให้เข้าไปที่ Account Quality ก่อน เพื่อดูว่าบัญชีของคุณถูกจำกัดเพราะอะไร",
      "duration_seconds": 12
    },
    {
      "scene_number": 3,
      "scene_type": "step",
      "text_content": "ขั้นตอนที่ 2: ยื่นอุทธรณ์",
      "voice_text": "ถ้าเห็นว่าโดน restrict ให้กดปุ่ม Request Review เพื่อยื่นอุทธรณ์ อธิบายให้ชัดว่าคุณไม่ได้ทำผิดนโยบาย",
      "duration_seconds": 12
    },
    {
      "scene_number": 4,
      "scene_type": "step",
      "text_content": "ขั้นตอนที่ 3: รอผล + เตรียมบัญชีสำรอง",
      "voice_text": "ระหว่างรอผล— ซึ่งอาจใช้เวลา 1-3 วัน ควรเตรียมบัญชีสำรองไว้ด้วย เพื่อไม่ให้ธุรกิจหยุดชะงัก",
      "duration_seconds": 10
    },
    {
      "scene_number": 5,
      "scene_type": "summary",
      "text_content": "สรุป + ติดต่อ @adsvance",
      "voice_text": "[friendly] สรุปคือ เช็ค Account Quality... ยื่นอุทธรณ์... แล้วเตรียมสำรอง ถ้าต้องการบัญชีสำรองคุณภาพ ทักมาที่ไลน์ @adsvance ได้เลยครับ!",
      "duration_seconds": 10
    }
  ],
  "total_duration_seconds": 52,
  "youtube_title": "บัญชีโฆษณาถูกปิด! 3 ขั้นตอนกู้คืนด่วน {Ads Vance}",
  "youtube_description": "วิธีแก้ปัญหาบัญชีโฆษณา Facebook ถูกปิดกะทันหัน...\n\nติดต่อซื้อบัญชี line id : @adsvance\nเข้ากลุ่มเทเลแกรม: https://t.me/adsvancech",
  "youtube_tags": ["facebook ads", "บัญชีโฆษณา", "แอดถูกปิด", "ads vance"]
}
```

**Behavior:**
- Always answers with RAG-backed accurate information
- CTA always mentions buying backup accounts from @adsvance
- Voice script uses Thai with natural pauses (...) and emphasis (—)
- Audio tags [confident], [friendly] for ElevenLabs V3
- Total duration 30-90 seconds
- Content filter: reject any script that advises policy violations

### Agent 3: Image Agent

**Purpose:** Generate image prompts for each scene, matching brand theme.

**Input:** Scene descriptions + brand theme config
**Output:** Image prompts for GPT Image 2

**Scene-type templates:**

**Question scene:**
```
Professional Q&A card design. Dark navy blue gradient background (#1a3a8f to #0d1f4d).
Top: "Ads Vance" logo text in white with orange accent.
Center: Chat bubble style with question text in Thai: "{question}".
Below bubble: "— {questioner_name}" in orange (#f5851f).
Bottom-right: Small leopard mascot icon.
Style: Modern, clean, bold Thai typography. 16:9 ratio.
```

**Step scene:**
```
Infographic step card. Dark navy blue gradient background.
Top-left: Step number "①" in large orange circle.
Center: Visual explanation of "{step_description}" with relevant icons.
Thai text labels in white, Noto Sans Thai Bold style.
Orange (#f5851f) accent lines and arrows.
Bottom-right: "Ads Vance" brand text small.
Modern flat design, clean, professional. {aspect_ratio} ratio.
```

**Summary/CTA scene:**
```
Call-to-action card. Dark navy blue gradient background.
Center: "ติดต่อซื้อบัญชี" in large white text.
Below: "Line: @adsvance" with Line icon in green.
Below: "Telegram: t.me/adsvancech" with Telegram icon.
Bottom: Leopard mascot character.
Energetic, inviting, professional. {aspect_ratio} ratio.
```

### Agent 4: Analytics Agent

**Purpose:** Analyze post performance and generate insights for feedback loop.

**Input:** Clip analytics from Zernio
**Output:** Performance report + recommendations

```json
{
  "period": "2026-W18",
  "top_performing": {
    "clip_id": "xxx",
    "title": "บัญชีโฆษณาถูกปิด!",
    "views": 1250,
    "retention_rate": 0.72,
    "why_good": "Account ban topics have highest search intent"
  },
  "worst_performing": {
    "clip_id": "yyy",
    "views": 45,
    "why_bad": "Pixel setup is too technical for audience"
  },
  "recommendations": [
    "Increase account/ban related topics from 25% to 40%",
    "Shorter clips (30-40s) perform 2x better than 60s+",
    "Questions with emotional hooks get 3x more views"
  ],
  "next_week_category_weights": {
    "account": 0.4,
    "payment": 0.3,
    "campaign": 0.2,
    "pixel": 0.1
  }
}
```

**Behavior:**
- Runs weekly after 7 days of data collection
- Feeds category weights back to Question Agent
- Tracks trends over time (which topics grow, which decline)
- Suggests optimal clip duration based on retention data

---

## Video Production Pipeline

### Step 1: Image Generation (GPT Image 2 via Kie.ai)

Per clip (5 scenes x 2 formats = 10 images + 1 thumbnail):

```
POST https://api.kie.ai/api/v1/jobs/createTask
{
  "model": "gpt-image-2-text-to-image",
  "input": {
    "prompt": "{scene_image_prompt}",
    "aspect_ratio": "16:9",  // or "9:16"
    "resolution": "2K"
  }
}
```

### Step 2: Voice Generation (ElevenLabs V3 via Kie.ai)

One voice file per clip (all scenes concatenated):

```
POST https://api.kie.ai/api/v1/jobs/createTask
{
  "model": "elevenlabs/text-to-dialogue-v3",
  "input": {
    "dialogue": [
      {"text": "{full_voice_script}", "voice": "Adam"}
    ],
    "language_code": "th",
    "stability": 0.5
  }
}
```

### Step 3: Video Assembly (FFmpeg)

Assemble 2 videos per clip:

**16:9 (YouTube):**
- 1920x1080, H.264, AAC
- Ken Burns zoom on each scene image
- Text overlays with Noto Sans Thai
- Fade transitions between scenes

**9:16 (Shorts/TikTok/IG Reels):**
- 1080x1920, H.264, AAC
- Same content, different image crops
- Optimized text positioning for vertical

### Step 4: Publishing (Zernio API)

```
POST https://zernio.com/api/v1/posts
{
  "text": "{youtube_title}\n\n{youtube_description}",
  "platforms": ["youtube", "tiktok", "instagram", "facebook"],
  "mediaUrls": ["{video_url}"],
  "scheduledFor": "2026-04-28T17:00:00Z"  // midnight BKK
}
```

### Step 5: Analytics Pull (Zernio API)

```
GET https://zernio.com/api/v1/analytics/{post_id}
```

Pull weekly → store in clip_analytics → feed to Analytics Agent.

---

## RAG Knowledge Base

### Sources to Crawl

**Official (weekly crawl):**
| Source | URL | Focus |
|--------|-----|-------|
| Meta Business Help Center | business.facebook.com/help | Policies, verification, appeals |
| Facebook Ads Policies | facebook.com/policies/ads | Ad review rules |
| Meta for Developers | developers.facebook.com | Pixel, API, technical |
| Meta Community Standards | transparency.meta.com | Content restrictions |

**Practitioner (weekly crawl):**
| Source | URL | Focus |
|--------|-----|-------|
| Jon Loomer Digital | jonloomer.com | Advanced tactics, settings |
| AdEspresso Blog | adespresso.com/blog | A/B testing, optimization |
| Social Media Examiner | socialmediaexaminer.com | Feature updates |
| Lebesgue Blog | lebesgue.io/blog | Data-driven strategies |

**Community (daily crawl):**
| Source | URL | Focus |
|--------|-----|-------|
| r/FacebookAds | reddit.com/r/FacebookAds | Real problems, real solutions |
| r/PPC | reddit.com/r/PPC | Cross-platform ad issues |

### Embedding Pipeline

1. Crawl source → extract text content
2. Chunk into ~500 token segments with overlap
3. Generate embeddings via Claude or OpenAI embedding API
4. Store in pgvector on Neon
5. Index for cosine similarity search

### RAG Query Flow

```
Agent question → generate embedding → cosine search top 5 chunks →
inject as context → agent generates answer with citations
```

---

## React Dashboard

### Pages

1. **Content Manager**
   - List all clips with status (draft/producing/ready/published/failed)
   - Preview clips before publish
   - View generated scripts and images
   - Retry failed productions

2. **RAG Manager**
   - List knowledge sources (enable/disable/add/remove)
   - View crawl status and last crawled time
   - Browse knowledge chunks
   - Manual knowledge entry

3. **Agent Config**
   - Edit system prompts for each agent
   - Adjust model, temperature
   - Enable/disable agents
   - View agent run history

4. **Schedule Manager**
   - View/edit cron schedules
   - Manual trigger (produce now, publish now)
   - View execution logs

5. **Analytics Dashboard**
   - Performance charts (views, engagement over time)
   - Top performing clips
   - Category performance comparison
   - Agent recommendations display

---

## Decomposition into Sub-Projects

### Sub-Project 1: Core Backend + DB
**Scope:** Go API server, Neon DB setup, Railway deploy, CRUD for all entities
**Deliverable:** Running API on Railway with DB on Neon

### Sub-Project 2: RAG + Multi-Agent Pipeline
**Scope:** Knowledge crawler, pgvector RAG, 4 agents, video production
**Deliverable:** Automated weekly clip production

### Sub-Project 3: Zernio Integration + Scheduler
**Scope:** Multi-platform posting, analytics pull, feedback loop, cron
**Deliverable:** Automated publishing and self-improving content

### Sub-Project 4: React Dashboard
**Scope:** Full control UI for all features
**Deliverable:** Web dashboard to monitor and manage everything

---

## Cost Estimate

| Item | Monthly Cost |
|------|-------------|
| Railway (Go server) | ~$5-20 |
| Neon PostgreSQL (Pro) | ~$19 |
| Kie.ai Voice (30 clips x 1 min) | ~$10-20 |
| Kie.ai Image (30 clips x 5 scenes x 2 formats + thumbs) | ~$30-50 |
| Claude API (4 agents x 30 runs) | ~$15-25 |
| Zernio (posting + analytics) | ~$29-49 |
| **Total** | **~$108-183/month** |
| **Owner time** | **0 hours/month** |

---

## Content Safety Rules

All agents must enforce these rules:

1. **Never** advise on circumventing Facebook policies
2. **Never** mention "gray hat", "black hat", or policy exploitation
3. **Always** frame advice as legitimate compliance and best practices
4. **Always** recommend proper appeals and official channels
5. **CTA** focuses on selling backup accounts as business continuity measure
6. Content positioning: "helping advertisers maintain business continuity"
