#!/usr/bin/env python3
"""Backtest: รัน Gemini per-scene QA (วิดีโอช่วงซีน + ภาพนิ่งเต็มความละเอียด)
บนคลิปเก่าที่รู้ผล Claude visual_qa แล้ว เพื่อเทียบก่อนแตะ production

ใช้ system prompt + prompt template ตัวจริงจาก agent_configs (โหลดจาก prompts.json)
เพื่อให้เทียบกับ Claude ได้อย่างยุติธรรม บวก addendum อธิบายว่ามีวิดีโอเพิ่มมา

  python3 backtest.py <manifest.json> <workdir> [clip_id ...]
"""
import base64
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor

KEY = os.environ.get("KIE_API_KEY") or ""
if not KEY:
    raise SystemExit("ตั้ง KIE_API_KEY ก่อนรัน")
URL = "https://api.kie.ai/gemini/v1/models/gemini-3-6-flash:streamGenerateContent"
PARALLEL = int(os.environ.get("BT_PARALLEL", "6"))
STILL_AT = 0.60  # ตำแหน่งภาพนิ่งในซีน

VIDEO_ADDENDUM = """

สิ่งที่คุณได้รับในรอบนี้ (ต่างจากเดิม): ต่อ 1 ซีน คุณจะได้ (1) วิดีโอของซีนนั้นพร้อมเสียงพากย์ (2) ภาพนิ่งความละเอียดเต็มของซีนเดียวกัน
- ใช้ "ภาพนิ่ง" อ่านตัวหนังสือ (คมกว่า) — ตำหนิพวกล้นกรอบ/ถูกครอป/สะกดผิด ให้ยืนยันจากภาพนิ่ง
- ใช้ "วิดีโอ" ตัดสินเรื่องจังหวะเวลา: คาราโอเกะที่ยังขึ้นไม่ครบในภาพนิ่งแต่ขึ้นครบในวิดีโอ = ปกติ ห้าม ok=false
- เสียง: ตั้ง ok=false เฉพาะเมื่อซีนนี้เงียบสนิททั้งช่วงหรือเสียงขาดหายผิดปกติ (ยังไม่ต้องตรวจว่าพากย์ตรงซับ)
- ซีนค้าง/ไม่ขยับเลยทั้งซีน = พัง"""


def load_prompts(workdir):
    p = json.load(open(os.path.join(workdir, "prompts.json")))
    return p["system_prompt"] + VIDEO_ADDENDUM, p["prompt_template"]


def render(tpl, question, n, ost, voice):
    return (tpl.replace("{{.SceneNumber}}", str(n))
               .replace("{{.Question}}", question or "")
               .replace("{{.OnScreenText}}", ost or "")
               .replace("{{.VoiceText}}", voice or ""))


def fetch_video(url, dest):
    if os.path.exists(dest) and os.path.getsize(dest) > 100_000:
        return dest
    subprocess.run(["curl", "-sS", "-A", "curl/8.7.1", "-o", dest, url], check=True)
    return dest


def probe(path):
    out = subprocess.run(["ffprobe", "-v", "error", "-show_entries", "format=duration",
                          "-of", "default=nw=1:nk=1", path], capture_output=True, text=True, check=True)
    return float(out.stdout.strip())


def cut(src, start, dur, out_seg, out_img):
    subprocess.run(["ffmpeg", "-v", "error", "-y", "-ss", f"{start:.2f}", "-t", f"{dur:.2f}", "-i", src,
                    "-c:v", "libx264", "-crf", "28", "-c:a", "aac", "-b:a", "64k", out_seg], check=True)
    subprocess.run(["ffmpeg", "-v", "error", "-y", "-ss", f"{start + dur * STILL_AT:.2f}", "-i", src,
                    "-frames:v", "1", "-q:v", "2", out_img], check=True)


def judge(system, user, seg, img):
    parts = [
        {"inline_data": {"mime_type": "video/mp4", "data": base64.b64encode(open(seg, "rb").read()).decode()}},
        {"inline_data": {"mime_type": "image/jpeg", "data": base64.b64encode(open(img, "rb").read()).decode()}},
        {"text": user},
    ]
    body = {
        "contents": [{"role": "user", "parts": parts}],
        "system_instruction": {"parts": [{"text": system}]},
        "generationConfig": {"temperature": 0.2},
    }
    req = urllib.request.Request(URL, data=json.dumps(body).encode(),
                                 headers={"Authorization": "Bearer " + KEY,
                                          "Content-Type": "application/json",
                                          "User-Agent": "curl/8.7.1"})
    txt, pt, cr = "", None, None
    with urllib.request.urlopen(req, timeout=900) as r:
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
                pt = o["usageMetadata"].get("promptTokenCount")
                cr = o.get("credits_consumed")
    clean = txt.strip()
    if "```" in clean:
        clean = clean.split("```")[1]
        if clean.startswith("json"):
            clean = clean[4:]
    parsed = json.loads(clean)  # ให้ error เด้งขึ้นไปให้ชั้น retry จัดการ
    return bool(parsed.get("ok")), [str(x) for x in parsed.get("issues", [])], pt, cr


def run_scene(job):
    system, user, seg, img, n = job
    last = ""
    for attempt in range(3):
        t0 = time.time()
        try:
            ok, issues, pt, cr = judge(system, user, seg, img)
            return {"n": n, "ok": ok, "issues": issues, "tokens": pt, "credits": cr,
                    "elapsed": round(time.time() - t0, 1), "attempts": attempt + 1}
        except Exception as e:  # network / 5xx / JSON ไม่ผ่าน
            last = f"{type(e).__name__}: {e}"[:200]
            time.sleep(3 * (attempt + 1))
    # fail-open เหมือน production: error โครงสร้างพื้นฐานไม่บล็อกคลิป
    return {"n": n, "ok": True, "issues": [], "error": last, "elapsed": None, "attempts": 3}


def run_clip(clip, workdir, system, tpl):
    cid = clip["clip_id"]
    res_path = os.path.join(workdir, "res", cid + ".json")
    if os.path.exists(res_path):
        print(f"[{cid[:8]}] ข้าม (มีผลแล้ว)", flush=True)
        return json.load(open(res_path))

    vid = fetch_video(clip["url"], os.path.join(workdir, "vid", cid + ".mp4"))
    real = probe(vid)
    scenes = sorted(clip["scenes"], key=lambda s: s["n"])
    total = sum(s["dur"] for s in scenes)
    scale = real / total if total > 0 else 1.0
    if abs(scale - 1.0) > 0.03:
        print(f"[{cid[:8]}] เตือน: ความยาวจริง {real:.1f}s vs ผลรวมซีน {total:.1f}s (scale {scale:.3f})", flush=True)

    jobs, t = [], 0.0
    for s in scenes:
        start, dur = t * scale, s["dur"] * scale
        t += s["dur"]
        seg = os.path.join(workdir, "cut", f"{cid}_{s['n']}.mp4")
        img = os.path.join(workdir, "cut", f"{cid}_{s['n']}.jpg")
        cut(vid, start, dur, seg, img)
        user = render(tpl, clip.get("question") or clip.get("title"), s["n"], s.get("ost"), s.get("voice"))
        jobs.append((system, user, seg, img, s["n"]))

    t0 = time.time()
    with ThreadPoolExecutor(max_workers=PARALLEL) as ex:
        verdicts = sorted(ex.map(run_scene, jobs), key=lambda v: v["n"])
    out = {
        "clip_id": cid, "passed_claude": clip["passed"], "format": clip["format"],
        "date": clip["date"], "video_duration": real, "wall_seconds": round(time.time() - t0, 1),
        "credits": round(sum(v.get("credits") or 0 for v in verdicts), 3),
        "gemini_passed": all(v["ok"] for v in verdicts),
        "verdicts": verdicts,
    }
    json.dump(out, open(res_path, "w"), ensure_ascii=False, indent=1)
    bad = [v["n"] for v in verdicts if not v["ok"]]
    print(f"[{cid[:8]}] claude={'pass' if clip['passed'] else 'FAIL'} "
          f"gemini={'pass' if out['gemini_passed'] else 'FAIL'} ตกซีน={bad} "
          f"{out['wall_seconds']}s {out['credits']}cr", flush=True)
    for f in (j[2] for j in jobs):  # ลบวิดีโอชิ้นย่อย เก็บภาพนิ่งไว้ตรวจด้วยตา
        try:
            os.remove(f)
        except OSError:
            pass
    return out


if __name__ == "__main__":
    manifest_path, workdir = sys.argv[1], sys.argv[2]
    only = set(sys.argv[3:])
    clips = json.load(open(manifest_path))
    if only:
        clips = [c for c in clips if c["clip_id"] in only or c["clip_id"][:8] in only]
    for d in ("vid", "cut", "res"):
        os.makedirs(os.path.join(workdir, d), exist_ok=True)
    system, tpl = load_prompts(workdir)
    for i, c in enumerate(clips, 1):
        print(f"--- {i}/{len(clips)} {c['clip_id'][:8]} ---", flush=True)
        try:
            run_clip(c, workdir, system, tpl)
        except Exception as e:
            print(f"[{c['clip_id'][:8]}] คลิปนี้ล้ม: {type(e).__name__}: {e}", flush=True)
