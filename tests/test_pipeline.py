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
