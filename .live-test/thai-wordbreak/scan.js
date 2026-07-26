// สแกนว่าคำไหนที่พจนานุกรมไทยของ Chromium (ICU) แบ่งผิด — ผลลัพธ์คือรายการ
// thaiLoanWords ใน internal/producer/thaiwrap.go
//
// ต้องรันใน Chromium เท่านั้น (Node ใช้ ICU คนละชุด ผลไม่ตรงกับตัวที่เรนเดอร์วิดีโอ):
//   1. เปิด Chrome → DevTools Console → วางไฟล์นี้ทั้งไฟล์ → Enter
//   2. หรือผ่าน Playwright MCP: browser_evaluate ด้วยเนื้อฟังก์ชัน scanThaiWordBreak
//
// คำถือว่า "พัง" เมื่อ Intl.Segmenter ซอยมันเป็นหลายชิ้นแทนที่จะเห็นเป็นคำเดียว
// ต้องทดสอบในบริบทประโยค เพราะ ICU แบ่งคำโดยดูข้อความรอบข้าง ไม่ใช่คำโดดๆ

const CANDIDATES = [
  // คำทับศัพท์ที่ใช้ในคอนเทนต์สาย Facebook Ads — เติมได้เรื่อยๆ แล้วรันซ้ำ
  'แอดมิน', 'แอดส์แวนซ์', 'ไอดีแอดส์แวนซ์', 'บีเอ็ม', 'เพจ', 'โดเมน', 'แคมเปญ', 'พิกเซล',
  'คอนเวอร์ชัน', 'ลิมิต', 'แบน', 'เทเลแกรม', 'ซีบีโอ', 'เอบีโอ', 'โอทีพี', 'แชทบอท',
  'พอร์ตโฟลิโอ', 'บิสซิเนส', 'เวอริฟาย', 'รีวิว', 'ฟีด', 'รีช', 'เอนเกจเมนต์', 'ทาร์เก็ต',
  'รีมาร์เก็ตติ้ง', 'บัดเจ็ต', 'สเกล', 'ครีเอทีฟ', 'อัลกอริทึม', 'เมต้า', 'เฟซบุ๊ก', 'เฟสบุ๊ก',
  'อินสตาแกรม', 'ติ๊กต็อก', 'เพย์เมนต์', 'เครดิต', 'สลิป', 'ดรอปชิป', 'เอเจนซี่', 'คลิก',
  'โปรไฟล์', 'ฟาร์ม', 'วอร์มอัพ', 'เซิร์ฟเวอร์', 'ไอพี', 'คุกกี้', 'พร็อกซี', 'คอมเมนต์',
  'แอคเคาท์', 'แอคเคาน์', 'ไอดี', 'บล็อก', 'ล็อก', 'อัปเดต', 'ออโต้', 'แมนวล', 'รีเซ็ต',
  'ดีเลย์', 'โทเคน', 'แดชบอร์ด', 'อินไซต์', 'เอนจิน', 'สปอนเซอร์', 'ทรัสต์',
];

function scanThaiWordBreak(candidates) {
  const seg = new Intl.Segmenter('th', { granularity: 'word' });
  const broken = [], safe = [];
  for (const w of candidates) {
    const ctx = 'ที่คุณใช้' + w + 'อยู่ตอนนี้';
    const pieces = [...seg.segment(ctx)].map(s => s.segment);
    (pieces.includes(w) ? safe : broken).push({ word: w, pieces: pieces.join('|') });
  }
  broken.sort((a, b) => [...b.word].length - [...a.word].length);
  return {
    goLiteral: broken.map(b => `\t"${b.word}",`).join('\n'),
    brokenCount: broken.length,
    broken: broken.map(b => `${b.word} → ${b.pieces}`),
    safe: safe.map(s => s.word),
  };
}

// คัดลอกค่า goLiteral ไปวางใน thaiLoanWords (เรียงยาว→สั้นให้แล้ว)
scanThaiWordBreak(CANDIDATES);
