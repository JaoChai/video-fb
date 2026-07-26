#!/usr/bin/env python3
"""เทียบผล backtest ของ Gemini กับผล Claude ที่บันทึกไว้ใน visual_qa
แบ่งเป็น 3 กอง แล้วดึงเฟรม 30/60/90% ของทุกซีนที่ผลไม่ตรงกัน ไว้ให้คนหรือ agent ตัดสินด้วยตา

  python3 compare.py <manifest.json> <workdir>
"""
import json
import os
import subprocess
import sys

SAMPLES = (0.30, 0.60, 0.90)


def frames_for(workdir, vid, cid, n, start, dur):
    out = []
    for pct in SAMPLES:
        path = os.path.join(workdir, "frames", f"{cid[:8]}_s{n}_{int(pct*100)}.jpg")
        if not os.path.exists(path):
            subprocess.run(["ffmpeg", "-v", "error", "-y", "-ss", f"{start + dur*pct:.2f}",
                            "-i", vid, "-frames:v", "1", "-q:v", "2", path], check=True)
        out.append(path)
    return out


def main(manifest_path, workdir):
    clips = {c["clip_id"]: c for c in json.load(open(manifest_path))}
    os.makedirs(os.path.join(workdir, "frames"), exist_ok=True)

    disputes, agree_fail, agree_pass, stats = [], [], [], {
        "clips": 0, "scenes": 0, "credits": 0.0, "errors": 0, "wall": 0.0}

    for name in sorted(os.listdir(os.path.join(workdir, "res"))):
        res = json.load(open(os.path.join(workdir, "res", name)))
        cid = res["clip_id"]
        clip = clips[cid]
        claude = {v["scene_number"]: v for v in clip["claude"]}
        scenes = {s["n"]: s for s in clip["scenes"]}
        real, total = res["video_duration"], sum(s["dur"] for s in clip["scenes"])
        scale = real / total if total else 1.0
        starts, t = {}, 0.0
        for s in sorted(clip["scenes"], key=lambda s: s["n"]):
            starts[s["n"]] = t * scale
            t += s["dur"]

        stats["clips"] += 1
        stats["scenes"] += len(res["verdicts"])
        stats["credits"] += res.get("credits") or 0
        stats["wall"] += res.get("wall_seconds") or 0
        stats["errors"] += sum(1 for v in res["verdicts"] if v.get("error"))

        vid = os.path.join(workdir, "vid", cid + ".mp4")
        for v in res["verdicts"]:
            n = v["n"]
            c_ok = claude.get(n, {}).get("ok", True)
            g_ok = v["ok"]
            if c_ok and g_ok:
                continue
            rec = {
                "clip_id": cid, "short": cid[:8], "format": clip["format"], "date": clip["date"][:10],
                "scene": n, "claude_ok": c_ok, "gemini_ok": g_ok,
                "claude_issues": claude.get(n, {}).get("issues", []),
                "gemini_issues": v.get("issues", []),
                "ost": scenes.get(n, {}).get("ost"), "voice": scenes.get(n, {}).get("voice"),
                "start": round(starts.get(n, 0), 2), "dur": round(scenes.get(n, {}).get("dur", 0) * scale, 2),
                "kind": "AGREE-FAIL" if (not c_ok and not g_ok)
                        else ("GEMINI-ONLY" if c_ok else "CLAUDE-ONLY"),
            }
            rec["frames"] = frames_for(workdir, vid, cid, n, rec["start"], rec["dur"])
            (agree_fail if rec["kind"] == "AGREE-FAIL" else disputes).append(rec)

    allrecs = agree_fail + disputes
    json.dump(allrecs, open(os.path.join(workdir, "disputes.json"), "w"), ensure_ascii=False, indent=1)

    by = lambda k: [r for r in allrecs if r["kind"] == k]
    print(f"คลิป {stats['clips']} · ซีน {stats['scenes']} · credits {stats['credits']:.1f} "
          f"· เวลา {stats['wall']/60:.1f} นาที · infra error {stats['errors']} ซีน")
    for k in ("AGREE-FAIL", "GEMINI-ONLY", "CLAUDE-ONLY"):
        rows = by(k)
        print(f"\n=== {k} ({len(rows)} ซีน) ===")
        for r in rows:
            print(f"  {r['short']} s{r['scene']:<2} [{r['format']}] "
                  f"C:{(r['claude_issues'] or ['-'])[0][:60]} | G:{(r['gemini_issues'] or ['-'])[0][:60]}")


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
