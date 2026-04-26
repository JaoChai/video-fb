from datetime import datetime, timezone, timedelta
from pathlib import Path

from googleapiclient.discovery import build
from googleapiclient.http import MediaFileUpload
from google_auth_oauthlib.flow import InstalledAppFlow

SCOPES = ["https://www.googleapis.com/auth/youtube.upload"]
BKK_OFFSET = timedelta(hours=7)


class YouTubeUploader:
    def __init__(self, client_secret_path: str = "client_secret.json"):
        self._client_secret_path = client_secret_path
        self._credentials = None

    def authenticate(self) -> None:
        flow = InstalledAppFlow.from_client_secrets_file(self._client_secret_path, SCOPES)
        self._credentials = flow.run_local_server(port=0)

    def _get_service(self):
        if self._credentials is None:
            self.authenticate()
        return build("youtube", "v3", credentials=self._credentials)

    def _build_upload_body(
        self,
        title: str,
        description: str,
        tags: list[str],
        publish_at: datetime,
    ) -> dict:
        return {
            "snippet": {
                "title": title,
                "description": description,
                "tags": tags,
                "categoryId": "27",
                "defaultLanguage": "th",
            },
            "status": {
                "privacyStatus": "private",
                "publishAt": publish_at.strftime("%Y-%m-%dT%H:%M:%S.000Z"),
                "selfDeclaredMadeForKids": False,
            },
        }

    def _calculate_publish_time(self, target_date: datetime) -> datetime:
        midnight_bkk = target_date.replace(hour=0, minute=0, second=0, microsecond=0)
        return midnight_bkk.astimezone(timezone.utc)

    def upload(
        self,
        video_path: str,
        title: str,
        description: str,
        tags: list[str],
        thumbnail_path: str | None = None,
        publish_at: datetime | None = None,
    ) -> str:
        service = self._get_service()
        body = self._build_upload_body(title, description, tags, publish_at)

        media = MediaFileUpload(video_path, mimetype="video/mp4", resumable=True)
        request = service.videos().insert(
            part="snippet,status",
            body=body,
            media_body=media,
        )
        response = request.execute()
        video_id = response["id"]

        if thumbnail_path and Path(thumbnail_path).exists():
            service.thumbnails().set(
                videoId=video_id,
                media_body=MediaFileUpload(thumbnail_path, mimetype="image/png"),
            ).execute()

        return video_id
