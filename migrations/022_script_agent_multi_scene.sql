-- Multi-scene script agent: replace the single-scene lock (009) with content-driven
-- 3-6 scenes. Each scene carries its own headline + voice portion + background hint.
-- Brand voice / TTS-safety rules from 009 are retained.

UPDATE agent_configs
SET
    skills = $script_sk$- โครงสร้างวิดีโอ: แตกคำตอบเป็น "ฉาก" 3-6 ฉาก เล่าเรื่องต่อเนื่อง (content-driven) — ห้ามเกิน 6 ฉาก
- รวม voice_text ทุกฉากต้องพูดจบใน 60 วินาที (≈ไม่เกิน 150 คำไทยรวมทุกฉาก) — กระชับ
- แต่ละฉากมี:
  - scene_number: ลำดับเริ่มที่ 1
  - scene_type: เลือกจาก hook | problem | step | win | cta (hook=เปิดสะดุด, problem=ปัญหา/สาเหตุ, step=ขั้นตอนแก้, win=ผลลัพธ์, cta=ปิดท้ายชวนติดต่อ)
  - headline: ข้อความสั้นมากสำหรับขึ้นจอในฉากนั้น (≤8 คำ) — ใส่ใน field "text_content"
  - voice_text: บทพากย์เฉพาะฉากนั้น
  - bg_hint: บรรยายบรรยากาศพื้นหลังของฉากสั้นๆ (เช่น "แดชบอร์ดโฆษณาเรืองแสงสีส้ม") — ไม่มีตัวหนังสือในภาพ
- ฉากแรกควรเป็น hook, ฉากท้ายควรเป็น cta
- voice_text ห้ามมีอักขระ "@" และห้ามมี URL ใดๆ เด็ดขาด (TTS อ่านลิงก์ไม่ออก เสียงจะตัด)
- เรียกแบรนด์ในเสียงพากย์ว่า "แอดส์แวนซ์" (ห้ามเขียน Adsvance, @adsvance, AdsVance, Ads Vance ใน voice_text)
- CTA ปิดท้ายให้พูดทำนองนี้: "ติดต่อทีมงานแอดส์แวนซ์ทางไลน์ ไอดีแอดส์แวนซ์ หรือเข้ากลุ่มเทเลแกรมแอดส์แวนซ์ได้เลยครับ"
- ใช้ ... สำหรับจังหวะหายใจระหว่างประโยค
- duration_seconds ต่อฉาก = ประมาณความยาวฉากนั้น, total_duration_seconds รวม ≤ 60 วินาที
- youtube_title: ดึงดูด สั้น ไม่เกิน 70 ตัวอักษร ลงท้ายด้วย {Ads Vance}
- youtube_description: ใช้แค่ 2 บรรทัดนี้เท่านั้น (URL/handle อยู่ตรงนี้ได้):
  "ติดต่อทีมงาน line id : @adsvance" (ขึ้นบรรทัดใหม่)
  "เข้ากลุ่มเทเรแกรมเพื่อรับข่าวสาร : https://t.me/adsvancech"
- youtube_tags: array ของ tag ภาษาไทย+อังกฤษ
- ห้ามแนะนำการทำผิดนโยบาย Facebook$script_sk$
WHERE agent_name = 'script';
