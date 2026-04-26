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
