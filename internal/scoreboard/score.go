// Package scoreboard คำนวณ "กระดานคะแนนสูตร": คะแนนของแต่ละสูตรที่ใช้ผลิตคลิป
// (content_format / category / style_preset) จากผลลัพธ์จริงบนแพลตฟอร์ม
// ฟังก์ชันในไฟล์นี้เป็น pure ทั้งหมด — ไม่แตะ DB ไม่แตะเวลา — เพื่อให้ทดสอบได้ตรงๆ
package scoreboard

// น้ำหนักคะแนนตาม spec: views 0.5, retention 0.3, ความไม่ flop 0.2
const (
	weightPct       = 0.5
	weightRetention = 0.3
	weightFlop      = 0.2

	// shrinkStrength คือจำนวนคลิป "สมมติ" ที่ค่ากลาง 0.5 ถ่วงไว้ ยิ่ง n จริงน้อย
	// คะแนนยิ่งถูกดึงเข้าหากลาง — กันสูตรที่มี 1-2 คลิปเด้งไปสุดขั้ว
	shrinkStrength = 5.0
	shrinkPrior    = 0.5
)

// Stat คือสถิติดิบของหนึ่งสูตรบนหนึ่งแพลตฟอร์ม MedianRetention = 0 หมายถึง
// "ไม่มีข้อมูล" (TikTok ไม่รายงาน หรือคลิปใหม่ที่ YouTube ยังไม่ออกรายงาน)
// ไม่ได้แปลว่า retention แย่ — ตัวเรียกต้องแยกสองกรณีนี้ให้ออก
type Stat struct {
	Dimension       string
	Value           string
	Platform        string
	N               int
	MedianPct       float64
	MedianRetention float64
	FlopRate        float64
}

// Score คือ Stat ที่คำนวณคะแนนแล้ว ScoreRaw ยังไม่ผ่าน shrinkage, ScoreFinal ผ่านแล้ว
// เก็บทั้งคู่เพื่อให้ debug ได้ว่าคะแนนต่ำเพราะผลงานจริงหรือเพราะข้อมูลน้อย
type Score struct {
	Stat
	ScoreRaw   float64
	ScoreFinal float64
}

// MinMaxNorm ยืดค่าชุดหนึ่งให้อยู่ในช่วง 0..1 ถ้าทุกค่าเท่ากัน (หรือมีค่าเดียว)
// จะคืน 0.5 ทั้งหมด เพราะไม่มีข้อมูลพอจะบอกว่าใครดีกว่าใคร
func MinMaxNorm(vals []float64) []float64 {
	out := make([]float64, len(vals))
	if len(vals) == 0 {
		return out
	}
	min, max := vals[0], vals[0]
	for _, v := range vals[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	if max == min {
		for i := range out {
			out[i] = 0.5
		}
		return out
	}
	for i, v := range vals {
		out[i] = (v - min) / (max - min)
	}
	return out
}

// ScoreRaw รวมสามสัญญาณเป็นคะแนนเดียว 0..1 เมื่อ hasRetention = false จะตัดพจน์
// retention ทิ้งแล้วปรับสัดส่วนที่เหลือให้รวมเป็น 1 (ไม่ใช่แทนค่าด้วย 0 ซึ่งจะ
// ลงโทษสูตรที่ยังไม่มีรายงาน retention ทั้งที่ยอดวิวอาจดี)
func ScoreRaw(medianPct, retentionNorm, flopRate float64, hasRetention bool) float64 {
	if hasRetention {
		return weightPct*medianPct + weightRetention*retentionNorm + weightFlop*(1-flopRate)
	}
	denom := weightPct + weightFlop
	return (weightPct*medianPct + weightFlop*(1-flopRate)) / denom
}

// Shrink ดึงคะแนนเข้าหาค่ากลาง 0.5 ตามจำนวนตัวอย่างที่มี
func Shrink(scoreRaw float64, n int) float64 {
	return (float64(n)*scoreRaw + shrinkStrength*shrinkPrior) / (float64(n) + shrinkStrength)
}

// ScoreAll คำนวณคะแนนให้ทุก Stat โดย normalize retention ภายในกลุ่ม
// (dimension, platform) เดียวกัน — เทียบ retention ข้ามแพลตฟอร์มไม่ได้เพราะ
// สเกลต่างกัน และเทียบข้ามมิติไม่มีความหมาย
func ScoreAll(stats []Stat) []Score {
	out := make([]Score, len(stats))
	groups := map[string][]int{}
	for i, s := range stats {
		out[i] = Score{Stat: s}
		groups[s.Dimension+"|"+s.Platform] = append(groups[s.Dimension+"|"+s.Platform], i)
	}

	for _, idxs := range groups {
		// normalize เฉพาะสมาชิกที่มี retention จริง
		var withRet []int
		var vals []float64
		for _, i := range idxs {
			if stats[i].MedianRetention > 0 {
				withRet = append(withRet, i)
				vals = append(vals, stats[i].MedianRetention)
			}
		}
		norm := MinMaxNorm(vals)
		normByIdx := map[int]float64{}
		for k, i := range withRet {
			normByIdx[i] = norm[k]
		}

		for _, i := range idxs {
			retNorm, has := normByIdx[i]
			out[i].ScoreRaw = ScoreRaw(stats[i].MedianPct, retNorm, stats[i].FlopRate, has)
			out[i].ScoreFinal = Shrink(out[i].ScoreRaw, stats[i].N)
		}
	}
	return out
}
