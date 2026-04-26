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
