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
