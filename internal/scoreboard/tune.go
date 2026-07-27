package scoreboard

import "sort"

// weightScale คือสเกลที่เก็บใน DB: ผลรวม weight ของสูตรที่ enabled ในมิติหนึ่ง = 100
// เลือกจำนวนเต็มฐาน 100 เพราะคอลัมน์ weight เป็น INTEGER และ 1 หน่วย = 1% อ่านง่าย
const weightScale = 100

// floorFactor / ceilFactor คือพื้นและเพดานเทียบกับส่วนแบ่งเท่าๆ กัน (uniform)
// ใช้ตัวคูณแทนเปอร์เซ็นต์ตายตัวเพราะจำนวนสูตรเปลี่ยนได้ — พื้น 10% ตายตัวจะเป็นไปไม่ได้
// ทางคณิตศาสตร์ทันทีที่มีสูตรเกิน 10 ตัว
const (
	floorFactor = 0.5
	ceilFactor  = 2.0
	// clampPasses คือจำนวนรอบสลับ clamp/normalize การ clamp ทำให้ผลรวมเพี้ยน
	// การ normalize ทำให้หลุด clamp — วนไม่กี่รอบก็ลู่เข้าพอสำหรับสเกลจำนวนเต็ม
	clampPasses = 3
)

// Combined คือคะแนนของหนึ่งสูตรที่รวมทุกแพลตฟอร์มแล้ว
type Combined struct {
	Value      string
	ScoreFinal float64
	N          int
}

// CombinePlatforms ยุบคะแนนข้ามแพลตฟอร์มของมิติที่ระบุ ถ่วงด้วยจำนวนคลิป —
// แพลตฟอร์มที่มีคลิปมากกว่าจึงมีน้ำหนักในการตัดสินมากกว่าโดยอัตโนมัติ และ
// แพลตฟอร์มที่หยุดโพสต์จะจางหายไปเองเมื่อหลุดหน้าต่างวัดผล
func CombinePlatforms(scores []Score, dimension string) []Combined {
	type acc struct {
		weighted float64
		n        int
	}
	byValue := map[string]*acc{}
	for _, s := range scores {
		if s.Dimension != dimension || s.N == 0 {
			continue
		}
		a := byValue[s.Value]
		if a == nil {
			a = &acc{}
			byValue[s.Value] = a
		}
		a.weighted += s.ScoreFinal * float64(s.N)
		a.n += s.N
	}

	out := make([]Combined, 0, len(byValue))
	for v, a := range byValue {
		out = append(out, Combined{Value: v, ScoreFinal: a.weighted / float64(a.n), N: a.n})
	}
	// เรียงชื่อเพื่อให้ผลลัพธ์ deterministic (การปัดเศษท้ายสุดขึ้นกับลำดับ)
	sort.Slice(out, func(i, j int) bool { return out[i].Value < out[j].Value })
	return out
}

// TuneWeights คำนวณ weight ชุดใหม่จากคะแนน โดยมีเบรกสามชั้นในฟังก์ชันนี้
// (ชั้นที่สี่ — ห้ามแตะ enabled — บังคับด้วยการที่ฟังก์ชันนี้ไม่รู้จัก enabled เลย):
//
//	1. ค่าที่ N < minN ถูกตรึงไว้ที่ส่วนแบ่งเดิม
//	2. ส่วนแบ่งถูก clamp ไว้ใน [floorFactor, ceilFactor] เท่าของ uniform
//	3. ขยับได้แค่ alpha ของระยะทางไปยังเป้าหมายต่อการเรียกหนึ่งครั้ง
//
// current คือ weight ปัจจุบันของสูตรที่ enabled ทั้งหมดในมิตินั้น (สเกลใดก็ได้)
// ผลลัพธ์เป็นสเกลผลรวม 100 เสมอ
func TuneWeights(current map[string]int, combined []Combined, minN int, alpha float64) map[string]int {
	if len(current) == 0 {
		return map[string]int{}
	}
	out := map[string]int{}
	if len(combined) == 0 {
		// ไม่มีคะแนน = ไม่มีข้อมูลใหม่ ห้ามขยับอะไรทั้งนั้น
		for k, v := range current {
			out[k] = v
		}
		return out
	}

	total := 0
	for _, w := range current {
		total += w
	}
	if total == 0 {
		total = len(current) // กันหารศูนย์: ถือว่าทุกตัวเท่ากัน
		for k := range current {
			current[k] = 1
		}
	}
	shareCurrent := map[string]float64{}
	for k, w := range current {
		shareCurrent[k] = float64(w) / float64(total)
	}

	scoreByValue := map[string]Combined{}
	for _, c := range combined {
		scoreByValue[c.Value] = c
	}

	uniform := 1.0 / float64(len(current))
	low, high := floorFactor*uniform, ceilFactor*uniform

	// แยกตัวที่ตรึง (ข้อมูลน้อย หรือไม่มีคะแนนเลย) ออกจากตัวที่ขยับได้
	var movable []string
	frozenShare := 0.0
	for k := range current {
		c, ok := scoreByValue[k]
		if !ok || c.N < minN {
			out[k] = -1 // ทำเครื่องหมายว่าตรึง เดี๋ยวเติมค่าตอนท้าย
			frozenShare += shareCurrent[k]
			continue
		}
		movable = append(movable, k)
	}
	sort.Strings(movable)

	if len(movable) == 0 {
		for k, v := range current {
			out[k] = v
		}
		return out
	}

	budget := 1.0 - frozenShare
	scoreSum := 0.0
	for _, k := range movable {
		scoreSum += scoreByValue[k].ScoreFinal
	}

	target := map[string]float64{}
	for _, k := range movable {
		if scoreSum > 0 {
			target[k] = scoreByValue[k].ScoreFinal / scoreSum * budget
		} else {
			target[k] = budget / float64(len(movable))
		}
	}

	// สลับ clamp กับ normalize จนกว่าจะลู่เข้า: clamp ทำให้ผลรวมเพี้ยน,
	// normalize ทำให้บางตัวหลุดขอบอีก — ไม่กี่รอบก็พอสำหรับสเกลจำนวนเต็ม
	for pass := 0; pass < clampPasses; pass++ {
		s := 0.0
		for _, k := range movable {
			if target[k] < low {
				target[k] = low
			}
			if target[k] > high {
				target[k] = high
			}
			s += target[k]
		}
		if s == 0 {
			break
		}
		for _, k := range movable {
			target[k] = target[k] / s * budget
		}
	}

	// เคลื่อนเข้าหาเป้าหมายแค่ alpha ของระยะทาง
	shareNew := map[string]float64{}
	for _, k := range movable {
		shareNew[k] = shareCurrent[k] + alpha*(target[k]-shareCurrent[k])
	}

	// แปลงเป็นจำนวนเต็มสเกล 100 แล้วแจกเศษที่เหลือแบบ largest-remainder
	// เพื่อให้ผลรวมเป็น 100 พอดีเสมอ
	type rem struct {
		key  string
		frac float64
	}
	assigned := 0
	var rems []rem
	for k := range current {
		var share float64
		if out[k] == -1 {
			share = shareCurrent[k]
		} else {
			share = shareNew[k]
		}
		exact := share * weightScale
		w := int(exact)
		out[k] = w
		assigned += w
		rems = append(rems, rem{key: k, frac: exact - float64(w)})
	}
	sort.Slice(rems, func(i, j int) bool {
		if rems[i].frac != rems[j].frac {
			return rems[i].frac > rems[j].frac
		}
		return rems[i].key < rems[j].key
	})
	for i := 0; assigned < weightScale; i, assigned = i+1, assigned+1 {
		out[rems[i%len(rems)].key]++
	}
	return out
}
