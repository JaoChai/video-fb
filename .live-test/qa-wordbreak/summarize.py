#!/usr/bin/env python3
"""สรุป eval_raw.json เป็นตารางสำหรับ RESULT.md — กันการพิมพ์ตัวเลขตามด้วยมือแล้วคลาดเคลื่อน

  python3 summarize.py

อ่าน eval_raw.json ที่ eval.py เขียนไว้ แล้วพิมพ์ตาราง markdown + ตัวเลขรวมของทั้งสามชุด
ground truth ของชุด zwsp อ่านจาก ZWSP_HAS_DEFECT ใน eval.py (แหล่งเดียวกัน ไม่ให้หลุดกัน)
"""
import json
import os

from eval import ZWSP_HAS_DEFECT

HERE = os.path.dirname(os.path.abspath(__file__))
rows = json.load(open(os.path.join(HERE, "eval_raw.json"), encoding="utf-8"))

by_frame = {}
for r in rows:
    by_frame.setdefault(r["frame"], []).append(r)


def flagged(r):
    """ตั้ง ok=false พร้อมรหัส wordbreak"""
    return (not r.get("ok", True)) and any(
        str(c).strip().lower() == "wordbreak" for c in r.get("codes", []))


print("| เฟรม | ผลแต่ละครั้ง | จับได้/ครั้ง |")
print("|---|---|---|")
for name in sorted(by_frame):
    runs = sorted(by_frame[name], key=lambda r: r["run"])
    marks = " ".join("F" if flagged(r) else ("x" if not r.get("ok", True) else ".") for r in runs)
    hits = sum(1 for r in runs if not r.get("ok", True))
    print(f"| {name} | `{marks}` | {hits}/{len(runs)} |")

print("\n(`F` = ok=false + codes wordbreak · `x` = ok=false รหัสอื่น · `.` = ปล่อยผ่าน)\n")

gate_tp = sum(1 for r in rows if r["frame"].startswith("bad_") and flagged(r))
gate_n = sum(1 for r in rows if r["frame"].startswith("bad_"))
gate_fp = sum(1 for r in rows if r["frame"].startswith("good_") and not r.get("ok", True))
gate_gn = sum(1 for r in rows if r["frame"].startswith("good_"))
print(f"gate recall (bad_*) : {gate_tp}/{gate_n}")
print(f"gate FP     (good_*): {gate_fp}/{gate_gn}")

z_hit = z_def = z_fp = z_clean = 0
for r in rows:
    if not r["frame"].startswith("zwsp_"):
        continue
    if ZWSP_HAS_DEFECT[r["frame"]]:
        z_def += 1
        z_hit += flagged(r)
    else:
        z_clean += 1
        z_fp += not r.get("ok", True)
print(f"zwsp จับตำหนิจริงได้ : {z_hit}/{z_def}")
print(f"zwsp ตีตกใบสะอาด    : {z_fp}/{z_clean}")

bad_any = sum(1 for n, rs in by_frame.items() if n.startswith("bad_") and any(flagged(r) for r in rs))
good_any = sum(1 for n, rs in by_frame.items()
               if n.startswith("good_") and any(not r.get("ok", True) for r in rs))
print(f"\nรายใบ: bad ที่จับได้อย่างน้อย 1 ครั้ง = {bad_any}/5 · "
      f"good ที่โดนผิดอย่างน้อย 1 ครั้ง = {good_any}/5")
