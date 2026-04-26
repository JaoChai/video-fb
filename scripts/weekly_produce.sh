#!/bin/bash
set -euo pipefail
cd /Users/jaochai/Code/video-fb
source .env
python3 -m src.cli upload --days 7
