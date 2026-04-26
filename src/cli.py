import argparse
import asyncio
import logging
import sys
from datetime import date, timedelta

from src.config import load_config
from src.script_generator import ScriptGenerator
from src.image_generator import ImageGenerator
from src.voice_generator import VoiceGenerator
from src.video_assembler import VideoAssembler
from src.qc import QCChecker
from src.youtube_uploader import YouTubeUploader
from src.pipeline import Pipeline

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger(__name__)


def build_pipeline() -> Pipeline:
    cfg = load_config()
    return Pipeline(
        script_generator=ScriptGenerator(
            api_key=cfg.anthropic_api_key,
        ),
        image_generator=ImageGenerator(
            api_key=cfg.kie_api_key,
            output_dir=f"{cfg.output_dir}/images",
        ),
        voice_generator=VoiceGenerator(
            api_key=cfg.kie_api_key,
            output_dir=f"{cfg.output_dir}/audio",
            default_voice=cfg.elevenlabs_voice,
            default_stability=cfg.elevenlabs_stability,
        ),
        video_assembler=VideoAssembler(
            font_path=f"{cfg.assets_dir}/fonts/NotoSansThai-Bold.ttf",
            assets_dir=cfg.assets_dir,
            output_dir=f"{cfg.output_dir}/videos",
        ),
        qc_checker=QCChecker(),
        youtube_uploader=YouTubeUploader(
            client_secret_path=cfg.youtube_client_secret_path,
        ),
    )


async def cmd_produce(args: argparse.Namespace) -> None:
    pipeline = build_pipeline()
    logger.info(f"Producing {args.count} clips...")
    results = await pipeline.produce_weekly(count=args.count)
    passed = sum(1 for r in results if r["qc_passed"])
    logger.info(f"Done: {passed}/{len(results)} passed QC")
    for r in results:
        status = "PASS" if r["qc_passed"] else f"FAIL: {r['qc_issues']}"
        logger.info(f"  {r['script'].id}: {status} → {r['video_path']}")


async def cmd_upload(args: argparse.Namespace) -> None:
    pipeline = build_pipeline()
    today = date.today()

    results = await pipeline.produce_weekly(count=args.days)
    for i, result in enumerate(results):
        publish_date = today + timedelta(days=i)
        video_id = pipeline.upload_clip(result, publish_date)
        if video_id:
            logger.info(f"Scheduled {result['script'].id} for {publish_date} → {video_id}")
        else:
            logger.warning(f"Skipped {result['script'].id} — QC failed")


def main() -> None:
    parser = argparse.ArgumentParser(description="Ads Vance Content Factory")
    sub = parser.add_subparsers(dest="command")

    produce = sub.add_parser("produce", help="Generate clips without uploading")
    produce.add_argument("--count", type=int, default=7, help="Number of clips")

    upload = sub.add_parser("upload", help="Generate and upload clips")
    upload.add_argument("--days", type=int, default=7, help="Days of clips to produce and schedule")

    args = parser.parse_args()

    if args.command == "produce":
        asyncio.run(cmd_produce(args))
    elif args.command == "upload":
        asyncio.run(cmd_upload(args))
    else:
        parser.print_help()
        sys.exit(1)


if __name__ == "__main__":
    main()
