#!/usr/bin/env bash
# Ingest insider KB pack เข้า Ads Vance ผ่าน KB API
# ใช้: BASE_URL=... ./scripts/ingest_insider_kb.sh   (เพิ่ม API_TOKEN=... ถ้า endpoint มี auth)
set -euo pipefail

: "${BASE_URL:?need BASE_URL e.g. https://adsvance-v2.up.railway.app}"
API_TOKEN="${API_TOKEN:-}"

DIR="$(dirname "$0")/insider_kb_content"
AUTH=()
if [ -n "$API_TOKEN" ]; then
	AUTH=(-H "Authorization: Bearer $API_TOKEN")
fi

shopt -s nullglob
files=("$DIR"/*.txt)

for f in "${files[@]}"; do
	name="insider-$(basename "$f" .txt)"
	category=$(basename "$f" .txt | sed 's/^[0-9]*_//; s/_/-/g')
	content=$(cat "$f")
	echo "==> ingesting $name (category=$category)"

	resp=$(curl -sS -X POST "$BASE_URL/api/v1/knowledge/sources" \
		"${AUTH[@]}" \
		-H "Content-Type: application/json" \
		-d "$(jq -n --arg n "$name" --arg c "$category" --arg ct "$content" '{name:$n, category:$c, content:$ct}')")

	id=$(echo "$resp" | jq -r '.id // empty')
	if [ -z "$id" ]; then
		echo "  FAIL: no id in response: $resp" >&2
		continue
	fi
	echo "  created source $id, embedding..."
	curl -sS -X POST "$BASE_URL/api/v1/knowledge/sources/$id/embed" "${AUTH[@]}" | jq -r '.chunks // "embedded"'
done

echo "done. rollback: ลบ sources ที่ name LIKE 'insider-%' ผ่าน DELETE /api/v1/knowledge/sources/{id}"
