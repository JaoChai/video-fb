#!/usr/bin/env python3
"""รวมคำตัดสิน ground truth (จาก agent) เข้ากับผล backtest แล้วคิดตัวเลขตามเกณฑ์ที่ตกลงไว้

  python3 aggregate.py <workdir>
"""
import json
import os
import sys
from collections import Counter, defaultdict


def main(workdir):
    disputes = json.load(open(os.path.join(workdir, "disputes.json")))
    key = lambda r: (r["short"], r["scene"])
    truth = {}
    bdir = os.path.join(workdir, "batch")
    for name in sorted(os.listdir(bdir)):
        if not name.startswith("verdict_"):
            continue
        for r in json.load(open(os.path.join(bdir, name))):
            truth[(r["short"], r["scene"])] = r

    missing = [key(d) for d in disputes if key(d) not in truth]
    if missing:
        print(f"!! ยังไม่มีคำตัดสิน {len(missing)} ซีน: {missing[:10]}")

    per_kind = defaultdict(Counter)
    rows = []
    for d in disputes:
        t = truth.get(key(d))
        if not t:
            continue
        v = t["verdict"]
        per_kind[d["kind"]][v] += 1
        rows.append({**d, "truth": v, "what_i_saw": t.get("what_i_saw", ""), "quote": t.get("quote", "")})

    print("=== คำตัดสินด้วยตา ต่อกอง ===")
    for k in ("AGREE-FAIL", "GEMINI-ONLY", "CLAUDE-ONLY"):
        c = per_kind[k]
        tot = sum(c.values())
        print(f"{k:<12} n={tot:<3} REAL={c['REAL']} BORDERLINE={c['BORDERLINE']} FALSE={c['FALSE']}")

    # ตำหนิจริง = REAL (BORDERLINE นับแยก ไม่รวมในตัวหาร)
    real = [r for r in rows if r["truth"] == "REAL"]
    real_by_who = Counter()
    for r in real:
        if not r["claude_ok"] and not r["gemini_ok"]:
            real_by_who["ทั้งคู่จับได้"] += 1
        elif r["gemini_ok"]:
            real_by_who["Claude จับได้ Gemini พลาด"] += 1
        else:
            real_by_who["Gemini จับได้ Claude พลาด"] += 1

    print("\n=== ตำหนิจริง (REAL) ทั้งหมด", len(real), "ซีน ===")
    for k, v in real_by_who.most_common():
        print(f"  {k}: {v}")

    claude_caught = sum(1 for r in real if not r["claude_ok"])
    gemini_caught = sum(1 for r in real if not r["gemini_ok"])
    if real:
        print(f"  recall Claude = {claude_caught}/{len(real)} = {claude_caught/len(real)*100:.0f}%")
        print(f"  recall Gemini = {gemini_caught}/{len(real)} = {gemini_caught/len(real)*100:.0f}%")

    # เกณฑ์ที่ตกลง: ต้องจับตำหนิที่ Claude เคยจับได้ >= 90%
    claude_real = [r for r in real if not r["claude_ok"]]
    keep = sum(1 for r in claude_real if not r["gemini_ok"])
    if claude_real:
        print(f"\nเกณฑ์ 1 — Gemini จับตำหนิจริงที่ Claude เคยจับได้: {keep}/{len(claude_real)} "
              f"= {keep/len(claude_real)*100:.0f}% (ต้อง ≥90%)")

    # เกณฑ์ 2: อัตราตีตกผิด — ระดับซีน และระดับคลิป
    fp_g = sum(1 for r in rows if r["truth"] == "FALSE" and not r["gemini_ok"])
    fp_c = sum(1 for r in rows if r["truth"] == "FALSE" and not r["claude_ok"])
    res_dir = os.path.join(workdir, "res")
    total_scenes = sum(len(json.load(open(os.path.join(res_dir, f)))["verdicts"])
                       for f in os.listdir(res_dir))
    print(f"เกณฑ์ 2 — ซีนที่ถูกตีตกผิด: Gemini {fp_g}/{total_scenes} = {fp_g/total_scenes*100:.1f}% "
          f"· Claude {fp_c}/{total_scenes} = {fp_c/total_scenes*100:.1f}%")

    # คลิปที่ถูกบล็อกทั้งที่ไม่มีตำหนิจริงเลย
    by_clip = defaultdict(list)
    for r in rows:
        by_clip[r["short"]].append(r)
    blocked_wrong = [c for c, rs in by_clip.items()
                     if any(not r["gemini_ok"] for r in rs)
                     and not any(r["truth"] == "REAL" and not r["gemini_ok"] for r in rs)]
    print(f"  คลิปที่ Gemini บล็อกโดยไม่มีตำหนิจริงเลยสักซีน: {len(blocked_wrong)}/{len(by_clip)} {blocked_wrong}")

    # เกณฑ์ 3: ต้องมี >=1 เคสที่ Gemini จับได้แต่ Claude พลาด
    wins = [r for r in real if r["claude_ok"] and not r["gemini_ok"]]
    print(f"เกณฑ์ 3 — ตำหนิจริงที่ Gemini จับได้แต่ Claude พลาด: {len(wins)} เคส (ต้อง ≥1)")
    for r in wins[:12]:
        print(f"    {r['short']} s{r['scene']} [{r['date']}] {r['what_i_saw'][:80]}")

    json.dump(rows, open(os.path.join(workdir, "final.json"), "w"), ensure_ascii=False, indent=1)


if __name__ == "__main__":
    main(sys.argv[1])
