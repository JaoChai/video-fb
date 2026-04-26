import os
from dataclasses import dataclass

from dotenv import load_dotenv


@dataclass(frozen=True)
class Config:
    anthropic_api_key: str
    kie_api_key: str
    youtube_client_secret_path: str
    elevenlabs_voice: str
    elevenlabs_stability: float
    output_dir: str
    assets_dir: str


def load_config() -> Config:
    load_dotenv()
    return Config(
        anthropic_api_key=os.environ["ANTHROPIC_API_KEY"],
        kie_api_key=os.environ["KIE_API_KEY"],
        youtube_client_secret_path=os.environ.get("YOUTUBE_CLIENT_SECRET_PATH", "client_secret.json"),
        elevenlabs_voice=os.environ.get("ELEVENLABS_VOICE", "Adam"),
        elevenlabs_stability=float(os.environ.get("ELEVENLABS_STABILITY", "0.5")),
        output_dir=os.environ.get("OUTPUT_DIR", "./output"),
        assets_dir=os.environ.get("ASSETS_DIR", "./assets"),
    )
