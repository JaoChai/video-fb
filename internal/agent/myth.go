package agent

import (
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
var ordinalPrefixes = []string{"ข้อ", "ขั้นที่", "ขั้น", "อย่างที่", "ที่", "แบบที่", "ข้อที่"}

// normalizeNumber ตัดคอมมาและศูนย์ท้ายทศนิยมออก เพื่อให้ "40,000" กับ "40000"
// นับเป็นตัวเลขเดียวกัน
func normalizeNumber(s string) string {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSuffix(s, ".")
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
// ตรวจทั้ง voice_text (สิ่งที่คนดูได้ยิน) และ on_screen_text (สิ่งที่คนดูเห็น)
// เพราะตัวเลขที่แต่งขึ้นเสียหายเท่ากันทั้งสองทาง
func FactNumberViolations(scenes []GeneratedScene, b *models.MythBelief) []string {
	if b == nil {
		return nil
	}
	allowed := catalogNumbers(b)

	var out []string
	for _, s := range scenes {
		for field, text := range map[string]string{"voice_text": s.VoiceText, "on_screen_text": s.OnScreenText} {
			for _, loc := range factNumberRe.FindAllStringIndex(text, -1) {
				num := text[loc[0]:loc[1]]
				if isOrdinalAt(text, loc[0]) || allowed[normalizeNumber(num)] {
					continue
				}
				out = append(out, fmt.Sprintf("scene %d %s: ตัวเลข %q ไม่มีในแถวคลัง %s",
					s.SceneNumber, field, num, b.BeliefKey))
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
		hay := strings.ToLower(s.VoiceText + " " + s.OnScreenText)
		for _, term := range unsourcedClaimTerms {
			if strings.Contains(hay, term) {
				out = append(out, fmt.Sprintf("scene %d: อ้าง %q แต่แถวคลัง %s ไม่มี source_url",
					s.SceneNumber, term, b.BeliefKey))
			}
		}
	}
	return out
}
