#!/usr/bin/env bash
# Ingest insider KB pack เข้า Ads Vance ผ่าน KB API
# ใช้: BASE_URL=... API_TOKEN=... ./scripts/ingest_insider_kb.sh
# หมายเหตุ: re-run สร้างซ้ำ insider-* sources ใหม่ทุกครั้ง (ไม่มี dedup) —
# ลบของเก่าเองผ่าน DELETE /api/v1/knowledge/sources/{id} ก่อน re-run ถ้าไม่ต้องการซ้ำ
set -euo pipefail

: "${BASE_URL:?need BASE_URL e.g. https://adsvance-v2.up.railway.app}"
: "${API_TOKEN:?need API_TOKEN (KB API requires Bearer auth)}"

DIR="$(dirname "$0")/insider_kb_content"
AUTH=(-H "Authorization: Bearer $API_TOKEN")

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
		-d "$(jq -n --arg n "$name" --arg c "$category" --arg ct "$content" '{name:$n, category:$c, content:$ct}')") || true

	id=$(echo "$resp" | jq -r '.data.id // empty' 2>/dev/null) || true
	if [ -z "$id" ]; then
		echo "  FAIL: no id in response: $resp" >&2
		continue
	fi
	echo "  created source $id, embedding..."
	curl -sS -X POST "$BASE_URL/api/v1/knowledge/sources/$id/embed" "${AUTH[@]}" | jq -r '.data.chunks // "embedded"' || true
done

echo "done. rollback: ลบ sources ที่ name LIKE 'insider-%' ผ่าน DELETE /api/v1/knowledge/sources/{id}"
