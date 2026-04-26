import asyncio
from pathlib import Path

import httpx

KIE_API_BASE = "https://api.kie.ai/api/v1"


class VoiceGenerator:
    def __init__(
        self,
        api_key: str,
        output_dir: str = "./output/audio",
        default_voice: str = "Adam",
        default_stability: float = 0.5,
    ):
        self._api_key = api_key
        self._output_dir = Path(output_dir)
        self._output_dir.mkdir(parents=True, exist_ok=True)
        self._default_voice = default_voice
        self._default_stability = default_stability

    def _build_request_body(self, text: str, voice: str, stability: float) -> dict:
        return {
            "model": "elevenlabs/text-to-dialogue-v3",
            "input": {
                "dialogue": [{"text": text, "voice": voice}],
                "language_code": "th",
                "stability": stability,
            },
        }

    async def _poll_task(self, client: httpx.AsyncClient, task_id: str, max_wait: int = 120) -> dict:
        for _ in range(max_wait // 3):
            resp = await client.get(
                f"{KIE_API_BASE}/jobs/getTaskDetail",
                params={"taskId": task_id},
                headers={"Authorization": f"Bearer {self._api_key}"},
            )
            data = resp.json()["data"]
            if data["status"] in ("completed", "success"):
                return data
            if data["status"] in ("failed", "error"):
                raise RuntimeError(f"Voice generation failed: {data}")
            await asyncio.sleep(3)
        raise TimeoutError(f"Voice generation timed out after {max_wait}s")

    async def generate(
        self,
        text: str,
        filename: str,
        voice: str | None = None,
        stability: float | None = None,
    ) -> str:
        voice = voice or self._default_voice
        stability = stability if stability is not None else self._default_stability
        body = self._build_request_body(text, voice, stability)

        async with httpx.AsyncClient(timeout=30) as client:
            resp = await client.post(
                f"{KIE_API_BASE}/jobs/createTask",
                json=body,
                headers={"Authorization": f"Bearer {self._api_key}"},
            )
            task_id = resp.json()["data"]["taskId"]

            result = await self._poll_task(client, task_id)
            audio_url = result["output"]["audio_url"]

            audio_resp = await client.get(audio_url)
            out_path = self._output_dir / filename
            out_path.write_bytes(audio_resp.content)

        return str(out_path)
