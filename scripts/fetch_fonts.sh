#!/usr/bin/env bash
# ดาวน์โหลดฟอนต์ OFL ที่ใช้ในธีม (vendor เป็น local .ttf — render offline)
set -euo pipefail
DEST="internal/producer/assets/fonts"
mkdir -p "$DEST"
base="https://raw.githubusercontent.com/google/fonts/main"
declare -A FONTS=(
  ["Kanit-Bold.ttf"]="$base/ofl/kanit/Kanit-Bold.ttf"
  ["Kanit-ExtraBold.ttf"]="$base/ofl/kanit/Kanit-ExtraBold.ttf"
  ["Kanit-Black.ttf"]="$base/ofl/kanit/Kanit-Black.ttf"
  ["Prompt-SemiBold.ttf"]="$base/ofl/prompt/Prompt-SemiBold.ttf"
  ["Prompt-Bold.ttf"]="$base/ofl/prompt/Prompt-Bold.ttf"
  ["Prompt-ExtraBold.ttf"]="$base/ofl/prompt/Prompt-ExtraBold.ttf"
  ["IBMPlexSansThai-Medium.ttf"]="$base/ofl/ibmplexsansthai/IBMPlexSansThai-Medium.ttf"
  ["IBMPlexSansThai-SemiBold.ttf"]="$base/ofl/ibmplexsansthai/IBMPlexSansThai-SemiBold.ttf"
)
for name in "${!FONTS[@]}"; do
  echo "→ $name"
  curl -fsSL "${FONTS[$name]}" -o "$DEST/$name"
done
echo "done: $(ls -1 "$DEST" | wc -l) fonts"
