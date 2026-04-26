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
