# แผน implementation: แก้คำไทยถูกหั่นกลางคำในแคปชั่น

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** หยุดไม่ให้คำภาษาไทยถูกหั่นกลางคำในแคปชั่นและข้อความบนจอ ทั้งที่เกิดจากโค้ดฝั่ง Go และจากพจนานุกรมไทยของ Chromium

**Architecture:** สองการเปลี่ยนแปลงที่แยกจากกันสิ้นเชิง — (1) ลบการหั่นวลีดิบที่ 42 ตัวอักษรออกจาก `splitCaptionPhrases()` ปล่อยให้ Chromium ขึ้นบรรทัดเอง (2) เพิ่มหน่วยใหม่ `GuardLoanWords()` ที่แทรก zero-width space คร่อมคำทับศัพท์ที่ ICU ไม่รู้จัก แล้วเรียกจากสองจุดปลายทาง คือแคปชั่นและข้อความบนจอ ทั้งคู่อยู่หลังเส้นทาง TTS แยกไปแล้ว

**Tech Stack:** Go 1.x (stdlib `strings` เท่านั้น ไม่เพิ่ม dependency) · เทมเพลต `html/template` เดิม · เทสต์ด้วย `go test`

**สเปคอ้างอิง:** `docs/superpowers/specs/2026-07-26-thai-word-break-captions-design.md`

## Global Constraints

- ไม่เพิ่ม dependency ใดๆ — ใช้ stdlib เท่านั้น
- ไม่ใส่ feature flag — revert commit ได้ทันทีถ้ามีปัญหา
- **ห้ามให้ ZWSP (`​`) หลุดไปเส้นทาง TTS** — `voice_text` เดียวกันถูกใช้ทั้งทำเสียงและทำแคปชั่น
- **ห้ามแตะ `Panel.Items[].Label` และ `Panel.Field.Value`** — ต้องคงค่าเดิมแบบ byte-for-byte เพื่อให้ผ่าน gate `agent.UIVocabViolations`
- ไม่แตะ visual QA, การเลือกวลีของ TTS, layout อื่น หรือคลิปเก่าที่เผยแพร่แล้ว
- คอมเมนต์ในโค้ดเขียนสไตล์เดียวกับไฟล์รอบข้าง (อธิบาย "ทำไม" ไม่ใช่ "ทำอะไร") ผสมไทย/อังกฤษได้ตามที่ไฟล์นั้นใช้อยู่
- ทุก task จบด้วย `go build ./... && go test ./internal/producer/...` ผ่าน

## File Structure

| ไฟล์ | สถานะ | หน้าที่ |
|---|---|---|
| `internal/producer/thaiwrap.go` | สร้างใหม่ | รายการคำทับศัพท์ + `GuardLoanWords()` — หน่วยเดียว หน้าที่เดียว ไม่มี dependency |
| `internal/producer/thaiwrap_test.go` | สร้างใหม่ | เทสต์ของหน่วยข้างบน |
| `internal/producer/captions.go` | แก้ | ลบ hard-split + `safeCut()` · เรียก `GuardLoanWords` ตอนประกอบ segment |
| `internal/producer/captions_test.go` | แก้ | เพิ่มเทสต์ token ไม่ถูกหั่น + ZWSP อยู่ในแคปชั่น |
| `internal/producer/scene_adapter.go` | แก้ | เรียก `guardSceneContent()` ก่อน `return c` ใน `buildSceneContent()` |
| `internal/producer/scene_adapter_test.go` | แก้ | เทสต์ว่าข้อความบนจอถูก guard และ `Panel.Items` ไม่ถูกแตะ |

---

### Task 1: หน่วย `GuardLoanWords` + รายการคำทับศัพท์

**Files:**
- Create: `internal/producer/thaiwrap.go`
- Test: `internal/producer/thaiwrap_test.go`

**Interfaces:**
- Consumes: ไม่มี (หน่วยตั้งต้น ใช้แค่ `strings` จาก stdlib)
- Produces: `func GuardLoanWords(s string) string` · `const zwsp = "​"` · `var thaiLoanWords []string`

- [ ] **Step 1: เขียนเทสต์ที่ต้องล้มเหลว**

สร้าง `internal/producer/thaiwrap_test.go`:

```go
package producer

import (
	"strings"
	"testing"
)

// วิธีอ่านเทสต์ชุดนี้: `⟦` แทน zero-width space เพื่อให้เห็นตำแหน่งที่คาดหวัง
func vis(s string) string { return strings.ReplaceAll(s, zwsp, "⟦") }

func TestGuardLoanWords_WrapsKnownWords(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			name: "คำทับศัพท์กลางประโยค",
			in:   "เจ้าของเก่าเคลมคืนได้ทั้งที่คุณคุมแอดมินอยู่",
			want: "เจ้าของเก่าเคลมคืนได้ทั้งที่คุณคุม" + zwsp + "แอดมิน" + zwsp + "อยู่",
		},
		{
			name: "คำประกอบต้องชนะคำย่อย",
			in:   "ทักไลน์ไอดีแอดส์แวนซ์ได้เลย",
			want: "ทักไลน์" + zwsp + "ไอดีแอดส์แวนซ์" + zwsp + "ได้เลย",
		},
		{
			name: "ไม่มีคำในรายการ = คืนค่าเดิม",
			in:   "เปลี่ยนโดเมนแล้วอัดงบต่อทันที",
			want: "เปลี่ยนโดเมนแล้วอัดงบต่อทันที",
		},
		{
			name: "สตริงว่าง",
			in:   "",
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := GuardLoanWords(c.in); got != c.want {
				t.Errorf("GuardLoanWords(%q)\n got: %s\nwant: %s", c.in, vis(got), vis(c.want))
			}
		})
	}
}

// เรียกซ้ำต้องได้ผลเดิม — จุดเรียกทั้งสองแห่งอาจทับกันได้ในอนาคต
func TestGuardLoanWords_Idempotent(t *testing.T) {
	in := "ทีมงานเทเลแกรมตอบเร็วมากครับ"
	once := GuardLoanWords(in)
	twice := GuardLoanWords(once)
	if once != twice {
		t.Errorf("เรียกซ้ำแล้วผลเปลี่ยน\n1 ครั้ง: %s\n2 ครั้ง: %s", vis(once), vis(twice))
	}
	if strings.Contains(twice, zwsp+zwsp) {
		t.Errorf("มี ZWSP ซ้อนกันหลังเรียกซ้ำ: %s", vis(twice))
	}
}

// ตัวอักษรจริงต้องไม่หายและไม่สลับที่ — ZWSP เป็นส่วนเกินที่ถอดออกแล้วต้องได้ต้นฉบับ
func TestGuardLoanWords_PreservesOriginalText(t *testing.T) {
	in := "ไอดีแอดส์แวนซ์กับเฟซบุ๊กและเมต้าคือคนละอย่างกัน"
	got := strings.ReplaceAll(GuardLoanWords(in), zwsp, "")
	if got != in {
		t.Errorf("ข้อความเพี้ยนหลังถอด ZWSP\n got: %q\nwant: %q", got, in)
	}
}

// HTML ที่ highlightTitleStr ใส่ไว้ต้องไม่ถูกทำลาย (คำในรายการเป็นอักษรไทยล้วน
// จึงไม่มีทางตรงกับ syntax ของแท็กซึ่งเป็น ASCII)
func TestGuardLoanWords_LeavesHTMLTagsIntact(t *testing.T) {
	in := `เปิด<span class="acc">แอดมิน</span>ให้ครบ`
	got := GuardLoanWords(in)
	if !strings.Contains(got, `<span class="acc">`) || !strings.Contains(got, `</span>`) {
		t.Errorf("แท็กถูกทำลาย: %s", vis(got))
	}
	if strings.Contains(got, "<"+zwsp) || strings.Contains(got, zwsp+">") {
		t.Errorf("แทรก ZWSP เข้าไปในแท็ก: %s", vis(got))
	}
}

// รายการต้องเรียงจากยาวไปสั้นเสมอ ไม่งั้น "แอดส์แวนซ์" จะไปชนะ "ไอดีแอดส์แวนซ์"
// เทสต์นี้มีไว้กันคนเพิ่มคำใหม่ต่อท้ายรายการโดยไม่ดูลำดับ
func TestThaiLoanWords_SortedLongestFirst(t *testing.T) {
	for i := 1; i < len(thaiLoanWords); i++ {
		prev := len([]rune(thaiLoanWords[i-1]))
		cur := len([]rune(thaiLoanWords[i]))
		if cur > prev {
			t.Errorf("รายการเรียงผิดที่ตำแหน่ง %d: %q (%d ตัว) ยาวกว่า %q (%d ตัว) ที่อยู่ก่อนหน้า",
				i, thaiLoanWords[i], cur, thaiLoanWords[i-1], prev)
		}
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าล้มเหลว**

Run: `go test ./internal/producer/ -run 'GuardLoanWords|ThaiLoanWords' -v`
Expected: FAIL — `undefined: GuardLoanWords`, `undefined: zwsp`, `undefined: thaiLoanWords`

- [ ] **Step 3: เขียน implementation**

สร้าง `internal/producer/thaiwrap.go`:

```go
package producer

import "strings"

// zwsp คือ zero-width space (U+200B) — อักขระกว้างศูนย์ที่บอก Chromium ว่า
// ตรงนี้ขึ้นบรรทัดใหม่ได้ ไม่มีผลต่อความกว้างหรือความสูงของกล่อง
const zwsp = "​"

// thaiLoanWords คือคำทับศัพท์ที่พจนานุกรมไทยของ Chromium (ICU) แบ่งผิด และพบจริง
// ในเนื้อหา 982 ซีน. ที่ต้องประกบด้วย zwsp เพราะคำที่ ICU ไม่รู้จัก "ลาม" ไปทำให้
// คำไทยข้างเคียงถูกแบ่งผิดด้วย — "แอดมินอยู่" ถูกแบ่งเป็น "แอด|มิ|นอ|ยู่" แล้วขึ้น
// บรรทัดกลางคำ "อยู่" ทั้งที่ตัวปัญหาคือ "แอดมิน".
//
// เรียงจากยาวไปสั้นเสมอ เพื่อให้คำประกอบชนะคำย่อย (ไอดีแอดส์แวนซ์ ต้องมาก่อน
// แอดส์แวนซ์ และ ไอดี) — TestThaiLoanWords_SortedLongestFirst บังคับกติกานี้ไว้
var thaiLoanWords = []string{
	"ไอดีแอดส์แวนซ์",
	"พอร์ตโฟลิโอ",
	"คอนเวอร์ชัน",
	"อัลกอริทึม",
	"แอดส์แวนซ์",
	"เพย์เมนต์",
	"แดชบอร์ด",
	"คอมเมนต์",
	"เทเลแกรม",
	"แอคเคาน์",
	"เอเจนซี่",
	"แชทบอท",
	"เฟซบุ๊ก",
	"เฟสบุ๊ก",
	"แอดมิน",
	"ซีบีโอ",
	"โอทีพี",
	"เอบีโอ",
	"บีเอ็ม",
	"เมต้า",
	"ลิมิต",
	"ไอดี",
	"ไอพี",
	"ฟีด",
	"รีช",
}

// GuardLoanWords แทรก zwsp คร่อมทุกคำในรายการที่พบในสตริง เพื่อบังคับให้
// Chromium ขึ้นบรรทัดที่ขอบคำจริงแทนที่จะเดาเอง.
//
// ล้าง zwsp เดิมทิ้งก่อนเสมอ ทำให้เรียกซ้ำได้โดยไม่สะสมอักขระ — ระบบไม่ได้ใช้
// zwsp เพื่อจุดประสงค์อื่น การล้างจึงปลอดภัย.
func GuardLoanWords(s string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, zwsp, "")

	var b strings.Builder
	b.Grow(len(s) + 32)
	for i := 0; i < len(s); {
		w := loanWordAt(s, i)
		if w == "" {
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteString(zwsp)
		b.WriteString(w)
		b.WriteString(zwsp)
		i += len(w)
	}
	return b.String()
}

// loanWordAt คืนคำในรายการที่ขึ้นต้นตรงตำแหน่ง i พอดี. รายการเรียงยาวไปสั้นอยู่แล้ว
// ผลที่ได้จึงเป็นคำที่ยาวที่สุดเสมอ.
func loanWordAt(s string, i int) string {
	for _, w := range thaiLoanWords {
		if strings.HasPrefix(s[i:], w) {
			return w
		}
	}
	return ""
}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/producer/ -run 'GuardLoanWords|ThaiLoanWords' -v`
Expected: PASS ทุกเคส

หมายเหตุ: `TestGuardLoanWords_Idempotent` เช็คว่าไม่มี `zwsp+zwsp` — ถ้าคำสองคำในรายการอยู่ติดกันสนิทในข้อความจริง (เช่น `เฟซบุ๊กแอดมิน`) จะเกิด ZWSP ติดกันสองตัว ซึ่งไม่มีผลต่อการแสดงผลแต่จะทำให้เทสต์นี้ล้ม — ถ้าเจอ ให้แก้ implementation ให้ข้ามการเขียน zwsp ตัวเปิดเมื่อ builder ลงท้ายด้วย zwsp อยู่แล้ว (เก็บสถานะด้วยตัวแปร bool ไม่ต้องเรียก `b.String()` ในลูป)

- [ ] **Step 5: commit**

```bash
go build ./... && go test ./internal/producer/...
git add internal/producer/thaiwrap.go internal/producer/thaiwrap_test.go
git commit -m "feat(captions): GuardLoanWords แทรก ZWSP คร่อมคำทับศัพท์ที่ ICU แบ่งผิด"
```

---

### Task 2: เลิกหั่นวลีกลางคำใน `splitCaptionPhrases`

**Files:**
- Modify: `internal/producer/captions.go:71-123` (ลบ `safeCut`, แก้ `splitCaptionPhrases`)
- Test: `internal/producer/captions_test.go` (เพิ่มเทสต์)

**Interfaces:**
- Consumes: ไม่มี
- Produces: `splitCaptionPhrases(text string) []string` — ลายเซ็นเดิม แต่ไม่หั่น token ที่ยาวเกิน `captionMaxRunes` อีกต่อไป

- [ ] **Step 1: เขียนเทสต์ที่ต้องล้มเหลว**

เพิ่มท้าย `internal/producer/captions_test.go`:

```go
// หกประโยคนี้คือ voice_text จริงของซีนที่ backtest (2026-07-26) จับได้ว่าผู้ชม
// เห็นคำขาดกลางคำ: แอดมินอ|ยู่ · บัญชีที่เค|ยมีปัญหา · ของทั้|งบัญชี ·
// เป็นปัญห|าแบบนี้ · ขาดไม่ไ|ด้ · ตัวตนผู้จ่|ายเงิน
func TestSplitCaptionPhrases_KeepsTokensWhole(t *testing.T) {
	voices := []string{
		"จุดที่สาม domain กับ Page ที่ยังผูกกับ BM เดิม ถ้ายังชี้กลับไปหาเขา เจ้าของเก่าเคลมคืนได้ทั้งที่คุณคุมแอดมินอยู่",
		"ถ้าเป็นบัญชีซื้อมา อย่าเอาบัตรใบเดียวกันไปผูกซ้ำกับบัญชีที่เคยมีปัญหา เพราะมันลากทั้งพอร์ตเสียหายได้",
		"เพราะระบบมองอัตราเปลี่ยนพฤติกรรมสะสมของทั้งบัญชี ไม่ใช่ขนาดงบต่อแคมเปญ",
		"พอร์ตคุณเคยพังจากจุดที่ไม่คิดว่าจะเป็นปัญหาแบบนี้ไหม ทักไลน์ทีมงานมาคุยกันได้ครับ",
		"ปี 2026 CAPI ไม่ใช่ทางเลือกแล้ว มันคือมาตรฐานขั้นต่ำที่ถือหลายบัญชีขาดไม่ได้",
		"ข้อสอง สำคัญมากสำหรับสาย agency คือ Payer KYC ถ้าใครจ่ายเงินแทนคนอื่น แพลตฟอร์มต้องเก็บข้อมูลและยืนยันตัวตนผู้จ่ายเงินด้วย",
	}
	for _, v := range voices {
		whole := map[string]bool{}
		for _, tok := range strings.Fields(v) {
			whole[tok] = true
		}
		for _, ph := range splitCaptionPhrases(v) {
			for _, tok := range strings.Fields(ph) {
				if !whole[tok] {
					t.Errorf("token %q ถูกหั่นมาจากคำเต็ม\nประโยค: %q", tok, v)
				}
			}
		}
	}
}

// วลีเดี่ยวที่ยาวเกิน captionMaxRunes ต้องอยู่ครบเป็นวลีเดียว ไม่ถูกซอย
func TestSplitCaptionPhrases_LongThaiRunStaysWhole(t *testing.T) {
	long := "แพลตฟอร์มต้องเก็บข้อมูลและยืนยันตัวตนผู้จ่ายเงินให้ครบทุกขั้นตอนก่อนอนุมัติ"
	got := splitCaptionPhrases(long)
	if len(got) != 1 {
		t.Fatalf("คาดว่าได้ 1 วลี ได้ %d วลี: %q", len(got), got)
	}
	if got[0] != long {
		t.Errorf("วลีเพี้ยน\n got: %q\nwant: %q", got[0], long)
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าล้มเหลว**

Run: `go test ./internal/producer/ -run 'SplitCaptionPhrases' -v`
Expected: FAIL — `TestSplitCaptionPhrases_KeepsTokensWhole` รายงาน token ที่ถูกหั่น (เช่น `แอดมินอ`) และ `LongThaiRunStaysWhole` ได้ 2 วลี

- [ ] **Step 3: แก้ implementation**

ใน `internal/producer/captions.go` ลบฟังก์ชัน `safeCut` ทั้งก้อน (บรรทัด 71-80) แล้วแทน `splitCaptionPhrases` ด้วย:

```go
// splitCaptionPhrases breaks one scene's narration into caption-sized phrases.
// It packs whitespace-separated tokens (Thai uses spaces between phrases, not
// words) up to captionMaxRunes.
//
// A single token longer than captionMaxRunes stays WHOLE. เดิมโค้ดนี้หั่นมันตาม
// จำนวนตัวอักษร ซึ่งตัดกลางคำแล้วทำให้ผู้ชมเห็นคำขาดข้ามเฟรม ("…แอดมินอ" ค้าง
// จนจบวลี แล้ววลีถัดไปขึ้นต้นด้วย "ยู่…") — backtest 2026-07-26 พบว่าเกิดกับ
// 20% ของซีน. ปล่อยให้ Chromium ขึ้นบรรทัดเองดีกว่า เพราะมันรู้ขอบเขตคำไทย
// (ส่วนคำทับศัพท์ที่มันไม่รู้จัก GuardLoanWords ช่วยประกบไว้อีกชั้น)
func splitCaptionPhrases(text string) []string {
	var phrases []string
	var cur []rune

	flush := func() {
		if len(cur) > 0 {
			phrases = append(phrases, string(cur))
			cur = cur[:0]
		}
	}

	for _, tok := range strings.Fields(text) {
		tr := []rune(tok)

		// Flush the current line if adding this token would overflow it.
		if len(cur) > 0 && len(cur)+1+len(tr) > captionMaxRunes {
			flush()
		}
		if len(cur) > 0 {
			cur = append(cur, ' ')
		}
		cur = append(cur, tr...)
	}
	flush()
	return phrases
}
```

จากนั้นลบ import `"unicode"` ออกจากหัวไฟล์ (เหลือผู้ใช้เพียง `safeCut` ที่เพิ่งลบไป)

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/producer/ -run 'Caption' -v`
Expected: PASS ทั้งเทสต์ใหม่และเทสต์เดิม (`MatchesGroundTruth`, `TimingWithinBoundsAndMonotonic`, `SkipsEmptyAndZeroWidth`, `LongTextSplitsIntoPhrases`, `CarryEmphasisFromScene`)

ถ้า `TestCaptionSegmentsFromScenes_LongTextSplitsIntoPhrases` ล้ม ให้อ่านว่ามันยืนยันอะไร — ถ้ามันคาดหวังจำนวนวลีที่มาจากการ hard-split ให้แก้เทสต์นั้นให้สะท้อนพฤติกรรมใหม่ (วลีเดียวยาวได้) ไม่ใช่แก้ implementation กลับ

- [ ] **Step 5: commit**

```bash
go build ./... && go test ./internal/producer/...
git add internal/producer/captions.go internal/producer/captions_test.go
git commit -m "fix(captions): เลิกหั่นวลีกลางคำที่ 42 ตัวอักษร ปล่อยให้ Chromium ขึ้นบรรทัด"
```

---

### Task 3: เรียก `GuardLoanWords` ในแคปชั่น (และกันไม่ให้รั่วไป TTS)

**Files:**
- Modify: `internal/producer/captions.go:52-66` (ในลูปประกอบ `TranscriptSegment`)
- Test: `internal/producer/captions_test.go`

**Interfaces:**
- Consumes: `GuardLoanWords(string) string` และ `zwsp` จาก Task 1
- Produces: `TranscriptSegment.Text` ที่มี ZWSP คร่อมคำทับศัพท์แล้ว

- [ ] **Step 1: เขียนเทสต์ที่ต้องล้มเหลว**

เพิ่มท้าย `internal/producer/captions_test.go`:

```go
// แคปชั่นต้องถูก guard แล้ว — นี่คือข้อความที่ไปโผล่บนจอจริง
func TestCaptionSegmentsFromScenes_GuardsLoanWords(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, VoiceText: "ทักไลน์ไอดีแอดส์แวนซ์ได้เลยครับ"},
	}
	bounds := []sceneBound{{Start: 0, End: 5}}

	segs := captionSegmentsFromScenes(scenes, bounds)
	if len(segs) == 0 {
		t.Fatal("expected segments, got none")
	}
	joined := ""
	for _, s := range segs {
		joined += s.Text
	}
	if !strings.Contains(joined, zwsp+"ไอดีแอดส์แวนซ์"+zwsp) {
		t.Errorf("แคปชั่นไม่ได้ประกบคำทับศัพท์ด้วย ZWSP: %q", joined)
	}
}

// emphasis ต้องยังจับคู่ได้หลัง guard — ZWSP อยู่นอกคำ ไม่ใช่ในคำ
func TestCaptionSegmentsFromScenes_EmphasisSurvivesGuard(t *testing.T) {
	scenes := []agent.GeneratedScene{
		{SceneNumber: 1, VoiceText: "ต้องถอดสิทธิ์แอดมินออกก่อนเสมอ", EmphasisWords: []string{"แอดมิน"}},
	}
	bounds := []sceneBound{{Start: 0, End: 5}}

	segs := captionSegmentsFromScenes(scenes, bounds)
	found := false
	for _, s := range segs {
		for _, e := range s.Emphasis {
			if e == "แอดมิน" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("คำเน้นหายไปหลัง guard: %+v", segs)
	}
}

// ZWSP ต้องไม่หลุดไปเส้นทาง TTS — ข้อความที่ส่งเข้า splitVoiceText ต้องดิบเสมอ
func TestSplitVoiceText_NeverSeesZWSP(t *testing.T) {
	raw := "ทักไลน์ไอดีแอดส์แวนซ์ได้เลยครับ ทีมงานตอบเร็วมาก"
	for _, chunk := range splitVoiceText(raw, 40) {
		if strings.Contains(chunk, zwsp) {
			t.Errorf("ZWSP รั่วเข้า TTS: %q", chunk)
		}
	}
}
```

- [ ] **Step 2: รันเทสต์ให้เห็นว่าล้มเหลว**

Run: `go test ./internal/producer/ -run 'GuardsLoanWords|EmphasisSurvivesGuard|NeverSeesZWSP' -v`
Expected: `GuardsLoanWords` FAIL (ไม่มี ZWSP ในแคปชั่น) · อีกสองตัว PASS อยู่แล้ว (เป็นเทสต์กันถอยหลัง)

- [ ] **Step 3: แก้ implementation**

ใน `internal/producer/captions.go` แก้ลูปประกอบ segment — สังเกตว่า `emphasisInPhrase` ต้องคำนวณจากข้อความ **ก่อน** guard เพื่อให้จับคู่คำเน้นได้ปกติ:

```go
		for j, ph := range phrases {
			start := cursor
			end := start + span*float64(weights[j])/float64(total)
			if j == len(phrases)-1 {
				end = b.End // pin the last phrase to the boundary; kills float drift
			}
			// หา emphasis จากข้อความดิบก่อน แล้วค่อยประกบคำทับศัพท์ด้วย ZWSP —
			// สลับลำดับแล้วคำเน้นที่เป็นคำทับศัพท์จะจับคู่ไม่เจอ
			emph := emphasisInPhrase(scenes[i].EmphasisWords, ph)
			segs = append(segs, TranscriptSegment{
				Text:     GuardLoanWords(ph),
				Start:    math.Round(start*100) / 100,
				End:      math.Round(end*100) / 100,
				Emphasis: emph,
			})
			cursor = end
		}
```

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/producer/ -run 'Caption|VoiceText' -v`
Expected: PASS ทุกตัว

เทสต์เดิม `TestCaptionSegmentsFromScenes_MatchesGroundTruth` ใช้ `runesNoSpace` ซึ่งไม่ถอด ZWSP — ถ้ามันล้ม ให้แก้ helper ให้ถอด ZWSP ออกก่อนเทียบ (ZWSP เป็นอักขระจัดบรรทัด ไม่ใช่เนื้อความ):

```go
func runesNoSpace(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, zwsp, "")), "")
}
```

- [ ] **Step 5: commit**

```bash
go build ./... && go test ./internal/producer/...
git add internal/producer/captions.go internal/producer/captions_test.go
git commit -m "feat(captions): ประกบคำทับศัพท์ในแคปชั่นด้วย ZWSP"
```

---

### Task 4: เรียก guard กับข้อความบนจอทุกช่อง

**Files:**
- Modify: `internal/producer/scene_adapter.go` (เพิ่ม `guardSceneContent` และเรียกก่อน `return c` ใน `buildSceneContent`)
- Test: `internal/producer/scene_adapter_test.go`

**Interfaces:**
- Consumes: `GuardLoanWords(string) string`, `zwsp` จาก Task 1 · `SceneContent` จาก `composition_types.go`
- Produces: `func guardSceneContent(c *SceneContent)` — แก้ค่าในตัว struct ที่ชี้ไป ไม่คืนค่า

- [ ] **Step 1: เขียนเทสต์ที่ต้องล้มเหลว**

เพิ่มท้าย `internal/producer/scene_adapter_test.go`:

```go
// ข้อความบนจอต้องถูก guard ทุกช่อง — Title ผ่าน highlightTitleStr มาก่อน
// จึงต้องยืนยันว่าแท็ก <span class="acc"> ยังอยู่ครบ
func TestBuildSceneContent_GuardsOnScreenText(t *testing.T) {
	s := agent.GeneratedScene{
		SceneNumber: 1,
		Layout:      "hero",
		Content:     json.RawMessage(`{"title":"เปิดแอดมินให้ครบ","cta":"ทักไอดี","sub":"เช็คเฟซบุ๊กก่อน"}`),
	}
	c := buildSceneContent(s, sceneBound{Start: 0, End: 5})

	if !strings.Contains(c.Title, zwsp+"แอดมิน"+zwsp) {
		t.Errorf("Title ไม่ถูก guard: %q", c.Title)
	}
	if !strings.Contains(c.CTA, zwsp+"ไอดี"+zwsp) {
		t.Errorf("CTA ไม่ถูก guard: %q", c.CTA)
	}
	if !strings.Contains(c.Sub, zwsp+"เฟซบุ๊ก"+zwsp) {
		t.Errorf("Sub ไม่ถูก guard: %q", c.Sub)
	}
}

// ป้ายเมนูใน uistep ต้องคงค่าเดิมทุกไบต์ — gate UIVocabViolations เทียบกับ
// catalog แบบตรงตัว ถ้าแทรก ZWSP เข้าไปคลิปจะถูกบล็อกทั้งใบ
func TestBuildSceneContent_LeavesUIPanelUntouched(t *testing.T) {
	s := agent.GeneratedScene{
		SceneNumber: 1,
		Layout:      "uistep",
		Content: json.RawMessage(`{"title":"เปิดแอดมิน",
			"panel":{"chrome":"Meta Business Suite","breadcrumb":"Settings",
			"items":[{"label":"Business settings","state":"target"}],
			"field":{"label":"ชื่อบัญชี","value":"Ads Vance"}}}`),
	}
	c := buildSceneContent(s, sceneBound{Start: 0, End: 5})

	if c.Panel == nil || len(c.Panel.Items) == 0 {
		t.Fatal("panel หายไป")
	}
	if got := c.Panel.Items[0].Label; got != "Business settings" {
		t.Errorf("ป้ายเมนูถูกแก้: %q", got)
	}
	if got := c.Panel.Field.Value; got != "Ads Vance" {
		t.Errorf("ค่าในช่องถูกแก้: %q", got)
	}
}

// คำที่ไม่ได้อยู่ในรายการต้องไม่ถูกแตะ — กัน guard ไปยุ่งกับข้อความปกติ
func TestBuildSceneContent_LeavesPlainThaiUntouched(t *testing.T) {
	s := agent.GeneratedScene{
		SceneNumber: 1,
		Layout:      "hero",
		Content:     json.RawMessage(`{"title":"เปลี่ยนโดเมนแล้วอัดงบต่อทันที"}`),
	}
	c := buildSceneContent(s, sceneBound{Start: 0, End: 5})
	if strings.Contains(c.Title, zwsp) {
		t.Errorf("แทรก ZWSP ในข้อความที่ไม่มีคำทับศัพท์: %q", c.Title)
	}
}
```

ถ้าไฟล์เทสต์ยังไม่ได้ import `encoding/json` หรือ `strings` ให้เพิ่ม

- [ ] **Step 2: รันเทสต์ให้เห็นว่าล้มเหลว**

Run: `go test ./internal/producer/ -run 'BuildSceneContent_Guards|BuildSceneContent_Leaves' -v`
Expected: `GuardsOnScreenText` FAIL (ไม่มี ZWSP) · อีกสองตัว PASS (เป็นเทสต์กันถอยหลัง)

- [ ] **Step 3: เขียน implementation**

เพิ่มฟังก์ชันนี้ใน `internal/producer/scene_adapter.go` (วางไว้ท้ายไฟล์):

```go
// guardSceneContent ประกบคำทับศัพท์ในทุกช่องข้อความที่ผู้ชมเห็น เรียกเป็นขั้นสุดท้าย
// ของ buildSceneContent เพื่อให้ทำงานหลัง highlightTitleStr แล้ว (สลับลำดับแล้ว
// ZWSP จะไปขวางการจับคู่คำเน้น).
//
// ยกเว้น Panel.Items[].Label และ Panel.Field.Value โดยตั้งใจ — สองช่องนี้ต้อง
// เทียบกับ ui_vocab ของ catalog ได้ทุกไบต์ (agent.UIVocabViolations) และเป็นชื่อ
// เมนูภาษาอังกฤษของ Meta อยู่แล้ว จึงไม่มีคำทับศัพท์ไทยให้ต้องประกบ.
func guardSceneContent(c *SceneContent) {
	c.Kicker = GuardLoanWords(c.Kicker)
	c.Title = GuardLoanWords(c.Title)
	c.Sub = GuardLoanWords(c.Sub)
	c.StatLabel = GuardLoanWords(c.StatLabel)
	c.Pill = GuardLoanWords(c.Pill)
	c.CTA = GuardLoanWords(c.CTA)
	c.Brand = GuardLoanWords(c.Brand)
	c.Stamp = GuardLoanWords(c.Stamp)
	c.Callout = GuardLoanWords(c.Callout)
	c.Hook = GuardLoanWords(c.Hook)
	for i := range c.Rows {
		c.Rows[i].Text = GuardLoanWords(c.Rows[i].Text)
	}
	for i := range c.Chips {
		c.Chips[i].N = GuardLoanWords(c.Chips[i].N)
		c.Chips[i].T = GuardLoanWords(c.Chips[i].T)
	}
	for i := range c.Panels {
		c.Panels[i].T = GuardLoanWords(c.Panels[i].T)
		c.Panels[i].Quote = GuardLoanWords(c.Panels[i].Quote)
	}
	if c.Panel != nil {
		c.Panel.Breadcrumb = GuardLoanWords(c.Panel.Breadcrumb)
		if c.Panel.Field != nil {
			c.Panel.Field.Label = GuardLoanWords(c.Panel.Field.Label)
		}
	}
}
```

แล้วเรียกก่อนบรรทัด `return c` ท้าย `buildSceneContent`:

```go
	// Derive after the hero fallback so speed follows the final layout.
	c.Speed = speedForLayout(c.Layout)
	guardSceneContent(&c)
	return c
}
```

หมายเหตุเรื่อง `Stat`/`Unit`/`Num`/`Of`/`CaseNo`: ไม่ guard เพราะเป็นตัวเลขและหน่วยสั้นที่ตั้ง `white-space:nowrap` ไว้แล้ว ไม่มีการขึ้นบรรทัดให้ผิด

- [ ] **Step 4: รันเทสต์ให้ผ่าน**

Run: `go test ./internal/producer/... -v`
Expected: PASS ทั้งแพ็กเกจ รวม render test เดิมทั้งหมด

ถ้า render test เดิม (`composition_*_render_test.go`) ล้มเพราะเทียบข้อความตรงตัว ให้แก้เทสต์นั้นให้ถอด ZWSP ก่อนเทียบ — อย่าถอด guard ออกจาก production path

- [ ] **Step 5: commit**

```bash
go build ./... && go test ./...
git add internal/producer/scene_adapter.go internal/producer/scene_adapter_test.go
git commit -m "feat(scenes): ประกบคำทับศัพท์ในข้อความบนจอทุกช่อง (ยกเว้น ui_vocab)"
```

---

### Task 5: ยืนยันด้วยการเรนเดอร์จริง

**Files:**
- ไม่แก้โค้ด — ขั้นตอนตรวจสอบผลลัพธ์ ถ้าเจอปัญหาให้ย้อนกลับไปแก้ task ที่เกี่ยว

**Interfaces:**
- Consumes: ทุกอย่างจาก Task 1-4
- Produces: หลักฐานว่าเฟรมจริงไม่มีคำขาดกลางคำ

- [ ] **Step 1: เรนเดอร์คลิปทดสอบจาก HTML**

รัน render test ที่สร้าง HTML จริงแล้วเก็บไฟล์ไว้ดู:

```bash
go test ./internal/producer/ -run 'RealClip|CompositionScenes' -v
```

Expected: PASS — ถ้าล้มให้อ่าน error ว่าเป็นเรื่อง ZWSP หรือไม่

- [ ] **Step 2: ยืนยันว่า ZWSP อยู่ใน HTML จริงและอยู่ถูกที่**

เขียนเทสต์ชั่วคราวหรือใช้ `RenderCompositionScenes` ตรงๆ ผ่าน `go run` เพื่อ dump HTML แล้วตรวจ:

```bash
go test ./internal/producer/ -run 'TestTemplateThaiWrapRules' -v
```

ตรวจด้วยตาว่า HTML ที่ได้: คำทับศัพท์มี `​` ประกบ · แท็ก `<span class="acc">` ยังครบ · ป้ายเมนูใน uistep ไม่มี `​`

- [ ] **Step 3: ตรวจว่ากล่องแคปชั่นไม่บังเนื้อหา**

วลีที่ยาวที่สุดหลังแก้คือ 67 ตัวอักษร ซึ่งกินสามบรรทัด (สูงราว 250px จากเดิมสูงสุด 185px) — เปิดเฟรมที่เรนเดอร์ได้ดูว่ากล่องไม่ทับหัวข้อหรือชิปด้านบน

ถ้าบัง: ลด `captionMaxRunes` จาก 42 เหลือ 34-36 ใน `captions.go` (ซึ่งทำให้แพ็กวลีสั้นลงโดย **ไม่กลับไปหั่นกลางคำ**) แล้วรันเทสต์ Task 2 ซ้ำ

- [ ] **Step 4: รันชุดทดสอบทั้งหมดครั้งสุดท้าย**

```bash
go build ./... && go test ./...
```

Expected: PASS ทั้งหมด

- [ ] **Step 5: commit และเปิด PR**

```bash
git push -u origin fix/thai-word-break-captions
gh pr create --title "fix: คำไทยถูกหั่นกลางคำในแคปชั่นและข้อความบนจอ" --body "$(cat <<'EOF'
## ที่มา
backtest Gemini visual QA 26 คลิป (246 ซีน) พบว่า 14 จาก 19 ตำหนิจริงคือคำถูกหั่นกลางคำ และยังเกิดกับคลิปวันที่ 20-22 ก.ค.

## ต้นเหตุสองชั้น (ยืนยันด้วยการทดลองแล้ว)
1. `splitCaptionPhrases()` หั่นวลีดิบที่ 42 ตัวอักษร → คำขาดข้ามเฟรม (20% ของซีน)
2. พจนานุกรมไทยของ Chromium ไม่รู้จักคำทับศัพท์ → แบ่ง `แอดมินอยู่` เป็น `แอด|มิ|นอ|ยู่` แล้วขึ้นบรรทัดกลางคำ `อยู่`

## การแก้
- ลบ hard-split ออก ปล่อยให้ Chromium ขึ้นบรรทัด (จำนวนวลีลดแค่ 5%, p90 เท่าเดิม)
- เพิ่ม `GuardLoanWords()` แทรก zero-width space คร่อมคำทับศัพท์ 25 คำที่พบจริงในเนื้อหา 982 ซีน
- ไม่แตะ `Panel.Items[].Label` / `Panel.Field.Value` เพื่อให้ผ่าน gate `UIVocabViolations`
- ไม่ให้ ZWSP หลุดไปเส้นทาง TTS (มีเทสต์กันไว้)

สเปค: `docs/superpowers/specs/2026-07-26-thai-word-break-captions-design.md`

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Self-Review

**1. Spec coverage**

| ข้อในสเปค | task ที่ทำ |
|---|---|
| ลบ hard-split + `safeCut` | Task 2 |
| `thaiLoanWords` เรียงยาวไปสั้น | Task 1 (มีเทสต์บังคับลำดับ) |
| `GuardLoanWords` idempotent, pure, ไม่มี error path | Task 1 |
| จุดเรียก 1: `captionSegmentsFromScenes` | Task 3 |
| จุดเรียก 2: ข้อความบนจอ | Task 4 (ทำที่ท้าย `buildSceneContent` แทน `clean()` เพราะต้องอยู่หลัง `highlightTitleStr`) |
| ห้าม ZWSP รั่วไป TTS | Task 3 (`TestSplitVoiceText_NeverSeesZWSP`) |
| ไม่ทำลายแท็ก HTML | Task 1 (`TestGuardLoanWords_LeavesHTMLTagsIntact`) |
| golden test จาก 6 เคสจริง | Task 2 |
| ยืนยันความสูงกล่องไม่บังเนื้อหา | Task 5 Step 3 |

**หมายเหตุการเบี่ยงจากสเปค:** สเปคเสนอให้เรียก guard ใน `clean()` ของ `scene_adapter.go` แต่ตรวจโค้ดจริงแล้วพบว่า `clean()` ถูกเรียก**ก่อน** `highlightTitleStr()` เสมอ ซึ่งขัดกับข้อกำหนดในสเปคเองที่ว่าต้อง guard หลัง highlight แผนนี้จึงย้ายไปทำที่ท้าย `buildSceneContent()` แทน ได้ผลครอบคลุมเท่ากันและลำดับถูกต้อง

**2. Placeholder scan:** ไม่มี TBD/TODO · ทุกขั้นตอนที่เป็นโค้ดมีโค้ดจริง · คำสั่งรันเทสต์ระบุครบพร้อมผลที่คาด

**3. Type consistency:** `GuardLoanWords(string) string` · `zwsp` · `thaiLoanWords` · `loanWordAt(string, int) string` · `guardSceneContent(*SceneContent)` — ชื่อและลายเซ็นตรงกันทุก task · ชื่อช่องของ `SceneContent`, `ContentChip`, `ContentRow`, `ContentPanel`, `ContentUIPanel` ตรงกับ `composition_types.go`
