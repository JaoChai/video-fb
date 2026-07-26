#!/usr/bin/env python3
"""วัดว่าเกณฑ์ wordbreak ใน prompt ของ visual_qa จับตำหนิได้จริงและไม่ตีตกภาพดี

  KIE_API_KEY=... python3 eval.py            # รันทุกเฟรมใน frames/
  KIE_API_KEY=... python3 eval.py --repeat 3 # ยิงซ้ำวัดความเสถียรของโมเดล

ไฟล์ใน frames/ ต้องชื่อ bad_*.png (คาดว่าโมเดลต้องจับได้) หรือ good_*.png (ต้องปล่อยผ่าน)
prompts.json ดึงมาจาก agent_configs ตัวจริงด้วย fetch_prompts.py — อย่าก๊อปจาก migration
เพราะของจริงคือ prompt เดิมต่อกับส่วนใหม่ ต่อกับ skills ต่อกับ insights

ต้องรันนอก sandbox (ต่อ https://api.kie.ai) — KIE_API_KEY ส่งผ่าน env เท่านั้น ห้ามเขียนลงไฟล์
"""
import base64
import json
import os
import sys
import urllib.request
from collections import Counter

URL = "https://api.kie.ai/claude/v1/messages"          # kieClaudeAPI ใน internal/agent/kiellm.go
MAX_TOKENS = 8000                                       # kieLLMMaxTokens
HERE = os.path.dirname(os.path.abspath(__file__))

KEY = os.environ.get("KIE_API_KEY") or ""
if not KEY:
    raise SystemExit("ตั้ง KIE_API_KEY ก่อนรัน — ดึงจาก Neon: SELECT value FROM settings WHERE key='kie_api_key'")


def render(tpl, scene_number, on_screen, voice):
    """เลียน renderTemplate ของ Go — string replace ล้วน ไม่ใช่ text/template"""
    return (tpl.replace("{{.Question}}", "วิธีดูแลบัญชีโฆษณาให้รอด")
               .replace("{{.SceneNumber}}", str(scene_number))
               .replace("{{.OnScreenText}}", on_screen)
               .replace("{{.VoiceText}}", voice))


def judge(cfg, png_path):
    img = base64.b64encode(open(png_path, "rb").read()).decode()
    user = render(cfg["template"], 1, "เตือนเรื่องบัญชีโฆษณา", "ระวังบัญชีโดนปิดโดยไม่รู้ตัว")
    body = {
        "model": cfg["model"],
        "system": cfg["system"],
        "max_tokens": MAX_TOKENS,
        "stream": False,
        "temperature": cfg["temperature"],
        "messages": [{"role": "user", "content": [
            {"type": "text", "text": user},
            {"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": img}},
        ]}],
    }
    # ต้องตั้ง User-Agent เอง — Cloudflare หน้า api.kie.ai บล็อก "Python-urllib/3.x"
    # ด้วย error code 1010 ทุก request (ตัว Go ผ่านเพราะส่ง "Go-http-client/...")
    req = urllib.request.Request(URL, data=json.dumps(body).encode(),
                                 headers={"Authorization": "Bearer " + KEY,
                                          "Content-Type": "application/json",
                                          "User-Agent": "adsvance-qa-eval/1.0"})
    with urllib.request.urlopen(req, timeout=300) as r:
        resp = json.load(r)
    text = "".join(p.get("text", "") for p in resp.get("content", []))
    if "```" in text:
        text = text.split("```")[1]
        if text.startswith("json"):
            text = text[4:]
    v = json.loads(text[text.index("{"):text.rindex("}") + 1])
    return bool(v.get("ok")), [str(x) for x in v.get("issues", [])], [str(c) for c in v.get("codes", [])]


def main():
    repeat = 1
    if "--repeat" in sys.argv:
        repeat = int(sys.argv[sys.argv.index("--repeat") + 1])
    cfg = json.load(open(os.path.join(HERE, "prompts.json")))
    frames_dir = os.path.join(HERE, "frames")
    tally, rows = Counter(), []
    raw_path = os.path.join(HERE, "eval_raw.json")

    for name in sorted(os.listdir(frames_dir)):
        if not name.endswith(".png"):
            continue
        expect_bad = name.startswith("bad_")
        for k in range(repeat):
            # ยิงพลาดหนึ่งครั้งไม่ควรทำให้ทั้งรอบที่จ่าย credit ไปแล้วสูญ — เก็บไว้แล้วไปต่อ
            try:
                ok, issues, codes = judge(cfg, os.path.join(frames_dir, name))
            except Exception as e:                       # noqa: BLE001
                tally["ERR"] += 1
                rows.append({"frame": name, "run": k + 1, "error": f"{type(e).__name__}: {e}"})
                print(f"{name} #{k+1}: ยิงไม่ผ่าน — {type(e).__name__}: {e}", flush=True)
                continue
            flagged_wordbreak = (not ok) and any(c.strip().lower() == "wordbreak" for c in codes)
            if expect_bad:
                tally["TP" if flagged_wordbreak else "FN"] += 1
            else:
                tally["FP" if not ok else "TN"] += 1
            rows.append({"frame": name, "run": k + 1, "ok": ok, "codes": codes, "issues": issues})
            print(f"{name} #{k+1}: ok={ok} codes={codes} issues={issues}", flush=True)
            # เขียนทุกครั้งเพื่อไม่ให้ผลที่ยิงไปแล้วหายถ้าตายกลางทาง
            json.dump(rows, open(raw_path, "w"), ensure_ascii=False, indent=1)

    tp, fn, fp, tn = tally["TP"], tally["FN"], tally["FP"], tally["TN"]
    print(f"\nจับตำหนิได้ (recall)  : {tp}/{tp+fn}")
    print(f"ตีตกภาพดี (FP)       : {fp}/{fp+tn}")
    if tally["ERR"]:
        print(f"ยิงไม่ผ่าน           : {tally['ERR']} ครั้ง (ไม่นับรวมข้างบน)")
    json.dump(rows, open(raw_path, "w"), ensure_ascii=False, indent=1)


if __name__ == "__main__":
    main()
