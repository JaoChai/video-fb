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
