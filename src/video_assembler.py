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
