package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/jaochai/video-fb/internal/models"
)

// mythVerdictLabelTH แปลง verdict ของแถวคลังเป็นคำไทยที่ทั้ง prompt และตรายางบนจอ
// ใช้ร่วมกัน — ที่เดียว เพื่อให้สิ่งที่ agent อ่านตรงกับสิ่งที่คนดูเห็น
func mythVerdictLabelTH(v string) string {
	switch v {
	case models.MythVerdictFalse:
		return "ไม่จริง"
	case models.MythVerdictHalfTrue:
		return "จริงครึ่งเดียว"
	case models.MythVerdictOutdated:
		return "เคยจริง วันนี้ไม่จริงแล้ว"
	}
	return ""
}

// MythVerdictLabelTH เปิดคำแปลให้แพ็กเกจอื่นใช้ (producer เอาไปเป็นข้อความตรายาง)
func MythVerdictLabelTH(v string) string { return mythVerdictLabelTH(v) }

// MythBrief คือบล็อกข้อมูลที่ script/scene agent ได้เห็น — ข้อเท็จจริงทุกตัวที่คลิป
// พูดได้ต้องอยู่ในบล็อกนี้ ไม่มีทางอื่น เพราะตะแกรงฝั่ง Go ตรวจตรงนี้ตรงๆ
func MythBrief(b *models.MythBelief) string {
	if b == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## ข้อมูลความเชื่อที่ต้องหักล้าง (ใช้ได้เฉพาะข้อมูลในบล็อกนี้ ห้ามเพิ่มตัวเลขหรือข้อเท็จจริงเอง)\n")
	sb.WriteString("ความเชื่อที่ยกมา: " + b.BeliefTH + "\n")
	sb.WriteString("ทำไมคนถึงเชื่อ (ใช้เล่าให้คนดูไม่รู้สึกถูกดูถูก): " + b.WhyBelievedTH + "\n")
	sb.WriteString("คำตัดสิน: " + mythVerdictLabelTH(b.Verdict) + "\n")
	sb.WriteString("ข้อเท็จจริงที่หักล้าง: " + b.FactTH + "\n")
	sb.WriteString("แหล่งอ้าง (แสดงบนจอ ห้ามแต่งชื่อแหล่งเอง): " + b.SourceLabel + "\n")
	sb.WriteString("ส่วนที่จริงของความเชื่อนี้ (ต้องพูดถึงเสมอ): " + b.NuanceTH + "\n")
	sb.WriteString("ความเสียหายถ้าเชื่อผิด (ใช้เป็นวัตถุดิบของ hook): " + b.CostTH + "\n")
	sb.WriteString("\nกฎเหล็ก: ตัวเลขทุกตัวที่พูดหรือขึ้นจอต้องปรากฏในบล็อกนี้ " +
		"ห้ามอ้างคะแนนความน่าเชื่อถือ ระดับบัญชี หรือเพดานงบที่บล็อกนี้ไม่ได้ระบุ\n")
	return sb.String()
}

// factNumberRe จับตัวเลข (มีจุดทศนิยม/คอมมาได้) ที่เป็นตัวเลขข้อเท็จจริง
var factNumberRe = regexp.MustCompile(`\d[\d,\.]*`)

// ordinalPrefixes คือคำที่อยู่หน้าตัวเลขแล้วทำให้เลขนั้นเป็นลำดับ ไม่ใช่ข้อเท็จจริง
// (สเปก §7.2) — "ข้อ 2" / "ขั้นที่ 3" ไม่ต้องมีในแถวคลัง
//
// ห้ามใส่ "ที่" เดี่ยว — มันกว้างเกินไป จะจับ "งบที่ 40000" ซึ่งไม่ใช่ลำดับ ทำให้ตัวเลข
// หลุดตะแกรง (fix รอบ 1) ใช้เฉพาะรูปเต็มที่จริงๆ บอกลำดับเท่านั้น
var ordinalPrefixes = []string{"ข้อ", "ข้อที่", "ขั้น", "ขั้นที่", "อย่างที่", "แบบที่", "ครั้งที่", "อันดับที่", "วิธีที่"}

// normalizeNumber ทำให้ตัวเลขที่เขียนต่างรูปแต่ค่าเท่ากันเทียบกันได้: ตัดคอมมา
// ("40,000" = "40000") และตัดศูนย์ท้ายทศนิยม ("3.0" = "3")
//
// การตัดศูนย์ท้ายทศนิยมสำคัญกับทิศทางที่ผิดพลาดแล้วเสียแรงคน: ถ้าคลังเขียน "3 วัน"
// แล้วโมเดลพูด "3.0" ตะแกรงจะตีคลิปที่ถูกต้องเข้า needs_review โดยไม่มีใครผิด
func normalizeNumber(s string) string {
	s = strings.ReplaceAll(s, ",", "")
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimSuffix(s, ".")
	}
	return s
}

// catalogNumbers รวมตัวเลขทุกตัวที่แถวคลังอนุญาต
func catalogNumbers(b *models.MythBelief) map[string]bool {
	allowed := map[string]bool{}
	for _, field := range []string{b.FactTH, b.NuanceTH, b.CostTH, b.BeliefTH, b.WhyBelievedTH} {
		for _, m := range factNumberRe.FindAllString(field, -1) {
			allowed[normalizeNumber(m)] = true
		}
	}
	return allowed
}

// scanField ผูก label ของช่องทางข้อความ (voice_text/on_screen_text/content) เข้ากับ
// ข้อความที่จะสแกนหาตัวเลข — content อาจมีหลาย string เลยใช้ slice ไม่ใช่ map
type scanField struct{ label, text string }

// contentStrings สกัดทุกค่า string ออกจาก Content ของซีนแบบ recursive (เดิน map/slice/string)
// เพราะ DOM ของเทมเพลตเอา content.title/sub/rows[].t ฯลฯ มาแสดงบนจอเหมือนกัน
// คืน nil เมื่อ Content ไม่ใช่ JSON ที่ parse ได้ — ไม่ panic ไม่คืน error
// (เพราะ scene ของโหมดอื่นอาจมี content ที่ไม่ใช่ object)
func contentStrings(raw json.RawMessage) []string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	var out []string
	var walk func(x any)
	walk = func(x any) {
		switch t := x.(type) {
		case string:
			out = append(out, t)
		case map[string]any:
			for _, val := range t {
				walk(val)
			}
		case []any:
			for _, val := range t {
				walk(val)
			}
		}
	}
	walk(v)
	return out
}

// isOrdinalAt บอกว่าตัวเลขที่ตำแหน่ง idx ถูกนำหน้าด้วยคำบอกลำดับหรือไม่
func isOrdinalAt(text string, idx int) bool {
	head := strings.TrimSpace(text[:idx])
	for _, p := range ordinalPrefixes {
		if strings.HasSuffix(head, p) {
			return true
		}
	}
	return false
}

// FactNumberViolations คือครึ่ง deterministic ของตะแกรงข้อเท็จจริง: คืนรายการตำหนิ
// เมื่อซีนพูดตัวเลขที่แถวคลังไม่ได้ให้มา คืน nil เมื่อ b == nil (คลิปโหมดอื่น)
//
// ตรวจทั้ง voice_text (สิ่งที่คนดูได้ยิน) on_screen_text (สิ่งที่คนดูเห็น) และ content.*
// (สิ่งที่เทมเพลตเอาขึ้น DOM) เพราะตัวเลขที่แต่งขึ้นเสียหายเท่ากันทุกช่องทาง
func FactNumberViolations(scenes []GeneratedScene, b *models.MythBelief) []string {
	if b == nil {
		return nil
	}
	allowed := catalogNumbers(b)

	var out []string
	for _, s := range scenes {
		fields := []scanField{
			{"voice_text", s.VoiceText},
			{"on_screen_text", s.OnScreenText},
		}
		for _, str := range contentStrings(s.Content) {
			fields = append(fields, scanField{"content", str})
		}
		for _, f := range fields {
			for _, loc := range factNumberRe.FindAllStringIndex(f.text, -1) {
				num := f.text[loc[0]:loc[1]]
				if isOrdinalAt(f.text, loc[0]) || allowed[normalizeNumber(num)] {
					continue
				}
				out = append(out, fmt.Sprintf("scene %d %s: ตัวเลข %q ไม่มีในแถวคลัง %s",
					s.SceneNumber, f.label, num, b.BeliefKey))
			}
		}
	}
	return out
}

// unsourcedClaimTerms คือคำที่อ้างระบบคะแนน/ระดับบัญชีของ Meta ซึ่งไม่มีเอกสาร
// ทางการรองรับ — พูดได้เฉพาะแถวที่มี source_url จริง
var unsourcedClaimTerms = []string{"trust score", "trustscore", "tier", "เทียร์", "hiva", "คะแนนความน่าเชื่อถือ"}

// DisallowedClaimViolations คืนรายการตำหนิเมื่อซีนอ้างระบบคะแนน/ระดับบัญชีขณะที่
// แถวคลังไม่มี source_url — วันนี้ยังไม่มีแถวไหนมี จึงเท่ากับห้ามทั้งหมด
func DisallowedClaimViolations(scenes []GeneratedScene, b *models.MythBelief) []string {
	if b == nil || strings.TrimSpace(b.SourceURL) != "" {
		return nil
	}
	var out []string
	for _, s := range scenes {
		// คำห้ามที่แต่งขึ้นอาจอยู่ใน content.* ได้เหมือนคำพูด — รวมเข้าไปใน haystack
		var hay strings.Builder
		hay.WriteString(s.VoiceText)
		hay.WriteString(" ")
		hay.WriteString(s.OnScreenText)
		for _, str := range contentStrings(s.Content) {
			hay.WriteString(" ")
			hay.WriteString(str)
		}
		low := strings.ToLower(hay.String())
		for _, term := range unsourcedClaimTerms {
			if strings.Contains(low, term) {
				out = append(out, fmt.Sprintf("scene %d: อ้าง %q แต่แถวคลัง %s ไม่มี source_url",
					s.SceneNumber, term, b.BeliefKey))
			}
		}
	}
	return out
}
