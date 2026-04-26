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
