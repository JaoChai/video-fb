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
