-- เปลี่ยน fetch_analytics จากรายสัปดาห์เป็นรายวัน + ลบ ghost rows
-- ghost rows: platform != 'youtube' ที่ปัจจุบันไม่ได้ configure แล้ว และค่า views=likes=0

UPDATE schedules
SET cron_expression = '0 4 * * *', name = 'Daily Analytics'
WHERE action = 'fetch_analytics';

DELETE FROM clip_analytics
WHERE platform IN ('facebook', 'instagram', 'tiktok')
  AND views = 0
  AND likes = 0
  AND comments = 0;
