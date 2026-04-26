# Ads Vance Content Factory — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a fully automated Python pipeline that produces and uploads 1 Facebook Ads tutorial video per day to YouTube channel @adsvance — zero manual intervention after setup.

**Architecture:** A Python CLI pipeline with 4 modules: script generator (Claude API), image generator (GPT Image 2 via Kie.ai), voice generator (ElevenLabs V3 via Kie.ai), and video assembler (FFmpeg). An orchestrator ties them together. A scheduler triggers weekly production and daily upload via cron.

**Tech Stack:** Python 3.14, httpx (async HTTP), anthropic SDK, FFmpeg 8.0, google-api-python-client (YouTube), Noto Sans Thai font, pytest.

---

## File Structure

```
video-fb/
├── src/
│   ├── __init__.py
│   ├── config.py              # env vars, API keys, constants
│   ├── models.py              # Pydantic models for Script, Overlay, etc.
│   ├── script_generator.py    # Claude API → 7 structured scripts
│   ├── image_generator.py     # GPT Image 2 via Kie.ai → infographic + thumbnail
│   ├── voice_generator.py     # ElevenLabs V3 via Kie.ai → Thai voiceover
│   ├── video_assembler.py     # FFmpeg → MP4 with Ken Burns + text overlays
│   ├── youtube_uploader.py    # YouTube Data API v3 → scheduled upload
│   ├── qc.py                  # Automated QC checks before upload
│   └── pipeline.py            # Orchestrator: ties all modules together
├── assets/
│   ├── fonts/
│   │   └── NotoSansThai-Bold.ttf
│   ├── intro.mp4              # 2-3s branded intro (created once)
│   └── outro.mp4              # 2-3s branded outro (created once)
├── output/
│   ├── scripts/               # Generated script JSONs
│   ├── images/                # Generated infographics + thumbnails
│   ├── audio/                 # Generated voice files
│   └── videos/                # Final assembled MP4s
├── data/
│   └── topic_history.json     # Track used topics for dedup
├── tests/
│   ├── __init__.py
│   ├── test_config.py
│   ├── test_models.py
│   ├── test_script_generator.py
│   ├── test_image_generator.py
│   ├── test_voice_generator.py
│   ├── test_video_assembler.py
│   ├── test_youtube_uploader.py
│   ├── test_qc.py
│   └── test_pipeline.py
├── requirements.txt
├── .env.example
└── README.md
```

---

## Task 1: Project Setup & Data Models

**Files:**
- Create: `requirements.txt`
- Create: `.env.example`
- Create: `src/__init__.py`
- Create: `src/config.py`
- Create: `src/models.py`
- Create: `tests/__init__.py`
- Create: `tests/test_models.py`

- [ ] **Step 1: Create requirements.txt**

```txt
anthropic>=0.52.0
httpx>=0.28.0
pydantic>=2.11.0
google-api-python-client>=2.170.0
google-auth-oauthlib>=1.2.0
python-dotenv>=1.1.0
pytest>=8.4.0
pytest-asyncio>=1.0.0
```

- [ ] **Step 2: Create .env.example**

```env
ANTHROPIC_API_KEY=sk-ant-xxx
KIE_API_KEY=kie-xxx
YOUTUBE_CLIENT_SECRET_PATH=client_secret.json
ELEVENLABS_VOICE=Adam
ELEVENLABS_STABILITY=0.5
OUTPUT_DIR=./output
ASSETS_DIR=./assets
```

- [ ] **Step 3: Install dependencies**

Run: `pip3 install -r requirements.txt`
Expected: All packages install successfully.

- [ ] **Step 4: Create src/__init__.py and tests/__init__.py**

```python
# src/__init__.py — empty
```

```python
# tests/__init__.py — empty
```

- [ ] **Step 5: Write the test for config**

Create `tests/test_config.py`:

```python
import os
import pytest
from unittest.mock import patch


def test_config_loads_from_env():
    env = {
        "ANTHROPIC_API_KEY": "sk-ant-test",
        "KIE_API_KEY": "kie-test",
        "ELEVENLABS_VOICE": "Brian",
        "ELEVENLABS_STABILITY": "0.8",
        "OUTPUT_DIR": "/tmp/output",
        "ASSETS_DIR": "/tmp/assets",
    }
    with patch.dict(os.environ, env, clear=False):
        from src.config import load_config

        cfg = load_config()
        assert cfg.anthropic_api_key == "sk-ant-test"
        assert cfg.kie_api_key == "kie-test"
        assert cfg.elevenlabs_voice == "Brian"
        assert cfg.elevenlabs_stability == 0.8
        assert cfg.output_dir == "/tmp/output"


def test_config_defaults():
    env = {
        "ANTHROPIC_API_KEY": "sk-ant-test",
        "KIE_API_KEY": "kie-test",
    }
    with patch.dict(os.environ, env, clear=False):
        from src.config import load_config

        cfg = load_config()
        assert cfg.elevenlabs_voice == "Adam"
        assert cfg.elevenlabs_stability == 0.5
```

- [ ] **Step 6: Run test to verify it fails**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_config.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'src.config'`

- [ ] **Step 7: Implement config.py**

Create `src/config.py`:

```python
import os
from dataclasses import dataclass

from dotenv import load_dotenv


@dataclass(frozen=True)
class Config:
    anthropic_api_key: str
    kie_api_key: str
    youtube_client_secret_path: str
    elevenlabs_voice: str
    elevenlabs_stability: float
    output_dir: str
    assets_dir: str


def load_config() -> Config:
    load_dotenv()
    return Config(
        anthropic_api_key=os.environ["ANTHROPIC_API_KEY"],
        kie_api_key=os.environ["KIE_API_KEY"],
        youtube_client_secret_path=os.environ.get("YOUTUBE_CLIENT_SECRET_PATH", "client_secret.json"),
        elevenlabs_voice=os.environ.get("ELEVENLABS_VOICE", "Adam"),
        elevenlabs_stability=float(os.environ.get("ELEVENLABS_STABILITY", "0.5")),
        output_dir=os.environ.get("OUTPUT_DIR", "./output"),
        assets_dir=os.environ.get("ASSETS_DIR", "./assets"),
    )
```

- [ ] **Step 8: Write the test for models**

Create `tests/test_models.py`:

```python
from src.models import TextOverlay, Script


def test_text_overlay_creation():
    overlay = TextOverlay(
        text="แอดถูก Reject!",
        appear_at_seconds=0,
        duration_seconds=5,
        position="top",
    )
    assert overlay.text == "แอดถูก Reject!"
    assert overlay.appear_at_seconds == 0
    assert overlay.position == "top"


def test_script_creation():
    script = Script(
        id="clip-2026-04-28",
        title="แอดถูก reject",
        youtube_title="แอดโดน Reject! {Ads Vance}",
        youtube_description="วิธีแก้...",
        youtube_tags=["facebook ads"],
        duration_target_seconds=50,
        voice_script="[confident] แอดคุณถูก reject...",
        image_prompt="infographic about...",
        thumbnail_prompt="thumbnail about...",
        text_overlays=[
            TextOverlay(text="Reject!", appear_at_seconds=0, duration_seconds=5, position="top")
        ],
    )
    assert script.id == "clip-2026-04-28"
    assert len(script.text_overlays) == 1


def test_script_voice_char_count():
    script = Script(
        id="test",
        title="t",
        youtube_title="t",
        youtube_description="t",
        youtube_tags=[],
        duration_target_seconds=50,
        voice_script="a" * 100,
        image_prompt="p",
        thumbnail_prompt="p",
        text_overlays=[],
    )
    assert script.voice_char_count <= 5000


def test_script_voice_char_count_too_long():
    script = Script(
        id="test",
        title="t",
        youtube_title="t",
        youtube_description="t",
        youtube_tags=[],
        duration_target_seconds=50,
        voice_script="a" * 5001,
        image_prompt="p",
        thumbnail_prompt="p",
        text_overlays=[],
    )
    assert script.voice_char_count > 5000
```

- [ ] **Step 9: Implement models.py**

Create `src/models.py`:

```python
from dataclasses import dataclass, field


@dataclass
class TextOverlay:
    text: str
    appear_at_seconds: float
    duration_seconds: float
    position: str  # "top", "bottom", "center"


@dataclass
class Script:
    id: str
    title: str
    youtube_title: str
    youtube_description: str
    youtube_tags: list[str]
    duration_target_seconds: int
    voice_script: str
    image_prompt: str
    thumbnail_prompt: str
    text_overlays: list[TextOverlay] = field(default_factory=list)

    @property
    def voice_char_count(self) -> int:
        return len(self.voice_script)
```

- [ ] **Step 10: Run all tests**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_config.py tests/test_models.py -v`
Expected: All PASS.

- [ ] **Step 11: Create output directories**

Run:
```bash
mkdir -p output/{scripts,images,audio,videos} data assets/fonts
```

- [ ] **Step 12: Download Noto Sans Thai font**

Run:
```bash
curl -L -o assets/fonts/NotoSansThai-Bold.ttf "https://github.com/google/fonts/raw/main/ofl/notosansthai/NotoSansThai%5Bwdth%2Cwght%5D.ttf"
```

- [ ] **Step 13: Create empty topic history**

Create `data/topic_history.json`:
```json
{
  "topics": []
}
```

- [ ] **Step 14: Commit**

```bash
git add requirements.txt .env.example src/ tests/ data/ assets/
git commit -m "feat: project setup with config, models, and directory structure"
```

---

## Task 2: Script Generator (Claude API)

**Files:**
- Create: `src/script_generator.py`
- Create: `tests/test_script_generator.py`

- [ ] **Step 1: Write the failing test**

Create `tests/test_script_generator.py`:

```python
import json
import pytest
from unittest.mock import AsyncMock, patch, MagicMock

from src.models import Script
from src.script_generator import ScriptGenerator


MOCK_CLAUDE_RESPONSE = json.dumps([
    {
        "id": "clip-2026-04-28",
        "title": "แอดถูก reject เพราะ Special Ad Category",
        "youtube_title": "แอดโดน Reject! แก้ง่ายๆ ใน 1 นาที {Ads Vance}",
        "youtube_description": "วิธีแก้...\n\nติดต่อ line id : @adsvance",
        "youtube_tags": ["facebook ads", "โฆษณาเฟสบุ๊ค"],
        "duration_target_seconds": 50,
        "voice_script": "[confident] แอดคุณถูก reject... แก้ได้ง่ายมาก",
        "image_prompt": "infographic about Special Ad Category...",
        "thumbnail_prompt": "thumbnail with bold Thai text...",
        "text_overlays": [
            {"text": "แอดถูก Reject!", "appear_at_seconds": 0, "duration_seconds": 5, "position": "top"}
        ]
    }
])


@pytest.mark.asyncio
async def test_generate_scripts_returns_list_of_scripts():
    mock_client = MagicMock()
    mock_message = MagicMock()
    mock_message.content = [MagicMock(text=MOCK_CLAUDE_RESPONSE)]
    mock_client.messages.create = AsyncMock(return_value=mock_message)

    generator = ScriptGenerator(api_key="test-key", topic_history_path="/tmp/test_topics.json")
    with patch.object(generator, "_get_client", return_value=mock_client):
        scripts = await generator.generate(count=1, category="account")

    assert len(scripts) == 1
    assert isinstance(scripts[0], Script)
    assert scripts[0].id == "clip-2026-04-28"
    assert "reject" in scripts[0].title


@pytest.mark.asyncio
async def test_generate_scripts_saves_to_topic_history(tmp_path):
    history_path = tmp_path / "topics.json"
    history_path.write_text('{"topics": []}')

    mock_client = MagicMock()
    mock_message = MagicMock()
    mock_message.content = [MagicMock(text=MOCK_CLAUDE_RESPONSE)]
    mock_client.messages.create = AsyncMock(return_value=mock_message)

    generator = ScriptGenerator(api_key="test-key", topic_history_path=str(history_path))
    with patch.object(generator, "_get_client", return_value=mock_client):
        await generator.generate(count=1, category="account")

    saved = json.loads(history_path.read_text())
    assert len(saved["topics"]) == 1
    assert saved["topics"][0]["title"] == "แอดถูก reject เพราะ Special Ad Category"


def test_build_prompt_contains_category():
    generator = ScriptGenerator(api_key="test-key", topic_history_path="/tmp/t.json")
    prompt = generator._build_prompt(count=7, category="payment", previous_topics=[])
    assert "payment" in prompt.lower() or "Payment" in prompt or "ชำระเงิน" in prompt


def test_build_prompt_contains_previous_topics():
    generator = ScriptGenerator(api_key="test-key", topic_history_path="/tmp/t.json")
    prompt = generator._build_prompt(count=7, category="account", previous_topics=["topic A", "topic B"])
    assert "topic A" in prompt
    assert "topic B" in prompt
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_script_generator.py -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'src.script_generator'`

- [ ] **Step 3: Implement script_generator.py**

Create `src/script_generator.py`:

```python
import json
from datetime import date, timedelta
from pathlib import Path

import anthropic

from src.models import Script, TextOverlay

CATEGORIES = {
    "account": "Account & Access — บัญชีถูกจำกัด, ยืนยันตัวตน, 2FA, เฟสปลิว, BM settings",
    "payment": "Payment & Billing — ชำระเงินไม่ผ่าน, เติมเงินไม่ขึ้น, เปลี่ยนบัตร, billing error",
    "campaign": "Campaign & Ads — แอดไม่วิ่ง, ถูก reject, targeting ผิด, ad review, Special Ad Category",
    "pixel": "Pixel & Tracking — ติดตั้ง pixel, custom conversion, event setup, Pixel not found",
}

WEEK_TO_CATEGORY = {0: "account", 1: "payment", 2: "campaign", 3: "pixel"}


class ScriptGenerator:
    def __init__(self, api_key: str, topic_history_path: str = "data/topic_history.json"):
        self._api_key = api_key
        self._topic_history_path = Path(topic_history_path)

    def _get_client(self) -> anthropic.AsyncAnthropic:
        return anthropic.AsyncAnthropic(api_key=self._api_key)

    def _load_topic_history(self) -> list[dict]:
        if not self._topic_history_path.exists():
            return []
        data = json.loads(self._topic_history_path.read_text())
        return data.get("topics", [])

    def _save_topic_history(self, topics: list[dict]) -> None:
        self._topic_history_path.parent.mkdir(parents=True, exist_ok=True)
        existing = self._load_topic_history()
        existing.extend(topics)
        self._topic_history_path.write_text(json.dumps({"topics": existing}, ensure_ascii=False, indent=2))

    def _get_recent_topics(self, days: int = 60) -> list[str]:
        cutoff = (date.today() - timedelta(days=days)).isoformat()
        history = self._load_topic_history()
        return [t["title"] for t in history if t.get("date", "") >= cutoff]

    def _build_prompt(self, count: int, category: str, previous_topics: list[str]) -> str:
        cat_desc = CATEGORIES.get(category, category)
        prev_section = ""
        if previous_topics:
            prev_list = "\n".join(f"- {t}" for t in previous_topics)
            prev_section = f"\n\nห้ามซ้ำกับหัวข้อที่เคยทำแล้ว:\n{prev_list}"

        return f"""คุณเป็นผู้เชี่ยวชาญ Facebook Ads ที่สร้างคอนเทนต์สอนทริคให้ช่อง YouTube "Ads Vance"
สร้าง {count} script สำหรับวิดีโอสั้น 30-60 วินาที ภาษาไทย

หมวดหมู่สัปดาห์นี้: {cat_desc}
{prev_section}

ตอบเป็น JSON array เท่านั้น ไม่ต้องมีข้อความอื่น

แต่ละ script ต้องมี fields เหล่านี้:
- "id": "clip-YYYY-MM-DD" (ใช้วันที่ที่จะ publish)
- "title": ชื่อหัวข้อภาษาไทย
- "youtube_title": ชื่อ YouTube ที่ดึงดูด ลงท้ายด้วย {{Ads Vance}} ไม่เกิน 70 ตัวอักษร
- "youtube_description": คำอธิบาย YouTube รวม "ติดต่อทีมงาน line id : @adsvance\\nเข้ากลุ่มเทเลแกรม: https://t.me/adsvancech"
- "youtube_tags": array ของ tags ภาษาไทยและอังกฤษ
- "duration_target_seconds": 30-60
- "voice_script": script เสียงพูดภาษาไทย ใช้ ... สำหรับพัก — สำหรับเน้น และ audio tags [confident] [friendly] [excited] [serious] เฉพาะจุดสำคัญ ลงท้ายด้วยเชิญทักไลน์ @adsvance
- "image_prompt": prompt สำหรับสร้าง infographic ภาษาอังกฤษ สไตล์ dark gradient background (#1a1a2e to #16213e) accent color #e94560 modern flat design มี Thai text labels มี step arrows มี brand text 'Ads Vance' bottom-right 16:9 ratio
- "thumbnail_prompt": prompt สำหรับสร้าง YouTube thumbnail ภาษาอังกฤษ bold Thai text dark dramatic background eye-catching 16:9
- "text_overlays": array ของ objects ที่มี "text", "appear_at_seconds", "duration_seconds", "position" (top/bottom/center) — 3-5 overlays ต่อ clip timing ตรงกับ voice_script"""

    async def generate(self, count: int = 7, category: str | None = None) -> list[Script]:
        if category is None:
            week_num = date.today().isocalendar()[1] % 4
            category = WEEK_TO_CATEGORY[week_num]

        previous_topics = self._get_recent_topics()
        prompt = self._build_prompt(count, category, previous_topics)

        client = self._get_client()
        message = await client.messages.create(
            model="claude-sonnet-4-6-20250514",
            max_tokens=8000,
            messages=[{"role": "user", "content": prompt}],
        )

        raw = message.content[0].text
        raw = raw.strip()
        if raw.startswith("```"):
            raw = raw.split("\n", 1)[1].rsplit("```", 1)[0]

        items = json.loads(raw)
        scripts = []
        for item in items:
            overlays = [
                TextOverlay(
                    text=o["text"],
                    appear_at_seconds=o["appear_at_seconds"],
                    duration_seconds=o["duration_seconds"],
                    position=o["position"],
                )
                for o in item.get("text_overlays", [])
            ]
            scripts.append(
                Script(
                    id=item["id"],
                    title=item["title"],
                    youtube_title=item["youtube_title"],
                    youtube_description=item["youtube_description"],
                    youtube_tags=item["youtube_tags"],
                    duration_target_seconds=item["duration_target_seconds"],
                    voice_script=item["voice_script"],
                    image_prompt=item["image_prompt"],
                    thumbnail_prompt=item["thumbnail_prompt"],
                    text_overlays=overlays,
                )
            )

        new_entries = [{"title": s.title, "date": date.today().isoformat(), "category": category} for s in scripts]
        self._save_topic_history(new_entries)

        return scripts


def get_current_category() -> str:
    week_num = date.today().isocalendar()[1] % 4
    return WEEK_TO_CATEGORY[week_num]
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_script_generator.py -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add src/script_generator.py tests/test_script_generator.py
git commit -m "feat: script generator using Claude API with topic rotation and dedup"
```

---

## Task 3: Image Generator (GPT Image 2 via Kie.ai)

**Files:**
- Create: `src/image_generator.py`
- Create: `tests/test_image_generator.py`

- [ ] **Step 1: Write the failing test**

Create `tests/test_image_generator.py`:

```python
import json
import pytest
from unittest.mock import AsyncMock, patch

from src.image_generator import ImageGenerator


@pytest.mark.asyncio
async def test_generate_image_calls_kie_api():
    mock_response_create = AsyncMock()
    mock_response_create.status_code = 200
    mock_response_create.json.return_value = {
        "code": 200,
        "data": {"taskId": "task-123"},
    }

    mock_response_status = AsyncMock()
    mock_response_status.status_code = 200
    mock_response_status.json.return_value = {
        "code": 200,
        "data": {
            "status": "completed",
            "output": {"image_url": "https://cdn.kie.ai/image-123.png"},
        },
    }

    mock_response_download = AsyncMock()
    mock_response_download.status_code = 200
    mock_response_download.content = b"fake-png-data"

    with patch("httpx.AsyncClient") as MockClient:
        client_instance = AsyncMock()
        client_instance.post = AsyncMock(return_value=mock_response_create)
        client_instance.get = AsyncMock(side_effect=[mock_response_status, mock_response_download])
        client_instance.__aenter__ = AsyncMock(return_value=client_instance)
        client_instance.__aexit__ = AsyncMock(return_value=False)
        MockClient.return_value = client_instance

        gen = ImageGenerator(api_key="kie-test", output_dir="/tmp/test_images")
        path = await gen.generate(
            prompt="test infographic",
            filename="test-infographic.png",
            aspect_ratio="16:9",
            resolution="2K",
        )

    assert path.endswith("test-infographic.png")
    client_instance.post.assert_called_once()
    call_body = client_instance.post.call_args[1]["json"]
    assert call_body["model"] == "gpt-image-2-text-to-image"
    assert call_body["input"]["prompt"] == "test infographic"
    assert call_body["input"]["aspect_ratio"] == "16:9"


def test_build_request_body():
    gen = ImageGenerator(api_key="kie-test", output_dir="/tmp")
    body = gen._build_request_body("my prompt", "16:9", "2K")
    assert body["model"] == "gpt-image-2-text-to-image"
    assert body["input"]["prompt"] == "my prompt"
    assert body["input"]["aspect_ratio"] == "16:9"
    assert body["input"]["resolution"] == "2K"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_image_generator.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement image_generator.py**

Create `src/image_generator.py`:

```python
import asyncio
from pathlib import Path

import httpx

KIE_API_BASE = "https://api.kie.ai/api/v1"


class ImageGenerator:
    def __init__(self, api_key: str, output_dir: str = "./output/images"):
        self._api_key = api_key
        self._output_dir = Path(output_dir)
        self._output_dir.mkdir(parents=True, exist_ok=True)

    def _build_request_body(self, prompt: str, aspect_ratio: str, resolution: str) -> dict:
        return {
            "model": "gpt-image-2-text-to-image",
            "input": {
                "prompt": prompt,
                "aspect_ratio": aspect_ratio,
                "resolution": resolution,
            },
        }

    async def _poll_task(self, client: httpx.AsyncClient, task_id: str, max_wait: int = 120) -> dict:
        for _ in range(max_wait // 3):
            resp = await client.get(
                f"{KIE_API_BASE}/jobs/getTaskDetail",
                params={"taskId": task_id},
                headers={"Authorization": f"Bearer {self._api_key}"},
            )
            data = resp.json()["data"]
            if data["status"] in ("completed", "success"):
                return data
            if data["status"] in ("failed", "error"):
                raise RuntimeError(f"Image generation failed: {data}")
            await asyncio.sleep(3)
        raise TimeoutError(f"Image generation timed out after {max_wait}s")

    async def generate(
        self,
        prompt: str,
        filename: str,
        aspect_ratio: str = "16:9",
        resolution: str = "2K",
    ) -> str:
        body = self._build_request_body(prompt, aspect_ratio, resolution)

        async with httpx.AsyncClient(timeout=30) as client:
            resp = await client.post(
                f"{KIE_API_BASE}/jobs/createTask",
                json=body,
                headers={"Authorization": f"Bearer {self._api_key}"},
            )
            task_id = resp.json()["data"]["taskId"]

            result = await self._poll_task(client, task_id)
            image_url = result["output"]["image_url"]

            img_resp = await client.get(image_url)
            out_path = self._output_dir / filename
            out_path.write_bytes(img_resp.content)

        return str(out_path)

    async def generate_pair(self, image_prompt: str, thumbnail_prompt: str, clip_id: str) -> tuple[str, str]:
        infographic_path = await self.generate(
            prompt=image_prompt,
            filename=f"{clip_id}-infographic.png",
        )
        thumbnail_path = await self.generate(
            prompt=thumbnail_prompt,
            filename=f"{clip_id}-thumbnail.png",
        )
        return infographic_path, thumbnail_path
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_image_generator.py -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add src/image_generator.py tests/test_image_generator.py
git commit -m "feat: image generator using GPT Image 2 via Kie.ai API"
```

---

## Task 4: Voice Generator (ElevenLabs V3 via Kie.ai)

**Files:**
- Create: `src/voice_generator.py`
- Create: `tests/test_voice_generator.py`

- [ ] **Step 1: Write the failing test**

Create `tests/test_voice_generator.py`:

```python
import pytest
from unittest.mock import AsyncMock, patch

from src.voice_generator import VoiceGenerator


@pytest.mark.asyncio
async def test_generate_voice_calls_kie_api():
    mock_response_create = AsyncMock()
    mock_response_create.status_code = 200
    mock_response_create.json.return_value = {
        "code": 200,
        "data": {"taskId": "voice-123"},
    }

    mock_response_status = AsyncMock()
    mock_response_status.status_code = 200
    mock_response_status.json.return_value = {
        "code": 200,
        "data": {
            "status": "completed",
            "output": {"audio_url": "https://cdn.kie.ai/voice-123.mp3"},
        },
    }

    mock_response_download = AsyncMock()
    mock_response_download.status_code = 200
    mock_response_download.content = b"fake-mp3-data"

    with patch("httpx.AsyncClient") as MockClient:
        client_instance = AsyncMock()
        client_instance.post = AsyncMock(return_value=mock_response_create)
        client_instance.get = AsyncMock(side_effect=[mock_response_status, mock_response_download])
        client_instance.__aenter__ = AsyncMock(return_value=client_instance)
        client_instance.__aexit__ = AsyncMock(return_value=False)
        MockClient.return_value = client_instance

        gen = VoiceGenerator(api_key="kie-test", output_dir="/tmp/test_audio")
        path = await gen.generate(
            text="[confident] สวัสดีครับ... นี่คือทริค",
            filename="test-voice.mp3",
            voice="Adam",
        )

    assert path.endswith("test-voice.mp3")
    call_body = client_instance.post.call_args[1]["json"]
    assert call_body["model"] == "elevenlabs/text-to-dialogue-v3"
    assert call_body["input"]["dialogue"][0]["voice"] == "Adam"
    assert call_body["input"]["language_code"] == "th"


def test_build_request_body():
    gen = VoiceGenerator(api_key="kie-test", output_dir="/tmp")
    body = gen._build_request_body("สวัสดี", "Adam", 0.5)
    assert body["model"] == "elevenlabs/text-to-dialogue-v3"
    assert body["input"]["dialogue"][0]["text"] == "สวัสดี"
    assert body["input"]["dialogue"][0]["voice"] == "Adam"
    assert body["input"]["language_code"] == "th"
    assert body["input"]["stability"] == 0.5
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_voice_generator.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement voice_generator.py**

Create `src/voice_generator.py`:

```python
import asyncio
from pathlib import Path

import httpx

KIE_API_BASE = "https://api.kie.ai/api/v1"


class VoiceGenerator:
    def __init__(
        self,
        api_key: str,
        output_dir: str = "./output/audio",
        default_voice: str = "Adam",
        default_stability: float = 0.5,
    ):
        self._api_key = api_key
        self._output_dir = Path(output_dir)
        self._output_dir.mkdir(parents=True, exist_ok=True)
        self._default_voice = default_voice
        self._default_stability = default_stability

    def _build_request_body(self, text: str, voice: str, stability: float) -> dict:
        return {
            "model": "elevenlabs/text-to-dialogue-v3",
            "input": {
                "dialogue": [{"text": text, "voice": voice}],
                "language_code": "th",
                "stability": stability,
            },
        }

    async def _poll_task(self, client: httpx.AsyncClient, task_id: str, max_wait: int = 120) -> dict:
        for _ in range(max_wait // 3):
            resp = await client.get(
                f"{KIE_API_BASE}/jobs/getTaskDetail",
                params={"taskId": task_id},
                headers={"Authorization": f"Bearer {self._api_key}"},
            )
            data = resp.json()["data"]
            if data["status"] in ("completed", "success"):
                return data
            if data["status"] in ("failed", "error"):
                raise RuntimeError(f"Voice generation failed: {data}")
            await asyncio.sleep(3)
        raise TimeoutError(f"Voice generation timed out after {max_wait}s")

    async def generate(
        self,
        text: str,
        filename: str,
        voice: str | None = None,
        stability: float | None = None,
    ) -> str:
        voice = voice or self._default_voice
        stability = stability if stability is not None else self._default_stability
        body = self._build_request_body(text, voice, stability)

        async with httpx.AsyncClient(timeout=30) as client:
            resp = await client.post(
                f"{KIE_API_BASE}/jobs/createTask",
                json=body,
                headers={"Authorization": f"Bearer {self._api_key}"},
            )
            task_id = resp.json()["data"]["taskId"]

            result = await self._poll_task(client, task_id)
            audio_url = result["output"]["audio_url"]

            audio_resp = await client.get(audio_url)
            out_path = self._output_dir / filename
            out_path.write_bytes(audio_resp.content)

        return str(out_path)
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_voice_generator.py -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add src/voice_generator.py tests/test_voice_generator.py
git commit -m "feat: voice generator using ElevenLabs V3 via Kie.ai API"
```

---

## Task 5: Video Assembler (FFmpeg)

**Files:**
- Create: `src/video_assembler.py`
- Create: `tests/test_video_assembler.py`

- [ ] **Step 1: Write the failing test**

Create `tests/test_video_assembler.py`:

```python
import subprocess
import pytest
from pathlib import Path
from unittest.mock import patch, MagicMock

from src.models import TextOverlay
from src.video_assembler import VideoAssembler


def test_build_ffmpeg_command():
    assembler = VideoAssembler(
        font_path="/tmp/font.ttf",
        assets_dir="/tmp/assets",
        output_dir="/tmp/output",
    )
    cmd = assembler._build_ffmpeg_command(
        image_path="/tmp/img.png",
        audio_path="/tmp/voice.mp3",
        output_path="/tmp/out.mp4",
        duration=50.0,
        text_overlays=[
            TextOverlay(text="Hello", appear_at_seconds=0, duration_seconds=5, position="top"),
            TextOverlay(text="World", appear_at_seconds=5, duration_seconds=10, position="bottom"),
        ],
    )
    assert cmd[0] == "ffmpeg"
    assert "-i" in cmd
    cmd_str = " ".join(cmd)
    assert "/tmp/img.png" in cmd_str
    assert "/tmp/voice.mp3" in cmd_str
    assert "/tmp/out.mp4" in cmd_str
    assert "zoompan" in cmd_str
    assert "drawtext" in cmd_str


def test_build_drawtext_filter():
    assembler = VideoAssembler(
        font_path="/tmp/font.ttf",
        assets_dir="/tmp/assets",
        output_dir="/tmp/output",
    )
    overlay = TextOverlay(text="ทดสอบ", appear_at_seconds=2, duration_seconds=5, position="bottom")
    filt = assembler._build_drawtext_filter(overlay, 0)
    assert "drawtext=" in filt
    assert "fontfile=/tmp/font.ttf" in filt
    assert "enable='between(t,2,7)'" in filt


@patch("subprocess.run")
def test_assemble_calls_ffmpeg(mock_run):
    mock_run.return_value = MagicMock(returncode=0)
    assembler = VideoAssembler(
        font_path="/tmp/font.ttf",
        assets_dir="/tmp/assets",
        output_dir="/tmp/output",
    )
    result = assembler.assemble(
        image_path="/tmp/img.png",
        audio_path="/tmp/voice.mp3",
        clip_id="test-clip",
        duration=50.0,
        text_overlays=[],
    )
    assert result.endswith("test-clip.mp4")
    mock_run.assert_called_once()
    call_args = mock_run.call_args[0][0]
    assert call_args[0] == "ffmpeg"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_video_assembler.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement video_assembler.py**

Create `src/video_assembler.py`:

```python
import subprocess
from pathlib import Path

from src.models import TextOverlay

FPS = 30
RESOLUTION = "1920x1080"


class VideoAssembler:
    def __init__(
        self,
        font_path: str = "assets/fonts/NotoSansThai-Bold.ttf",
        assets_dir: str = "assets",
        output_dir: str = "./output/videos",
    ):
        self._font_path = font_path
        self._assets_dir = Path(assets_dir)
        self._output_dir = Path(output_dir)
        self._output_dir.mkdir(parents=True, exist_ok=True)

    def _build_drawtext_filter(self, overlay: TextOverlay, index: int) -> str:
        end_time = overlay.appear_at_seconds + overlay.duration_seconds
        y_pos = {"top": "h*0.08", "bottom": "h*0.85", "center": "(h-th)/2"}
        y = y_pos.get(overlay.position, "h*0.85")

        escaped_text = overlay.text.replace("'", "'\\''").replace(":", "\\:")
        return (
            f"drawtext=fontfile={self._font_path}"
            f":text='{escaped_text}'"
            f":fontsize=48"
            f":fontcolor=white"
            f":borderw=2:bordercolor=black"
            f":x=(w-tw)/2:y={y}"
            f":enable='between(t,{overlay.appear_at_seconds},{end_time})'"
            f":alpha='if(lt(t-{overlay.appear_at_seconds},0.3),(t-{overlay.appear_at_seconds})/0.3,1)'"
        )

    def _build_ffmpeg_command(
        self,
        image_path: str,
        audio_path: str,
        output_path: str,
        duration: float,
        text_overlays: list[TextOverlay],
    ) -> list[str]:
        total_frames = int(duration * FPS)
        zoom_increment = 0.1 / total_frames

        zoompan = (
            f"zoompan=z='1+{zoom_increment}*in':x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)'"
            f":d={total_frames}:s=1920x1080:fps={FPS}"
        )

        filters = [zoompan]
        for i, overlay in enumerate(text_overlays):
            filters.append(self._build_drawtext_filter(overlay, i))

        filter_chain = ",".join(filters)

        return [
            "ffmpeg", "-y",
            "-loop", "1", "-i", image_path,
            "-i", audio_path,
            "-filter_complex", f"[0:v]{filter_chain}[v]",
            "-map", "[v]", "-map", "1:a",
            "-c:v", "libx264", "-preset", "medium", "-crf", "23",
            "-c:a", "aac", "-b:a", "128k",
            "-t", str(duration),
            "-pix_fmt", "yuv420p",
            "-shortest",
            output_path,
        ]

    def assemble(
        self,
        image_path: str,
        audio_path: str,
        clip_id: str,
        duration: float,
        text_overlays: list[TextOverlay],
    ) -> str:
        output_path = str(self._output_dir / f"{clip_id}.mp4")
        cmd = self._build_ffmpeg_command(image_path, audio_path, output_path, duration, text_overlays)

        subprocess.run(cmd, check=True, capture_output=True)
        return output_path
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_video_assembler.py -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add src/video_assembler.py tests/test_video_assembler.py
git commit -m "feat: video assembler with FFmpeg Ken Burns effect and text overlays"
```

---

## Task 6: QC Module

**Files:**
- Create: `src/qc.py`
- Create: `tests/test_qc.py`

- [ ] **Step 1: Write the failing test**

Create `tests/test_qc.py`:

```python
import json
import subprocess
import pytest
from pathlib import Path
from unittest.mock import patch, MagicMock

from src.qc import QCChecker, QCResult


def test_qc_pass(tmp_path):
    video = tmp_path / "test.mp4"
    video.write_bytes(b"x" * 2_000_000)

    probe_output = json.dumps({
        "streams": [
            {"codec_type": "video", "width": 1920, "height": 1080},
            {"codec_type": "audio"},
        ],
        "format": {"duration": "45.0"},
    })

    with patch("subprocess.run") as mock_run:
        mock_run.return_value = MagicMock(returncode=0, stdout=probe_output)
        checker = QCChecker()
        result = checker.check(str(video))

    assert result.passed is True
    assert len(result.issues) == 0


def test_qc_fail_too_short(tmp_path):
    video = tmp_path / "test.mp4"
    video.write_bytes(b"x" * 2_000_000)

    probe_output = json.dumps({
        "streams": [
            {"codec_type": "video", "width": 1920, "height": 1080},
            {"codec_type": "audio"},
        ],
        "format": {"duration": "10.0"},
    })

    with patch("subprocess.run") as mock_run:
        mock_run.return_value = MagicMock(returncode=0, stdout=probe_output)
        checker = QCChecker()
        result = checker.check(str(video))

    assert result.passed is False
    assert any("duration" in issue.lower() for issue in result.issues)


def test_qc_fail_no_audio(tmp_path):
    video = tmp_path / "test.mp4"
    video.write_bytes(b"x" * 2_000_000)

    probe_output = json.dumps({
        "streams": [
            {"codec_type": "video", "width": 1920, "height": 1080},
        ],
        "format": {"duration": "45.0"},
    })

    with patch("subprocess.run") as mock_run:
        mock_run.return_value = MagicMock(returncode=0, stdout=probe_output)
        checker = QCChecker()
        result = checker.check(str(video))

    assert result.passed is False
    assert any("audio" in issue.lower() for issue in result.issues)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_qc.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement qc.py**

Create `src/qc.py`:

```python
import json
import subprocess
from dataclasses import dataclass, field
from pathlib import Path


@dataclass
class QCResult:
    passed: bool
    issues: list[str] = field(default_factory=list)


class QCChecker:
    def __init__(
        self,
        min_duration: float = 25.0,
        max_duration: float = 120.0,
        min_file_size: int = 1_000_000,
        max_file_size: int = 50_000_000,
    ):
        self._min_duration = min_duration
        self._max_duration = max_duration
        self._min_file_size = min_file_size
        self._max_file_size = max_file_size

    def _probe(self, video_path: str) -> dict:
        result = subprocess.run(
            [
                "ffprobe", "-v", "quiet",
                "-print_format", "json",
                "-show_streams", "-show_format",
                video_path,
            ],
            capture_output=True,
            text=True,
        )
        return json.loads(result.stdout)

    def check(self, video_path: str) -> QCResult:
        issues = []
        path = Path(video_path)

        if not path.exists():
            return QCResult(passed=False, issues=["File does not exist"])

        file_size = path.stat().st_size
        if file_size < self._min_file_size:
            issues.append(f"File size too small: {file_size} bytes")
        if file_size > self._max_file_size:
            issues.append(f"File size too large: {file_size} bytes")

        probe = self._probe(video_path)

        streams = probe.get("streams", [])
        has_video = any(s["codec_type"] == "video" for s in streams)
        has_audio = any(s["codec_type"] == "audio" for s in streams)

        if not has_video:
            issues.append("No video stream found")
        if not has_audio:
            issues.append("No audio stream found")

        duration = float(probe.get("format", {}).get("duration", 0))
        if duration < self._min_duration:
            issues.append(f"Duration too short: {duration:.1f}s (min {self._min_duration}s)")
        if duration > self._max_duration:
            issues.append(f"Duration too long: {duration:.1f}s (max {self._max_duration}s)")

        if has_video:
            video_stream = next(s for s in streams if s["codec_type"] == "video")
            width = video_stream.get("width", 0)
            height = video_stream.get("height", 0)
            if width < 1920 or height < 1080:
                issues.append(f"Resolution too low: {width}x{height} (need 1920x1080)")

        return QCResult(passed=len(issues) == 0, issues=issues)
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_qc.py -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add src/qc.py tests/test_qc.py
git commit -m "feat: automated QC checker using ffprobe"
```

---

## Task 7: YouTube Uploader

**Files:**
- Create: `src/youtube_uploader.py`
- Create: `tests/test_youtube_uploader.py`

- [ ] **Step 1: Write the failing test**

Create `tests/test_youtube_uploader.py`:

```python
import pytest
from datetime import datetime, timezone, timedelta
from unittest.mock import MagicMock, patch

from src.youtube_uploader import YouTubeUploader


def test_build_upload_body():
    uploader = YouTubeUploader(client_secret_path="/tmp/secret.json")
    body = uploader._build_upload_body(
        title="แอดโดน Reject! {Ads Vance}",
        description="วิธีแก้...",
        tags=["facebook ads", "โฆษณา"],
        publish_at=datetime(2026, 4, 28, 17, 0, 0, tzinfo=timezone.utc),
    )
    assert body["snippet"]["title"] == "แอดโดน Reject! {Ads Vance}"
    assert body["snippet"]["tags"] == ["facebook ads", "โฆษณา"]
    assert body["snippet"]["categoryId"] == "27"
    assert body["status"]["privacyStatus"] == "private"
    assert "2026-04-28T17:00:00" in body["status"]["publishAt"]


def test_calculate_publish_time():
    uploader = YouTubeUploader(client_secret_path="/tmp/secret.json")
    bkk = timezone(timedelta(hours=7))
    publish_time = uploader._calculate_publish_time(
        target_date=datetime(2026, 4, 28, tzinfo=bkk)
    )
    assert publish_time.hour == 17
    assert publish_time.tzinfo == timezone.utc


@patch("src.youtube_uploader.build")
@patch("src.youtube_uploader.InstalledAppFlow")
def test_upload_calls_youtube_api(mock_flow, mock_build):
    mock_service = MagicMock()
    mock_insert = MagicMock()
    mock_insert.execute.return_value = {"id": "video-abc123"}
    mock_service.videos.return_value.insert.return_value = mock_insert
    mock_build.return_value = mock_service

    mock_creds = MagicMock()
    mock_creds.valid = True
    mock_flow.from_client_secrets_file.return_value.run_local_server.return_value = mock_creds

    uploader = YouTubeUploader(client_secret_path="/tmp/secret.json")
    uploader._credentials = mock_creds

    video_id = uploader.upload(
        video_path="/tmp/test.mp4",
        title="Test",
        description="Desc",
        tags=["tag1"],
        thumbnail_path="/tmp/thumb.png",
        publish_at=datetime(2026, 4, 28, 17, 0, 0, tzinfo=timezone.utc),
    )
    assert video_id == "video-abc123"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_youtube_uploader.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement youtube_uploader.py**

Create `src/youtube_uploader.py`:

```python
from datetime import datetime, timezone, timedelta
from pathlib import Path

from googleapiclient.discovery import build
from googleapiclient.http import MediaFileUpload
from google_auth_oauthlib.flow import InstalledAppFlow

SCOPES = ["https://www.googleapis.com/auth/youtube.upload"]
BKK_OFFSET = timedelta(hours=7)


class YouTubeUploader:
    def __init__(self, client_secret_path: str = "client_secret.json"):
        self._client_secret_path = client_secret_path
        self._credentials = None

    def authenticate(self) -> None:
        flow = InstalledAppFlow.from_client_secrets_file(self._client_secret_path, SCOPES)
        self._credentials = flow.run_local_server(port=0)

    def _get_service(self):
        if self._credentials is None:
            self.authenticate()
        return build("youtube", "v3", credentials=self._credentials)

    def _build_upload_body(
        self,
        title: str,
        description: str,
        tags: list[str],
        publish_at: datetime,
    ) -> dict:
        return {
            "snippet": {
                "title": title,
                "description": description,
                "tags": tags,
                "categoryId": "27",
                "defaultLanguage": "th",
            },
            "status": {
                "privacyStatus": "private",
                "publishAt": publish_at.strftime("%Y-%m-%dT%H:%M:%S.000Z"),
                "selfDeclaredMadeForKids": False,
            },
        }

    def _calculate_publish_time(self, target_date: datetime) -> datetime:
        midnight_bkk = target_date.replace(hour=0, minute=0, second=0, microsecond=0)
        return midnight_bkk.astimezone(timezone.utc)

    def upload(
        self,
        video_path: str,
        title: str,
        description: str,
        tags: list[str],
        thumbnail_path: str | None = None,
        publish_at: datetime | None = None,
    ) -> str:
        service = self._get_service()
        body = self._build_upload_body(title, description, tags, publish_at)

        media = MediaFileUpload(video_path, mimetype="video/mp4", resumable=True)
        request = service.videos().insert(
            part="snippet,status",
            body=body,
            media_body=media,
        )
        response = request.execute()
        video_id = response["id"]

        if thumbnail_path and Path(thumbnail_path).exists():
            service.thumbnails().set(
                videoId=video_id,
                media_body=MediaFileUpload(thumbnail_path, mimetype="image/png"),
            ).execute()

        return video_id
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_youtube_uploader.py -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add src/youtube_uploader.py tests/test_youtube_uploader.py
git commit -m "feat: YouTube uploader with scheduled publish at midnight BKK time"
```

---

## Task 8: Pipeline Orchestrator

**Files:**
- Create: `src/pipeline.py`
- Create: `tests/test_pipeline.py`

- [ ] **Step 1: Write the failing test**

Create `tests/test_pipeline.py`:

```python
import json
import pytest
from datetime import date, datetime, timezone, timedelta
from unittest.mock import AsyncMock, MagicMock, patch

from src.models import Script, TextOverlay
from src.pipeline import Pipeline


def _make_script(clip_id: str = "clip-2026-04-28") -> Script:
    return Script(
        id=clip_id,
        title="Test title",
        youtube_title="Test YT title {Ads Vance}",
        youtube_description="Test desc\n\nline: @adsvance",
        youtube_tags=["test"],
        duration_target_seconds=50,
        voice_script="[confident] Test script...",
        image_prompt="Test infographic prompt",
        thumbnail_prompt="Test thumbnail prompt",
        text_overlays=[
            TextOverlay(text="Test", appear_at_seconds=0, duration_seconds=5, position="top"),
        ],
    )


@pytest.mark.asyncio
async def test_produce_single_clip():
    mock_image_gen = AsyncMock()
    mock_image_gen.generate_pair = AsyncMock(return_value=("/tmp/infographic.png", "/tmp/thumb.png"))

    mock_voice_gen = AsyncMock()
    mock_voice_gen.generate = AsyncMock(return_value="/tmp/voice.mp3")

    mock_assembler = MagicMock()
    mock_assembler.assemble.return_value = "/tmp/output.mp4"

    mock_qc = MagicMock()
    mock_qc.check.return_value = MagicMock(passed=True, issues=[])

    pipeline = Pipeline(
        script_generator=AsyncMock(),
        image_generator=mock_image_gen,
        voice_generator=mock_voice_gen,
        video_assembler=mock_assembler,
        qc_checker=mock_qc,
        youtube_uploader=MagicMock(),
    )

    script = _make_script()
    result = await pipeline.produce_clip(script)

    assert result["video_path"] == "/tmp/output.mp4"
    assert result["thumbnail_path"] == "/tmp/thumb.png"
    assert result["qc_passed"] is True
    mock_image_gen.generate_pair.assert_called_once()
    mock_voice_gen.generate.assert_called_once()
    mock_assembler.assemble.assert_called_once()


@pytest.mark.asyncio
async def test_produce_weekly_batch():
    scripts = [_make_script(f"clip-{i}") for i in range(7)]

    mock_script_gen = AsyncMock()
    mock_script_gen.generate = AsyncMock(return_value=scripts)

    mock_image_gen = AsyncMock()
    mock_image_gen.generate_pair = AsyncMock(return_value=("/tmp/info.png", "/tmp/thumb.png"))

    mock_voice_gen = AsyncMock()
    mock_voice_gen.generate = AsyncMock(return_value="/tmp/voice.mp3")

    mock_assembler = MagicMock()
    mock_assembler.assemble.return_value = "/tmp/output.mp4"

    mock_qc = MagicMock()
    mock_qc.check.return_value = MagicMock(passed=True, issues=[])

    pipeline = Pipeline(
        script_generator=mock_script_gen,
        image_generator=mock_image_gen,
        voice_generator=mock_voice_gen,
        video_assembler=mock_assembler,
        qc_checker=mock_qc,
        youtube_uploader=MagicMock(),
    )

    results = await pipeline.produce_weekly()
    assert len(results) == 7
    mock_script_gen.generate.assert_called_once_with(count=7)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_pipeline.py -v`
Expected: FAIL — `ModuleNotFoundError`

- [ ] **Step 3: Implement pipeline.py**

Create `src/pipeline.py`:

```python
import asyncio
import json
import logging
from datetime import date, datetime, timezone, timedelta
from pathlib import Path

from src.models import Script
from src.script_generator import ScriptGenerator
from src.image_generator import ImageGenerator
from src.voice_generator import VoiceGenerator
from src.video_assembler import VideoAssembler
from src.qc import QCChecker
from src.youtube_uploader import YouTubeUploader

logger = logging.getLogger(__name__)
BKK = timezone(timedelta(hours=7))


class Pipeline:
    def __init__(
        self,
        script_generator: ScriptGenerator,
        image_generator: ImageGenerator,
        voice_generator: VoiceGenerator,
        video_assembler: VideoAssembler,
        qc_checker: QCChecker,
        youtube_uploader: YouTubeUploader,
    ):
        self._scripts = script_generator
        self._images = image_generator
        self._voices = voice_generator
        self._assembler = video_assembler
        self._qc = qc_checker
        self._uploader = youtube_uploader

    async def produce_clip(self, script: Script) -> dict:
        logger.info(f"Producing clip: {script.id} — {script.title}")

        infographic_path, thumbnail_path = await self._images.generate_pair(
            image_prompt=script.image_prompt,
            thumbnail_prompt=script.thumbnail_prompt,
            clip_id=script.id,
        )

        voice_path = await self._voices.generate(
            text=script.voice_script,
            filename=f"{script.id}-voice.mp3",
        )

        video_path = self._assembler.assemble(
            image_path=infographic_path,
            audio_path=voice_path,
            clip_id=script.id,
            duration=float(script.duration_target_seconds),
            text_overlays=script.text_overlays,
        )

        qc_result = self._qc.check(video_path)
        if not qc_result.passed:
            logger.warning(f"QC failed for {script.id}: {qc_result.issues}")

        return {
            "script": script,
            "video_path": video_path,
            "thumbnail_path": thumbnail_path,
            "voice_path": voice_path,
            "infographic_path": infographic_path,
            "qc_passed": qc_result.passed,
            "qc_issues": qc_result.issues,
        }

    async def produce_weekly(self, count: int = 7) -> list[dict]:
        scripts = await self._scripts.generate(count=count)
        results = []
        for script in scripts:
            result = await self.produce_clip(script)
            results.append(result)
        return results

    def upload_clip(self, clip_result: dict, publish_date: date) -> str | None:
        if not clip_result["qc_passed"]:
            logger.warning(f"Skipping upload for {clip_result['script'].id} — QC failed")
            return None

        script = clip_result["script"]
        publish_at = datetime(
            publish_date.year, publish_date.month, publish_date.day,
            tzinfo=BKK,
        ).astimezone(timezone.utc)

        video_id = self._uploader.upload(
            video_path=clip_result["video_path"],
            title=script.youtube_title,
            description=script.youtube_description,
            tags=script.youtube_tags,
            thumbnail_path=clip_result["thumbnail_path"],
            publish_at=publish_at,
        )
        logger.info(f"Uploaded {script.id} → youtube.com/watch?v={video_id}")
        return video_id

    def save_scripts(self, scripts: list[Script], output_dir: str = "output/scripts") -> None:
        out = Path(output_dir)
        out.mkdir(parents=True, exist_ok=True)
        for script in scripts:
            data = {
                "id": script.id,
                "title": script.title,
                "youtube_title": script.youtube_title,
                "youtube_description": script.youtube_description,
                "youtube_tags": script.youtube_tags,
                "duration_target_seconds": script.duration_target_seconds,
                "voice_script": script.voice_script,
                "image_prompt": script.image_prompt,
                "thumbnail_prompt": script.thumbnail_prompt,
                "text_overlays": [
                    {
                        "text": o.text,
                        "appear_at_seconds": o.appear_at_seconds,
                        "duration_seconds": o.duration_seconds,
                        "position": o.position,
                    }
                    for o in script.text_overlays
                ],
            }
            path = out / f"{script.id}.json"
            path.write_text(json.dumps(data, ensure_ascii=False, indent=2))
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m pytest tests/test_pipeline.py -v`
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add src/pipeline.py tests/test_pipeline.py
git commit -m "feat: pipeline orchestrator tying all modules together"
```

---

## Task 9: CLI Entry Points

**Files:**
- Create: `src/cli.py`

- [ ] **Step 1: Create CLI entry point**

Create `src/cli.py`:

```python
import argparse
import asyncio
import logging
import sys
from datetime import date, timedelta

from src.config import load_config
from src.script_generator import ScriptGenerator
from src.image_generator import ImageGenerator
from src.voice_generator import VoiceGenerator
from src.video_assembler import VideoAssembler
from src.qc import QCChecker
from src.youtube_uploader import YouTubeUploader
from src.pipeline import Pipeline

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger(__name__)


def build_pipeline() -> Pipeline:
    cfg = load_config()
    return Pipeline(
        script_generator=ScriptGenerator(
            api_key=cfg.anthropic_api_key,
        ),
        image_generator=ImageGenerator(
            api_key=cfg.kie_api_key,
            output_dir=f"{cfg.output_dir}/images",
        ),
        voice_generator=VoiceGenerator(
            api_key=cfg.kie_api_key,
            output_dir=f"{cfg.output_dir}/audio",
            default_voice=cfg.elevenlabs_voice,
            default_stability=cfg.elevenlabs_stability,
        ),
        video_assembler=VideoAssembler(
            font_path=f"{cfg.assets_dir}/fonts/NotoSansThai-Bold.ttf",
            assets_dir=cfg.assets_dir,
            output_dir=f"{cfg.output_dir}/videos",
        ),
        qc_checker=QCChecker(),
        youtube_uploader=YouTubeUploader(
            client_secret_path=cfg.youtube_client_secret_path,
        ),
    )


async def cmd_produce(args: argparse.Namespace) -> None:
    pipeline = build_pipeline()
    logger.info(f"Producing {args.count} clips...")
    results = await pipeline.produce_weekly(count=args.count)
    passed = sum(1 for r in results if r["qc_passed"])
    logger.info(f"Done: {passed}/{len(results)} passed QC")
    for r in results:
        status = "PASS" if r["qc_passed"] else f"FAIL: {r['qc_issues']}"
        logger.info(f"  {r['script'].id}: {status} → {r['video_path']}")


async def cmd_upload(args: argparse.Namespace) -> None:
    pipeline = build_pipeline()
    today = date.today()

    results = await pipeline.produce_weekly(count=args.days)
    for i, result in enumerate(results):
        publish_date = today + timedelta(days=i)
        video_id = pipeline.upload_clip(result, publish_date)
        if video_id:
            logger.info(f"Scheduled {result['script'].id} for {publish_date} → {video_id}")
        else:
            logger.warning(f"Skipped {result['script'].id} — QC failed")


def main() -> None:
    parser = argparse.ArgumentParser(description="Ads Vance Content Factory")
    sub = parser.add_subparsers(dest="command")

    produce = sub.add_parser("produce", help="Generate clips without uploading")
    produce.add_argument("--count", type=int, default=7, help="Number of clips")

    upload = sub.add_parser("upload", help="Generate and upload clips")
    upload.add_argument("--days", type=int, default=7, help="Days of clips to produce and schedule")

    args = parser.parse_args()

    if args.command == "produce":
        asyncio.run(cmd_produce(args))
    elif args.command == "upload":
        asyncio.run(cmd_upload(args))
    else:
        parser.print_help()
        sys.exit(1)


if __name__ == "__main__":
    main()
```

- [ ] **Step 2: Verify CLI runs**

Run: `cd /Users/jaochai/Code/video-fb && python3 -m src.cli --help`
Expected: Shows help with `produce` and `upload` subcommands.

- [ ] **Step 3: Commit**

```bash
git add src/cli.py
git commit -m "feat: CLI entry points for produce and upload commands"
```

---

## Task 10: Integration Test with Real APIs

- [ ] **Step 1: Create .env with real keys**

Copy `.env.example` to `.env` and fill in real API keys:
```bash
cp .env.example .env
# Edit .env with real keys
```

- [ ] **Step 2: Test single clip production**

Run:
```bash
cd /Users/jaochai/Code/video-fb && python3 -m src.cli produce --count 1
```

Expected: Generates 1 clip in `output/videos/` with:
- Script JSON in `output/scripts/`
- Infographic + thumbnail in `output/images/`
- Voice MP3 in `output/audio/`
- Final MP4 in `output/videos/`
- QC pass message in logs

- [ ] **Step 3: Review the generated clip**

Open the MP4 in a video player:
```bash
open output/videos/clip-*.mp4
```

Check:
- Image displays correctly
- Thai voice sounds natural
- Text overlays appear at correct times
- Ken Burns zoom effect is smooth
- Duration is 30-60 seconds

- [ ] **Step 4: Test YouTube upload (dry run)**

First authenticate with YouTube:
```bash
cd /Users/jaochai/Code/video-fb && python3 -c "
from src.youtube_uploader import YouTubeUploader
u = YouTubeUploader()
u.authenticate()
print('Authenticated successfully')
"
```

Then upload 1 test clip:
```bash
python3 -m src.cli upload --days 1
```

Expected: Video uploaded and scheduled for midnight.

- [ ] **Step 5: Commit integration test results**

```bash
git add .env.example data/
git commit -m "feat: verified integration with real APIs"
```

---

## Task 11: Cron Setup for Automation

- [ ] **Step 1: Create production script**

Create `scripts/weekly_produce.sh`:

```bash
#!/bin/bash
set -euo pipefail
cd /Users/jaochai/Code/video-fb
source .env
python3 -m src.cli upload --days 7
```

- [ ] **Step 2: Make executable**

```bash
chmod +x scripts/weekly_produce.sh
```

- [ ] **Step 3: Set up cron job**

Run: `crontab -e` and add:
```cron
# Every Monday at 10:00 Bangkok time (03:00 UTC) — produce and schedule 7 clips
0 3 * * 1 /Users/jaochai/Code/video-fb/scripts/weekly_produce.sh >> /Users/jaochai/Code/video-fb/logs/cron.log 2>&1
```

Create log directory:
```bash
mkdir -p /Users/jaochai/Code/video-fb/logs
```

- [ ] **Step 4: Test the cron script manually**

Run:
```bash
/Users/jaochai/Code/video-fb/scripts/weekly_produce.sh
```

Expected: 7 clips produced and scheduled for the next 7 days.

- [ ] **Step 5: Commit**

```bash
git add scripts/ logs/.gitkeep
git commit -m "feat: weekly cron automation script"
```

---

## Task Summary

| Task | Component | Key Outcome |
|------|-----------|------------|
| 1 | Project Setup | Config, models, dependencies, fonts |
| 2 | Script Generator | Claude API → 7 structured scripts/week |
| 3 | Image Generator | GPT Image 2 → infographic + thumbnail |
| 4 | Voice Generator | ElevenLabs V3 → Thai voiceover |
| 5 | Video Assembler | FFmpeg → MP4 with Ken Burns + text overlays |
| 6 | QC Module | Automated quality checks |
| 7 | YouTube Uploader | Scheduled upload at midnight BKK |
| 8 | Pipeline | Orchestrator tying all modules |
| 9 | CLI | `produce` and `upload` commands |
| 10 | Integration Test | Real API verification |
| 11 | Cron Automation | Weekly automated production |
