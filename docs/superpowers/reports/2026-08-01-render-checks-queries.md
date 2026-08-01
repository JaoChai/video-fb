# คิวรีสถิติ render_checks (เฟส 1)

ใช้ตอบ 3 คำถามที่ตัดสินว่าเฟส 2/3 ควรทำอย่างไร รันหลัง deploy ~2 สัปดาห์ (~40 คลิป)
โปรเจกต์ Neon: `snowy-grass-75448787`

ทุกคิวรีในไฟล์นี้ **รันจริงแล้วบน Neon branch ทดสอบ** (2026-08-01) ไม่ใช่คิวรีที่เขียนลอยๆ

## 1. แต่ละด่าน fail กี่เปอร์เซ็นต์

`passed = false` หมายถึงด่านนั้นไม่ผ่านจริง — รวมกรณีที่ CLI จบด้วย exit 0 แต่หน้าเพจ
มีสัญญาณพัง (`[Browser:PAGEERROR]` = เรนเดอร์ค้างเป็นภาพนิ่ง) ไม่ใช่แค่ exit code

```sql
SELECT stage,
       count(*) AS runs,
       count(*) FILTER (WHERE NOT passed) AS failed,
       round(100.0 * count(*) FILTER (WHERE NOT passed) / count(*), 1) AS fail_pct
FROM render_checks
GROUP BY stage ORDER BY stage;
```

## 2. lint กินเวลาเท่าไร

```sql
SELECT stage,
       percentile_disc(0.5) WITHIN GROUP (ORDER BY duration_ms) AS median_ms,
       percentile_disc(0.95) WITHIN GROUP (ORDER BY duration_ms) AS p95_ms,
       max(duration_ms) AS max_ms
FROM render_checks GROUP BY stage ORDER BY stage;
```

## 3. finding ที่พบซ้ำ (แยก runner_error ออกจาก finding จริง)

```sql
SELECT stage,
       f.value #>> '{}' AS finding,
       count(*) AS hits
FROM render_checks rc, jsonb_array_elements(rc.findings) AS f(value)
WHERE f.value #>> '{}' NOT LIKE 'runner_error:%'
GROUP BY stage, finding
ORDER BY hits DESC LIMIT 20;
```

## 4. รัน CLI ไม่ได้บ่อยแค่ไหน (ต้องแยกจากข้อ 3 ก่อนเปิด gate ในเฟส 2)

```sql
SELECT stage, count(*) AS runner_errors
FROM render_checks rc, jsonb_array_elements(rc.findings) AS f(value)
WHERE f.value #>> '{}' LIKE 'runner_error:%'
GROUP BY stage ORDER BY stage;
```

## 5. ตรวจทันทีหลัง deploy — คลิปแรกต้องมี 3 แถว

```sql
SELECT stage, passed, duration_ms, findings, created_at
FROM render_checks ORDER BY created_at DESC LIMIT 3;
```

## เกณฑ์ตัดสินเฟส 2

- ข้อ 1 บอกว่าเปิด gate แล้วคลิปจะถูกบล็อกกี่ % — ถ้า lint fail เกิน ~20% ต้องแก้เทมเพลตก่อน ไม่ใช่เปิด gate
- ข้อ 2 บอกว่า lint คุ้มค่าที่จะรันก่อนเรนเดอร์ไหม (ถ้า median เกิน ~60 วินาที ต้องคิดใหม่)
- ข้อ 4 ต้องใกล้ 0 ก่อนเปิด gate — ไม่งั้นเราจะบล็อกคลิปเพราะ npx ล่ม

## หมายเหตุที่ค้นพบระหว่างทดสอบ migration

`internal/database/migrations.go:40` รันไฟล์ .sql ทั้งไฟล์ด้วย `pool.Exec(ctx, string(sql))`
ซึ่งรองรับหลายคำสั่งต่อไฟล์ได้ — ยืนยันจาก `045_auto_review.sql` (7 คำสั่ง) ที่ apply
บน prod สำเร็จเมื่อ 2026-07-02

แต่ **Neon MCP `run_sql` รันได้ทีละคำสั่งเท่านั้น** (prepared statement) ตอนทดสอบ
migration ด้วยมือผ่าน MCP จึงต้องแยกรันทีละคำสั่ง ข้อจำกัดนี้เป็นของเครื่องมือ
ไม่ใช่ของ migration runner
