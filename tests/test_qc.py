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
