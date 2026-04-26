import json
import pytest
from unittest.mock import AsyncMock, patch

from src.image_generator import ImageGenerator


@pytest.mark.asyncio
async def test_generate_image_calls_kie_api():
    mock_response_create = AsyncMock()
    mock_response_create.status_code = 200
    mock_response_create.json = lambda: {
        "code": 200,
        "data": {"taskId": "task-123"},
    }

    mock_response_status = AsyncMock()
    mock_response_status.status_code = 200
    mock_response_status.json = lambda: {
        "code": 200,
        "data": {
            "status": "completed",
            "output": {"image_url": "https://cdn.kie.ai/image-123.png"},
        },
    }

    mock_response_download = AsyncMock()
    mock_response_download.status_code = 200
    mock_response_download.content = b"fake-png-data"

    with patch("src.image_generator.httpx.AsyncClient") as MockClient:
        client_instance = AsyncMock()
        client_instance.post = AsyncMock(return_value=mock_response_create)
        client_instance.get = AsyncMock(side_effect=[mock_response_status, mock_response_download])
        client_instance.__aenter__ = AsyncMock(return_value=client_instance)
        client_instance.__aexit__ = AsyncMock(return_value=False)
        MockClient.return_value = client_instance

        gen = ImageGenerator(api_key="kie-test", output_dir="/tmp/test_images")
        path = await gen.generate(
            prompt="test infographic",
            filename="test-infographic.png",
            aspect_ratio="16:9",
            resolution="2K",
        )

    assert path.endswith("test-infographic.png")
    client_instance.post.assert_called_once()
    call_body = client_instance.post.call_args[1]["json"]
    assert call_body["model"] == "gpt-image-2-text-to-image"
    assert call_body["input"]["prompt"] == "test infographic"
    assert call_body["input"]["aspect_ratio"] == "16:9"


def test_build_request_body():
    gen = ImageGenerator(api_key="kie-test", output_dir="/tmp")
    body = gen._build_request_body("my prompt", "16:9", "2K")
    assert body["model"] == "gpt-image-2-text-to-image"
    assert body["input"]["prompt"] == "my prompt"
    assert body["input"]["aspect_ratio"] == "16:9"
    assert body["input"]["resolution"] == "2K"
