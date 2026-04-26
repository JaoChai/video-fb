import pytest
from unittest.mock import AsyncMock, patch

from src.voice_generator import VoiceGenerator


@pytest.mark.asyncio
async def test_generate_voice_calls_kie_api():
    from unittest.mock import MagicMock

    mock_response_create = MagicMock()
    mock_response_create.status_code = 200
    mock_response_create.json.return_value = {
        "code": 200,
        "data": {"taskId": "voice-123"},
    }

    mock_response_status = MagicMock()
    mock_response_status.status_code = 200
    mock_response_status.json.return_value = {
        "code": 200,
        "data": {
            "status": "completed",
            "output": {"audio_url": "https://cdn.kie.ai/voice-123.mp3"},
        },
    }

    mock_response_download = MagicMock()
    mock_response_download.status_code = 200
    mock_response_download.content = b"fake-mp3-data"

    with patch("src.voice_generator.httpx.AsyncClient") as MockClient:
        client_instance = AsyncMock()
        client_instance.post = AsyncMock(return_value=mock_response_create)
        client_instance.get = AsyncMock(side_effect=[mock_response_status, mock_response_download])
        client_instance.__aenter__ = AsyncMock(return_value=client_instance)
        client_instance.__aexit__ = AsyncMock(return_value=False)
        MockClient.return_value = client_instance

        gen = VoiceGenerator(api_key="kie-test", output_dir="/tmp/test_audio")
        path = await gen.generate(
            text="[confident] สวัสดีครับ... นี่คือทริค",
            filename="test-voice.mp3",
            voice="Adam",
        )

    assert path.endswith("test-voice.mp3")
    call_body = client_instance.post.call_args[1]["json"]
    assert call_body["model"] == "elevenlabs/text-to-dialogue-v3"
    assert call_body["input"]["dialogue"][0]["voice"] == "Adam"
    assert call_body["input"]["language_code"] == "th"


def test_build_request_body():
    gen = VoiceGenerator(api_key="kie-test", output_dir="/tmp")
    body = gen._build_request_body("สวัสดี", "Adam", 0.5)
    assert body["model"] == "elevenlabs/text-to-dialogue-v3"
    assert body["input"]["dialogue"][0]["text"] == "สวัสดี"
    assert body["input"]["dialogue"][0]["voice"] == "Adam"
    assert body["input"]["language_code"] == "th"
    assert body["input"]["stability"] == 0.5
