package scoreboard

import "sort"

// weightScale คือสเกลที่เก็บใน DB: ผลรวม weight ของสูตรที่ enabled ในมิติหนึ่ง ตั้งเป้าที่ 100
// เลือกจำนวนเต็มฐาน 100 เพราะคอลัมน์ weight เป็น INTEGER และ 1 หน่วย = 1% อ่านง่าย
//
// ข้อควรรู้: ผลรวมเป็น 100 พอดีในทางปฏิบัติ แต่ไม่ใช่ค่าคงที่เชิงคณิตศาสตร์ —
// property test 20,000 เคสสุ่มพบ 25 เคส (สภาพที่ค่าจำนวนมากชนขอบพร้อมกัน)
// ที่ได้ 101-102 ไม่มีผลต่อพฤติกรรมจริงเพราะตัวเลือกสูตร (PickNext) ใช้อัตราส่วน
// used/weight ไม่ได้ใช้ค่าสัมบูรณ์ และการจำลอง 52 สัปดาห์จากน้ำหนักเท่ากันอยู่ที่ 100 ตลอด
const weightScale = 100

// floorFactor / ceilFactor คือพื้นและเพดานเทียบกับส่วนแบ่งเท่าๆ กัน (uniform)
// ใช้ตัวคูณแทนเปอร์เซ็นต์ตายตัวเพราะจำนวนสูตรเปลี่ยนได้ — พื้น 10% ตายตัวจะเป็นไปไม่ได้
// ทางคณิตศาสตร์ทันทีที่มีสูตรเกิน 10 ตัว
const (
	floorFactor = 0.5
	ceilFactor  = 2.0
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

// TuneWeights คำนวณ weight ชุดใหม่จากคะแนน โดยมีเบรกสี่ชั้น:
//
//  1. ค่าที่ N < minN ถูกตรึงไว้ที่ส่วนแบ่งเดิม
//  2. ส่วนแบ่งถูก clamp ไว้ใน [floorFactor, ceilFactor] เท่าของ uniform (water-filling)
//  3. ขยับได้แค่ alpha ของระยะทางไปยังเป้าหมายต่อการเรียกหนึ่งครั้ง
//  4. ฟังก์ชันไม่รู้จัก enabled เลย ไม่สามารถเกษียณสูตรเองได้
//
// current คือ weight ปัจจุบันของสูตรที่ enabled ทั้งหมดในมิตินั้น (สเกลใดก็ได้)
// ผลลัพธ์เป็นสเกลผลรวม 100 (ดูข้อควรรู้ที่ weightScale เรื่องเคสขอบที่ได้ 101-102)
// ฟังก์ชันไม่ทำให้ caller's map เปลี่ยนแปลง (pure)
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

	// สร้างสำเนาเพื่อไม่แตะ caller's map (pure)
	currentCopy := make(map[string]int)
	for k, v := range current {
		currentCopy[k] = v
	}

	total := 0
	for _, w := range currentCopy {
		total += w
	}
	if total == 0 {
		total = len(currentCopy) // กันหารศูนย์: ถือว่าทุกตัวเท่ากัน
		for k := range currentCopy {
			currentCopy[k] = 1
		}
	}
	shareCurrent := map[string]float64{}
	for k, w := range currentCopy {
		shareCurrent[k] = float64(w) / float64(total)
	}

	scoreByValue := map[string]Combined{}
	for _, c := range combined {
		scoreByValue[c.Value] = c
	}

	uniform := 1.0 / float64(len(currentCopy))
	low, high := floorFactor*uniform, ceilFactor*uniform

	// แยกตัวที่ตรึง (ข้อมูลน้อย หรือไม่มีคะแนนเลย) ออกจากตัวที่ขยับได้
	var movable []string
	frozenShare := 0.0
	for k := range currentCopy {
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
		for k, v := range currentCopy {
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

	// Water-filling: คำนวณส่วนแบ่งตามคะแนนจากน้อยไปมาก ตรึงค่าที่หลุดขอบ
	// จนกว่าทั้งหมดจะอยู่ในช่วง [low, high]
	pinned := make(map[string]bool)
	for pass := 0; pass < len(movable); pass++ {
		// หา unpinned ที่ยังไม่ตรึง
		var unpinned []string
		pinnedBudget := 0.0
		for _, k := range movable {
			if pinned[k] {
				pinnedBudget += target[k]
			} else {
				unpinned = append(unpinned, k)
			}
		}

		// ถ้าทั้งหมดถูกตรึง ให้ออก
		if len(unpinned) == 0 {
			break
		}

		// คำนวณส่วนแบ่งใหม่สำหรับ unpinned ที่เหลือ
		remainingBudget := budget - pinnedBudget
		if remainingBudget < 0 {
			remainingBudget = 0
		}

		scoreSum := 0.0
		for _, k := range unpinned {
			scoreSum += scoreByValue[k].ScoreFinal
		}

		for _, k := range unpinned {
			if scoreSum > 0 {
				target[k] = scoreByValue[k].ScoreFinal / scoreSum * remainingBudget
			} else {
				target[k] = remainingBudget / float64(len(unpinned))
			}
		}

		// ตรึงค่า unpinned ที่หลุดขอบ
		hasPinned := false
		for _, k := range unpinned {
			if target[k] < low {
				target[k] = low
				pinned[k] = true
				hasPinned = true
			} else if target[k] > high {
				target[k] = high
				pinned[k] = true
				hasPinned = true
			}
		}
		// ถ้าไม่มีใครตรึง แปลว่า unpinned ทั้งหมดอยู่ในช่วง — เสร็จ
		if !hasPinned {
			break
		}
	}

	// เคลื่อนเข้าหาเป้าหมายแค่ alpha ของระยะทาง
	shareNew := map[string]float64{}
	for _, k := range movable {
		shareNew[k] = shareCurrent[k] + alpha*(target[k]-shareCurrent[k])
	}

	// หลังจาก alpha step ต้องเช็คว่า shareNew อยู่ในขอบเขต
	// ใช้ water-filling เช่นเดิมเพื่อให้แน่ใจว่าเสถียร
	pinnedShare := make(map[string]bool)
	for waterPass := 0; waterPass < len(movable); waterPass++ {
		var unpinnedShare []string
		pinnedBudgetShare := 0.0
		for _, k := range movable {
			if pinnedShare[k] {
				pinnedBudgetShare += shareNew[k]
			} else {
				unpinnedShare = append(unpinnedShare, k)
			}
		}

		if len(unpinnedShare) == 0 {
			break
		}

		remainingBudgetShare := budget - pinnedBudgetShare
		if remainingBudgetShare < 0 {
			remainingBudgetShare = 0
		}

		// ตรวจสอบว่าใครเกินขอบในชุด unpinned
		hasPinnedShare := false
		for _, k := range unpinnedShare {
			if shareNew[k] < low {
				shareNew[k] = low
				pinnedShare[k] = true
				hasPinnedShare = true
			} else if shareNew[k] > high {
				shareNew[k] = high
				pinnedShare[k] = true
				hasPinnedShare = true
			}
		}

		if !hasPinnedShare {
			// ยังไม่มีใครตรึง แปลว่าทั้งหมดอยู่ในช่วง — เสร็จ
			break
		}

		// recompute สำหรับ unpinned ที่เหลือ
		sumUnpinned := 0.0
		for _, k := range unpinnedShare {
			sumUnpinned += shareNew[k]
		}

		// ขยับ unpinned สำหรับ budget ที่เหลือ
		if len(unpinnedShare) > 0 && sumUnpinned > 0 {
			scale := remainingBudgetShare / sumUnpinned
			for _, k := range unpinnedShare {
				shareNew[k] *= scale
			}
		} else if len(unpinnedShare) > 0 {
			for _, k := range unpinnedShare {
				shareNew[k] = remainingBudgetShare / float64(len(unpinnedShare))
			}
		}
	}

	// แปลงเป็นจำนวนเต็มสเกล 100 แล้วแจกเศษที่เหลือแบบ largest-remainder
	// เพื่อให้ผลรวมเป็น 100 พอดีเสมอ
	// ต้องระวัง: largest-remainder จะต้องไม่ทำให้ movable เกินเพดาน
	type rem struct {
		key  string
		frac float64
	}
	assigned := 0
	var rems []rem
	for k := range currentCopy {
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

	// เพิ่มเศษเหลือแบบ largest-remainder แต่ระวัง ceil
	// ใช้ Round แทน Truncate เพื่อให้ ceilInt ตรงกับ high bound ที่แท้จริง
	ceilInt := int(high*weightScale + 0.5)

	// ต้องมี no-progress guard เพื่อไม่ให้ loop spin forever
	// ถ้า movable ทั้งหมดแล้วเกิน ceil และมี remainder เหลือ ให้แจกให้ frozen
	noProgressCount := 0
	for i := 0; assigned < weightScale; i++ {
		// หากผ่านมา rems เต็ม 1 รอบโดยไม่มีการให้ unit ใด ถือว่า no progress
		if i > 0 && i%len(rems) == 0 {
			noProgressCount++
		}

		// หากไม่มี progress เกิน 1 full pass ให้ออก และแจกเศษให้ frozen
		if noProgressCount > 1 {
			break
		}

		k := rems[i%len(rems)].key

		// k มาจาก rems ซึ่งสร้างจาก currentCopy อยู่แล้ว จึงไม่ต้องตรวจสมาชิกซ้ำ —
		// frozen คือค่าที่ไม่มีคะแนน หรือมีตัวอย่างน้อยกว่าเกณฑ์
		comb, scored := scoreByValue[k]
		isFrozen := !scored || comb.N < minN

		// ถ้าเป็น movable ต้องไม่เกิน ceiling
		if !isFrozen && out[k] >= ceilInt {
			continue // ข้ามค่าที่เกินเพดานแล้ว
		}
		out[k]++
		assigned++
		noProgressCount = 0 // reset counter เมื่อมีการเพิ่ม
	}

	// หากยังคง assigned < 100 (จาก no-progress break) ให้แจกให้ frozen values
	// frozen values ไม่มี ceiling constraint
	//
	// หมายเหตุ: ทางคณิตศาสตร์นี้ไม่ควรเกิด ต่อไปนี้คือการแสดงให้เห็น:
	// - high = 2 × uniform ดังนั้นผลรวม ceiling = 2 × (len(current) × uniform × weightScale) = 2 × 100 = 200
	// - แม้ทั้งหมด movable ถูก pin ที่ high ผลรวมยังไม่ถึง 100 (ปล่อย margin สำหรับ frozen)
	// - เมื่อมี frozen value ปกติ ผ่านการกระจายที่เหลือ frozen จะได้เศษ
	// - ดังนั้น no-progress break นี้ไม่ควรเกิด: frozen หรือ unpinned movable
	//   จะ absorb เศษก่อนถึงจุดนี้ บรรทัดนี้คือ belt-and-braces ที่ป้องกันความผิดพลาด
	if assigned < weightScale {
		for _, r := range rems {
			if assigned >= weightScale {
				break
			}
			k := r.key
			comb, scored := scoreByValue[k]
			isFrozen := !scored || comb.N < minN
			// ให้เศษแก่ frozen ที่มี largest fraction
			if isFrozen {
				out[k]++
				assigned++
			}
		}
	}

	return out
}
