import asyncio
from pathlib import Path

import httpx

KIE_API_BASE = "https://api.kie.ai/api/v1"


class ImageGenerator:
    def __init__(self, api_key: str, output_dir: str = "./output/images"):
        self._api_key = api_key
        self._output_dir = Path(output_dir)
        self._output_dir.mkdir(parents=True, exist_ok=True)

    def _build_request_body(self, prompt: str, aspect_ratio: str, resolution: str) -> dict:
        return {
            "model": "gpt-image-2-text-to-image",
            "input": {
                "prompt": prompt,
                "aspect_ratio": aspect_ratio,
                "resolution": resolution,
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
                raise RuntimeError(f"Image generation failed: {data}")
            await asyncio.sleep(3)
        raise TimeoutError(f"Image generation timed out after {max_wait}s")

    async def generate(
        self,
        prompt: str,
        filename: str,
        aspect_ratio: str = "16:9",
        resolution: str = "2K",
    ) -> str:
        body = self._build_request_body(prompt, aspect_ratio, resolution)

        async with httpx.AsyncClient(timeout=30) as client:
            resp = await client.post(
                f"{KIE_API_BASE}/jobs/createTask",
                json=body,
                headers={"Authorization": f"Bearer {self._api_key}"},
            )
            task_id = resp.json()["data"]["taskId"]

            result = await self._poll_task(client, task_id)
            image_url = result["output"]["image_url"]

            img_resp = await client.get(image_url)
            out_path = self._output_dir / filename
            out_path.write_bytes(img_resp.content)

        return str(out_path)

    async def generate_pair(self, image_prompt: str, thumbnail_prompt: str, clip_id: str) -> tuple[str, str]:
        infographic_path = await self.generate(
            prompt=image_prompt,
            filename=f"{clip_id}-infographic.png",
        )
        thumbnail_path = await self.generate(
            prompt=thumbnail_prompt,
            filename=f"{clip_id}-thumbnail.png",
        )
        return infographic_path, thumbnail_path
