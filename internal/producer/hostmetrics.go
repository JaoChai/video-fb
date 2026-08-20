package producer

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// เส้นทางลิมิต/การใช้หน่วยความจำของ cgroup ตามที่ "มองจากในคอนเทนเนอร์" — v2 ก่อน
// แล้วค่อยถอยไป v1 · ตั้งใจไม่ไล่ตาม /proc/self/cgroup เพราะบนเครื่องจริงลิมิตอาจอยู่
// ใน slice ซ้อน ซึ่งไม่ใช่เคสที่เราสนใจ (เราสนใจคอนเทนเนอร์ Railway)
var cgroupMemCurrentPaths = []string{
	"/sys/fs/cgroup/memory.current",
	"/sys/fs/cgroup/memory/memory.usage_in_bytes",
}

var cgroupMemMaxPaths = []string{
	"/sys/fs/cgroup/memory.max",
	"/sys/fs/cgroup/memory/memory.limit_in_bytes",
}

// parseCgroupBytes แปลงเนื้อไฟล์ cgroup memory เป็นไบต์ · "max" คือไม่จำกัด และ
// cgroup v1 ใช้ตัวเลขมหึมา (2^63-1 ปัดหน้าเพจ) แทนคำว่าไม่จำกัด — ทั้งสองแบบคืน false
func parseCgroupBytes(content string) (int64, bool) {
	s := strings.TrimSpace(content)
	if s == "" || s == "max" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 || n >= 1<<60 {
		return 0, false
	}
	return n, true
}

// readFirstCgroupBytes อ่านเส้นทางแรกที่อ่านได้และให้ค่าที่ใช้ได้จริง
func readFirstCgroupBytes(paths []string) (int64, bool) {
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if n, ok := parseCgroupBytes(string(b)); ok {
			return n, true
		}
	}
	return 0, false
}

// diskFreeMB คืนที่ว่าง (MB) ของ filesystem ที่ dir อยู่
func diskFreeMB(dir string) (int64, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, false
	}
	return int64(st.Bavail) * int64(st.Bsize) / (1024 * 1024), true
}

// hostSnapshot สรุปสภาพเครื่องเป็นบรรทัดเดียวสำหรับ log — เขียนไว้เพราะตอนเรนเดอร์
// ค้างจน protocolTimeout (20 ส.ค. 2026) เราไม่มีตัวเลขแรมหรือดิสก์ของคอนเทนเนอร์
// สักตัว จึงสรุปไม่ได้ว่าตันที่อะไร ครั้งหน้าต้องมี
func hostSnapshot(dir string) string {
	mem := "n/a"
	if cur, ok := readFirstCgroupBytes(cgroupMemCurrentPaths); ok {
		if max, okMax := readFirstCgroupBytes(cgroupMemMaxPaths); okMax {
			mem = fmt.Sprintf("%d/%dMB", cur/(1024*1024), max/(1024*1024))
		} else {
			mem = fmt.Sprintf("%dMB/unlimited", cur/(1024*1024))
		}
	}
	disk := "n/a"
	if free, ok := diskFreeMB(dir); ok {
		disk = fmt.Sprintf("%dMB", free)
	}
	return fmt.Sprintf("mem %s disk_free %s", mem, disk)
}

// hostSampleInterval คือจังหวะสุ่มวัดระหว่างเรนเดอร์ · 30 วินาทีให้ภาพพอเห็นการไต่
// ของหน่วยความจำในงานที่ปกติใช้ 75-180 วินาที โดยไม่ทำให้ log ท่วม
const hostSampleInterval = 30 * time.Second

// logHostDuring ยิง log สภาพเครื่องเป็นระยะจนกว่า stop จะถูกปิด
func logHostDuring(tag, dir string, interval time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			log.Printf("host %s: %s", tag, hostSnapshot(dir))
		}
	}
}
