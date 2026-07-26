#!/usr/bin/env python3
"""รอบยืนยัน (two-strike): ซีนที่ด่านแรกตก ยิงซ้ำด้วยภาพนิ่งที่ 85% ของซีน
ซีนจะ "ตกจริง" ก็ต่อเมื่อรอบยืนยันตกด้วย — ตรรกะเดียวกับ ConfirmMerge ใน production

  python3 confirm.py <manifest.json> <workdir>
"""
import base64
import json
import os
import subprocess
import sys
import time
import urllib.request
from concurrent.futures import ThreadPoolExecutor

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from backtest import URL, KEY, load_prompts, render  # noqa: E402

CONFIRM_AT = 0.85
PARALLEL = int(os.environ.get("BT_PARALLEL", "6"))


def judge_still(system, user, img):
    parts = [
        {"inline_data": {"mime_type": "image/jpeg", "data": base64.b64encode(open(img, "rb").read()).decode()}},
        {"text": user},
    ]
    body = {"contents": [{"role": "user", "parts": parts}],
            "system_instruction": {"parts": [{"text": system}]},
            "generationConfig": {"temperature": 0.2}}
    req = urllib.request.Request(URL, data=json.dumps(body).encode(),
                                 headers={"Authorization": "Bearer " + KEY,
                                          "Content-Type": "application/json",
                                          "User-Agent": "curl/8.7.1"})
    txt, cr = "", None
    with urllib.request.urlopen(req, timeout=600) as r:
        for raw in r:
            line = raw.decode("utf-8", "ignore").strip()
            if not line.startswith("data: ") or line == "data: [DONE]":
                continue
            try:
                o = json.loads(line[6:])
            except Exception:
                continue
            for c in o.get("candidates", []):
                for p in c.get("content", {}).get("parts", []):
                    txt += p.get("text", "")
            if "usageMetadata" in o:
                cr = o.get("credits_consumed")
    clean = txt.strip()
    if "```" in clean:
        clean = clean.split("```")[1]
        if clean.startswith("json"):
            clean = clean[4:]
    p = json.loads(clean)
    return bool(p.get("ok")), [str(x) for x in p.get("issues", [])], cr


def work(job):
    system, user, img, cid, n = job
    for attempt in range(3):
        try:
            ok, issues, cr = judge_still(system, user, img)
            return {"clip": cid, "n": n, "confirm_ok": ok, "confirm_issues": issues, "credits": cr}
        except Exception as e:
            err = f"{type(e).__name__}: {e}"[:150]
            time.sleep(3 * (attempt + 1))
    return {"clip": cid, "n": n, "confirm_ok": True, "confirm_issues": [], "error": err}


def main(manifest_path, workdir):
    clips = {c["clip_id"]: c for c in json.load(open(manifest_path))}
    system, tpl = load_prompts(workdir)
    os.makedirs(os.path.join(workdir, "confirm"), exist_ok=True)
    jobs = []
    for name in sorted(os.listdir(os.path.join(workdir, "res"))):
        res = json.load(open(os.path.join(workdir, "res", name)))
        cid = res["clip_id"]
        clip = clips[cid]
        scenes = {s["n"]: s for s in clip["scenes"]}
        real, total = res["video_duration"], sum(s["dur"] for s in clip["scenes"])
        scale = real / total if total else 1.0
        starts, t = {}, 0.0
        for s in sorted(clip["scenes"], key=lambda s: s["n"]):
            starts[s["n"]] = t * scale
            t += s["dur"]
        for v in res["verdicts"]:
            if v["ok"]:
                continue
            n = v["n"]
            img = os.path.join(workdir, "confirm", f"{cid[:8]}_s{n}_85.jpg")
            if not os.path.exists(img):
                at = starts[n] + scenes[n]["dur"] * scale * CONFIRM_AT
                subprocess.run(["ffmpeg", "-v", "error", "-y", "-ss", f"{at:.2f}",
                                "-i", os.path.join(workdir, "vid", cid + ".mp4"),
                                "-frames:v", "1", "-q:v", "2", img], check=True)
            user = render(tpl, clip.get("question") or clip.get("title"), n,
                          scenes[n].get("ost"), scenes[n].get("voice"))
            jobs.append((system, user, img, cid, n))

    print(f"ยิงรอบยืนยัน {len(jobs)} ซีน (ภาพนิ่งที่ {int(CONFIRM_AT*100)}% ของซีน)", flush=True)
    t0 = time.time()
    with ThreadPoolExecutor(max_workers=PARALLEL) as ex:
        out = list(ex.map(work, jobs))
    cr = sum(o.get("credits") or 0 for o in out)
    still = sum(1 for o in out if not o["confirm_ok"])
    print(f"เสร็จใน {time.time()-t0:.0f}s · {cr:.1f} credits · ยังตกอยู่ {still}/{len(out)} ซีน", flush=True)
    json.dump(out, open(os.path.join(workdir, "confirm.json"), "w"), ensure_ascii=False, indent=1)


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
