# 4 ช่วงเวลา 4 โหมด — สเปกออกแบบ

วันที่ 2026-07-28 · สถานะ: รออนุมัติ implement

## ปัญหา

prod ผลิต 4 คลิป/วัน แต่มีหน้าตาแค่ 2 แบบ — 12:00 กับ 18:00 เป็นแฟ้มคดีเหมือนกันเป๊ะ, 15:00 กับ 21:00 เป็นคู่มือเหมือนกันเป๊ะ คนที่ตามช่องเห็นของซ้ำวันละสองรอบ

เป้าหมาย: ให้ทั้ง 4 slot มีหน้าตาต่างกันคนละแบบ โดยไม่เพิ่มงบภาพ AI และไม่ทำให้ตะแกรงกันคลิปสอนผิดอ่อนลง

## สิ่งที่ค้นพบก่อนออกแบบ (กำหนดรูปร่างของสเปกนี้)

1. **ตะแกรงคลิปสอนผูกกับ layout `uistep` โดยตรง** — `tutorialGateFailure` เรียก `UIVocabViolations` (ข้ามทุกซีนที่ `ClampLayout != "uistep"`) และ `CountUIStepScenes` เทียบกับจำนวนขั้นในคลัง slot สอน (15:00 / 21:00) จึงทิ้ง `uistep` ไม่ได้ ต้องห่อด้วยสไตล์ใหม่แทน มิฉะนั้นตะแกรงเปิดค้างและคลิปที่แต่งชื่อเมนูเองจะ auto-publish
2. **`FormatsRepo.PickNext` เลือกจาก enabled format ทั้งหมด** ไม่รู้จัก slot ถ้าไม่ล็อก 18:00 จะสุ่มได้ `case_story` แล้วเล่าเคสเสียหายผ่านหน้าจอแชท
3. **retention แยกโหมดไม่ได้ในตอนนี้** — ทุก content_format ได้ ~0.1 เท่ากันหมด (15-17 คลิป/format, 30 วัน) การเลือกโหมดจึงอิงตรรกะเนื้อหา ไม่ใช่ข้อมูล สเปกนี้ออกแบบให้ **วัดผลได้ทีหลัง**: 1 slot = 1 โหมดตายตัว ทำให้เทียบ retention ข้ามโหมดได้ตรงๆ

## การจับคู่

| ช่วง | โหมด | `Mode` | content_format ที่อนุญาต | ตะแกรงสอน |
|---|---|---|---|---|
| 12:00 | แฟ้มคดี *(มีอยู่)* | `case` | `case_story`, `news` | — |
| 15:00 | คู่มือ spotlight *(มีอยู่)* | `tutorial` | `basic` | ✅ |
| 18:00 | **แชทลูกค้า** *(ใหม่)* | `chat` | `qa`, `tips` | — |
| 21:00 | **ห้องควบคุม** *(ใหม่)* | `warroom` | `tutorial` | ✅ |

เหตุผลการวาง: 18:00 เนื้อหาคือการตอบลูกค้ารายคนอยู่แล้ว หน้าจอแชทจึงเข้ากับบทที่มีโดยไม่ต้องแก้ prompt โครงสร้าง · 21:00 เป็นคลิปสอนมือโปร ห้องควบคุมที่มีตัวเลข/ไฟเตือนเข้ากับกลุ่มนี้กว่าคู่มือมือใหม่ · 15:00 คงคู่มือไว้เพราะเพิ่งออกแบบมาเพื่อมือใหม่โดยเฉพาะ

## โหมดใหม่ 1: แชทลูกค้า (`chat`) — 18:00

หน้าจอแชทเต็มเฟรม เหมือนแอบส่องกล่องข้อความ ฟองซ้าย = ลูกค้าถาม ฟองขวาสีส้ม = เราตอบ

**preset** `ChatPreset` — palette `Brand` เหมือนทุกโหมด · `HeadingFont` Sarabun/Kanit · `Motion{EntranceDur: 0.32, EntranceEase: "power2.out", BGZoomTo: 1.03}` (ฟองข้อความควรโผล่เร็ว ไม่ใช่ภาพยนตร์) · `ImageAnchor` = ภาพถ่ายบุคคลจริงกำลังใช้มือถือตอนกลางคืน แสงจอสีน้ำเงินบนหน้า ไม่มีข้อความ/UI ในภาพ

**layout ใหม่ 3 ตัว**

| layout | ใช้ทำอะไร | ฟิลด์ที่ scene agent ต้องคืน |
|---|---|---|
| `chat_in` | ลูกค้าเล่าปัญหา | `asker` (ชื่อผู้ถาม — ใหม่), `stamp` (เวลา เช่น "21:47 น." — ใช้ฟิลด์เดิมของ case), `msgs[]` (ใหม่) |
| `chat_out` | เราตอบทีละข้อความ | `msgs[]`, `verdict` (สรุปเขียว, ไม่บังคับ) |
| `recap` | สรุปท้ายคลิป | `title`, `chips[]` |

`msgs[]` เป็น type ใหม่ `ContentMessage{From string "them"|"me", Text string, Alert bool}` — `alert` ทำฟองแดง (ใช้กับข้อความที่เป็นสัญญาณอันตราย) เพิ่มลง `SceneContent` เป็น `Msgs []ContentMessage \`json:"msgs,omitempty"\`` และ `Asker string`, `Verdict string`

ซีนที่ใช้ร่วมกับโหมดอื่นได้ตามเดิม: `hook`, `hero`, `stat`, `cta`

**นโยบายภาพ**: 1 ภาพ — ซีนเปิดเท่านั้น ซีนแชทที่เหลือเอาภาพเปิดมาหรี่เป็นพื้นหลัง (กลไก `reuseCover` ที่มีอยู่แล้วใน template) ต้นทุนภาพเท่าคู่มือ

## โหมดใหม่ 2: ห้องควบคุม (`warroom`) — 21:00

จอมอนิเตอร์บนพื้นตารางเรืองแสง กราฟ ตัวเลข ไฟสถานะแดง/เขียว **`uistep` ยังอยู่** แต่ถูกห่อด้วยกรอบจอมอนิเตอร์แทนแผงลอยของคู่มือ

**preset** `WarRoomPreset` — palette `Brand` · `HeadingFont` Prompt/Kanit (หน้าตาเทคนิคกว่า) · `Motion{EntranceDur: 0.26, EntranceEase: "power4.out", BGZoomTo: 1.03}` · `ImageAnchor` = ภาพถ่ายห้องทำงานกลางคืน จอหลายจอเรืองแสงน้ำเงิน ไม่มีข้อความอ่านออกบนจอ

**layout ใหม่ 2 ตัว + uistep ที่แต่งใหม่**

| layout | ใช้ทำอะไร | ฟิลด์ |
|---|---|---|
| `dashboard` | เปิดด้วยตัวเลขที่ผิดปกติ | `statLabel` (ชื่อเมตริก), `chips[]` (= KPI, ใช้ `ContentChip` เดิมที่เพิ่ม `bad`), `callout` (บรรทัดเตือนใต้กราฟ — ใช้ฟิลด์เดิมของ tutorial) |
| `alarm` | เตือน/สรุปสิ่งที่ต้องทำ | `title`, `rows[]` — ฟิลด์เดิมล้วน ไม่มีของใหม่ |
| `uistep` | ขั้นตอนกดเมนู — **schema เดิมทุกฟิลด์ ไม่แก้อะไร** | `panel`, `callout`, `num`, `of`, `stepTotal` |

โหมดนี้ไม่เพิ่มฟิลด์ใหม่ใน `SceneContent` เลย — ใช้ของที่มีอยู่ทั้งหมด ยกเว้น `ContentChip.Bad`

`uistep` ใช้ schema เดิมโดยไม่แก้อะไรเลย ต่างแค่ CSS ที่ `[data-format='warroom'] .scene[data-layout="uistep"]` — เพิ่มกรอบจอ ไฟสถานะ 3 ดวง และเปลี่ยนพื้นแผงเป็นดำน้ำเงิน ผลคือ **ตะแกรง `ui_vocab` + นับขั้นตอนทำงานเหมือนเดิม 100% โดยไม่ต้องแตะโค้ดตะแกรง**

`ContentChip` ต้องเพิ่ม `Bad bool \`json:"bad,omitempty"\`` เพื่อให้ KPI ที่แย่เป็นสีแดง

**นโยบายภาพ**: 1 ภาพ — ซีนเปิด เหมือนคู่มือ

## การล็อก content_format ต่อ slot

เพิ่ม method `FormatsRepo.PickNextIn(ctx, allowed []string)` — คิวรีเดิมบวก `AND cf.format_name = ANY($1)` ตัวเลือก least-used + weight ทำงานเหมือนเดิมภายในชุดที่อนุญาต

`ProduceWeekly` รับ `allowed []string` เพิ่ม แล้ว scheduler ส่งชุดต่อ slot:
- `produceNoon` → `ProduceWeekly(ctx, 1, []string{"case_story", "news"})`
- `produceEvening` → `ProduceWeekly(ctx, 1, []string{"qa", "tips"})`

**เส้นทาง fallback ที่ต้องแก้ด้วย**: เมื่อ `ErrNoFreshNews` โค้ดปัจจุบัน fallback ไป `PickNext` แล้วสุดท้ายบังคับ `qa` — ต้องเปลี่ยนเป็น fallback ภายใน `allowed` ของ slot นั้น (12:00 ที่ news ล้ม → `case_story` ไม่ใช่ `qa`) ถ้า `allowed` เหลือตัวเดียวและตัวนั้นล้ม ให้คืน error ตามปกติ (คลิปนั้นไม่ผลิต ดีกว่าผลิตผิดโหมด)

`clipMode` เปลี่ยนจากตัดสินด้วย tutorial/basic เป็น map ตรงจาก content_format:

```
basic            → ModeTutorial   (15:00 คู่มือ spotlight)
tutorial         → ModeWarRoom    (21:00 ห้องควบคุม)
qa, tips         → ModeChat       (18:00 แชท)
case_story, news → ModeCase       (12:00 แฟ้มคดี)
อื่นๆ (ไม่รู้จัก) → ModeCase       (ค่าปลอดภัย)
```

หมายเหตุ: `basic` → `tutorial` mode และ `tutorial` → `warroom` mode — ชื่อ content_format กับชื่อโหมดไม่ตรงกันโดยตั้งใจ เพราะ content_format คือ "ใครเขียนบท" ส่วนโหมดคือ "หน้าตาเป็นแบบไหน" (หลักการเดิมจาก `agentModeFor` vs `clipMode`) **ต้องมีคอมเมนต์กำกับจุดนี้** ไม่งั้นคนอ่านครั้งแรกจะคิดว่าสลับกันผิด

## agent rows ที่ต้องเพิ่ม (migration)

| แถว | หน้าที่ | ฐานที่ copy มา |
|---|---|---|
| `script_chat` | เขียนบทเป็นบทสนทนา ถาม-ตอบสลับกัน | `script_case` |
| `scene_chat` | แตกซีนเป็น `chat_in`/`chat_out`/`recap` | `scene_case` |
| `critic_chat` | ตรวจว่าบทสนทนาเป็นธรรมชาติ ไม่ใช่บรรยาย | `critic` |
| `script_warroom` | เขียนบทสอนมือโปร เปิดด้วยตัวเลข | `script_tutorial` |
| `scene_warroom` | แตกซีนเป็น `dashboard`/`uistep`/`alarm` — **ต้องคง 1 uistep ต่อ 1 ขั้นในคลัง** | `scene_tutorial` |
| `critic_warroom` | ตรวจแบบเดียวกับ critic_tutorial | `critic_tutorial` |

`modeAgentConfig` fail-open อยู่แล้ว (แถวหายหรือปิด → ใช้แถวฐาน) แต่ `scene_warroom` ที่หายจะทำให้ scene agent ไม่รู้จัก layout ใหม่ แล้ว `ClampLayout` จะลดทุกอย่างเป็น `hero` → คลิปหน้าตาเรียบแต่**ตะแกรงจะจับได้เอง** (uistep เหลือ 0 ≠ จำนวนขั้น) จึงไม่ auto-publish ของเสีย

## ไฟล์ที่ต้องแตะ

| ไฟล์ | แก้อะไร |
|---|---|
| `internal/producer/chat_format.go` *(ใหม่)* | `ChatPreset`, image prompt builder |
| `internal/producer/warroom_format.go` *(ใหม่)* | `WarRoomPreset`, image prompt builder |
| `internal/producer/case_format.go` | เพิ่ม `ModeChat`/`ModeWarRoom`, ขยาย `imageScenesForMode` + `promptForScene` |
| `internal/producer/presets.go` | `PresetByKey` รู้จัก 2 preset ใหม่ |
| `internal/producer/composition_types.go` | `ContentMessage`, ฟิลด์ `Msgs`/`Asker`/`Verdict`/`Alarm`, `ContentChip.Bad` |
| `internal/agent/scene_content.go` | เพิ่ม 5 layout ใหม่ใน `sceneLayouts` |
| `templates/layout_multi_scene.html.tmpl` | CSS 2 บล็อก (`[data-format='chat']`, `[data-format='warroom']`) + JS renderer 5 branch |
| `internal/orchestrator/tutorial.go` | `clipMode` map ใหม่ + `presetFor` รองรับ 4 โหมด |
| `internal/orchestrator/orchestrator.go` | `ProduceWeekly(ctx, count, allowed)` + fallback ภายใน allowed |
| `internal/repository/formats.go` | `PickNextIn` |
| `internal/scheduler/scheduler.go` | ส่ง allowed ต่อ slot |
| `migrations/073_four_mode_slots.sql` | agent rows 6 แถว |

## ลำดับทำ

1. โครงข้อมูล + โหมด (`ModeChat`, `ModeWarRoom`, preset, types, `sceneLayouts`) → เทสต์ผ่าน
2. template CSS + JS renderer → เทสต์เรนเดอร์จริง 1 เฟรมต่อ layout ใหม่
3. `PickNextIn` + ล็อก format ต่อ slot + `clipMode` ใหม่ → เทสต์
4. migration agent rows
5. เรนเดอร์ตัวอย่างจริงด้วย `HF_RENDER` ดูด้วยตา ก่อนเปิดใช้บน prod

## ตรวจว่าสำเร็จยังไง

- `go build ./...` + `go test ./...` ผ่าน
- เทสต์ใหม่: `clipMode` ให้ 4 โหมดถูกต้องตาม content_format ทั้ง 6 ค่า
- เทสต์ใหม่: `PickNextIn` ไม่เคยคืน format นอก allowed
- เทสต์ใหม่: `imageScenesForMode` คืน 1 ภาพสำหรับ `chat` และ `warroom`
- เทสต์เรนเดอร์: HTML ที่ออกมามี `data-format="chat"` / `data-format="warroom"` และ markup ของ layout ใหม่ครบ
- **เทสต์กันถอยหลัง**: คลิป warroom ที่ scene agent คืน 0 uistep ต้องถูกตะแกรงจับ (ยืนยันว่าห่อ uistep แล้วตะแกรงยังทำงาน)
- eyeball 1 คลิปต่อโหมดใหม่ ก่อนปล่อยยาว

## ความเสี่ยง

| ความเสี่ยง | ผลถ้าเกิด | กันยังไง |
|---|---|---|
| scene agent โหมดใหม่คืน layout ที่ไม่รู้จัก | `ClampLayout` ลดเป็น `hero` — คลิปเรียบแต่ไม่พัง | prompt ระบุ layout ที่ใช้ได้ + ตะแกรงจับฝั่ง warroom |
| ห่อ uistep แล้ว CSS ทับกันจนอ่านไม่ออก | คลิปสอนอ่านไม่รู้เรื่อง | เรนเดอร์จริงดูด้วยตาก่อนเปิด (ขั้นที่ 5) |
| ล็อก format แล้วคลังหัวข้อของ slot นั้นตัน | คลิปไม่ผลิต | fallback ภายใน allowed + ถ้าเหลือตัวเดียวแล้วล้มให้ error ดังๆ ไม่ผลิตผิดโหมด |
| คลิปเก่าถูก retry หลัง deploy | rebuild ด้วยโหมดใหม่ตาม content_format | `presetFor` ตัดสินจาก content_format อยู่แล้ว — พฤติกรรมนี้ถูกต้อง ไม่ต้องแก้ |

## Rollback

ไม่ใส่ feature flag — ระบบเพิ่งลบ flag ชุดใหญ่ไปเพราะกลายเป็นโค้ดตาย (PR #31) การเพิ่มกลับจะซ้ำรอยเดิม

rollback = `git revert` ของ PR นี้ ส่วน agent rows ที่ migration เพิ่มไว้จะค้างใน DB แต่ไม่มีใครเรียก (`modeAgentConfig` จะหาแถวชื่อ `_chat`/`_warroom` ก็ต่อเมื่อโหมดนั้นมีอยู่) จึงไม่ต้อง down-migration

## สิ่งที่จงใจไม่ทำ

- ไม่เพิ่มงบภาพ AI — โหมดใหม่ทั้งคู่ใช้ 1 ภาพ ที่เหลือเป็น CSS
- ไม่แตะโหมดแฟ้มคดีกับคู่มือที่รันอยู่ (ลดพื้นที่ที่พังได้)
- ไม่ทำระบบสุ่มโหมด — 1 slot = 1 โหมดตายตัว เพื่อให้เทียบ retention ข้ามโหมดได้
- ไม่แตะตะแกรง `ui_vocab` — ห่อ `uistep` แทนการเขียนตะแกรงใหม่
