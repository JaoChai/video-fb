import pytest
from datetime import datetime, timezone, timedelta
from unittest.mock import MagicMock, patch

from src.youtube_uploader import YouTubeUploader


def test_build_upload_body():
    uploader = YouTubeUploader(client_secret_path="/tmp/secret.json")
    body = uploader._build_upload_body(
        title="แอดโดน Reject! {Ads Vance}",
        description="วิธีแก้...",
        tags=["facebook ads", "โฆษณา"],
        publish_at=datetime(2026, 4, 28, 17, 0, 0, tzinfo=timezone.utc),
    )
    assert body["snippet"]["title"] == "แอดโดน Reject! {Ads Vance}"
    assert body["snippet"]["tags"] == ["facebook ads", "โฆษณา"]
    assert body["snippet"]["categoryId"] == "27"
    assert body["status"]["privacyStatus"] == "private"
    assert "2026-04-28T17:00:00" in body["status"]["publishAt"]


def test_calculate_publish_time():
    uploader = YouTubeUploader(client_secret_path="/tmp/secret.json")
    bkk = timezone(timedelta(hours=7))
    publish_time = uploader._calculate_publish_time(
        target_date=datetime(2026, 4, 28, tzinfo=bkk)
    )
    assert publish_time.hour == 17
    assert publish_time.tzinfo == timezone.utc


@patch("src.youtube_uploader.MediaFileUpload")
@patch("src.youtube_uploader.build")
@patch("src.youtube_uploader.InstalledAppFlow")
def test_upload_calls_youtube_api(mock_flow, mock_build, mock_media_upload):
    mock_service = MagicMock()
    mock_insert = MagicMock()
    mock_insert.execute.return_value = {"id": "video-abc123"}
    mock_service.videos.return_value.insert.return_value = mock_insert
    mock_build.return_value = mock_service

    mock_creds = MagicMock()
    mock_creds.valid = True
    mock_flow.from_client_secrets_file.return_value.run_local_server.return_value = mock_creds

    mock_media = MagicMock()
    mock_media_upload.return_value = mock_media

    uploader = YouTubeUploader(client_secret_path="/tmp/secret.json")
    uploader._credentials = mock_creds

    video_id = uploader.upload(
        video_path="/tmp/test.mp4",
        title="Test",
        description="Desc",
        tags=["tag1"],
        thumbnail_path="/tmp/thumb.png",
        publish_at=datetime(2026, 4, 28, 17, 0, 0, tzinfo=timezone.utc),
    )
    assert video_id == "video-abc123"
