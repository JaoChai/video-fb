# Full Stack Refactor — Backend + Frontend

**Date:** 2026-04-30
**Approach:** Layer-by-Layer (bottom-up)
**Scope:** Backend internal structure + Frontend shared patterns
**Breaking changes:** API changes allowed freely
**Tests:** Structure-only refactor, no new tests

---

## Decisions

- Refactor ทุกจุดทั้ง backend + frontend
- API endpoints ปรับได้เต็มที่ (frontend แก้ตาม)
- โฟกัสโครงสร้าง ไม่เพิ่ม test coverage ในรอบนี้
- ใช้ Layer-by-Layer approach: models → repository → service → handler → frontend

---

## Layer 1: Backend — Models + Service Layer

### 1.1 แยก `models/models.go` ออกเป็นไฟล์ย่อย

**ปัจจุบัน:** `models/models.go` (172 lines) เป็น dumping ground — domain types, API DTOs, response wrappers รวมอยู่ในไฟล์เดียว

**เปลี่ยนเป็น:**

```
internal/models/
  clip.go         → Clip, ClipMetadata, ClipAnalytics, Scene
  knowledge.go    → KnowledgeSource, KnowledgeSourceSummary, KnowledgeChunk
  agent.go        → AgentConfig + BuildSystemPrompt()
  schedule.go     → Schedule
  theme.go        → BrandTheme
  request.go      → CreateClipRequest, UpdateClipRequest, CreateSceneRequest
  response.go     → APIResponse
```

**เหตุผล:** แยกตาม domain ทำให้หาง่าย แก้ง่าย ไม่ต้อง scroll ผ่าน type ที่ไม่เกี่ยว

### 1.2 เพิ่ม Settings Repository Methods

**ปัจจุบัน:** `orchestrator.go` ดึง settings โดย query DB ตรง (`o.pool.QueryRow(ctx, "SELECT value FROM settings WHERE key = '...'")`). ทำซ้ำ 2 จุด (ProduceWeekly + RetryClip)

**เปลี่ยนเป็น:**

- เพิ่ม methods ใน `repository/settings.go`:
  - `GetCategories(ctx) ([]string, error)`
  - `GetBrandAliases(ctx) (map[string]string, error)`
- Orchestrator inject `settingsRepo` แทนการถือ `*pgxpool.Pool` ตรง

### 1.3 ย้าย Business Logic ออกจาก Handler

**ปัจจุบัน:** `handler/orchestrator.go` RetryFailed() มี retry loop, query failed clips, progress tracking (lines 80-127)

**เปลี่ยนเป็น:**

- สร้าง `orchestrator.RetryAllFailed(ctx, maxRetries int) error` — ย้าย loop + progress tracking เข้ามา
- Handler เหลือแค่: parse request → validate → เรียก service → return HTTP response

### 1.4 KieAI Config Struct

**ปัจจุบัน:** `producer/kieai.go` (412 lines) มี hardcoded timeouts และ magic numbers กระจายทั่ว:
- HTTP client timeouts: 30s vs 5min
- Task timeouts: 180s vs 300s
- Poll interval: 3s
- Max retries: 5

**เปลี่ยนเป็น:**

```go
type KieConfig struct {
    TaskTimeout   time.Duration
    PollInterval  time.Duration
    MaxRetries    int
    HTTPTimeout   time.Duration
}
```

- Inject ผ่าน constructor `NewKieClient(cfg KieConfig)`
- ค่า default ตั้งใน `config/config.go` หรือ hardcoded default ใน constructor

---

## Layer 2: Backend — Orchestrator Refactor

### 2.1 แยก God Function `ProduceWeekly()`

**ปัจจุบัน:** `ProduceWeekly()` ทำ 5 อย่างในฟังก์ชันเดียว (427 lines total file):
1. อ่าน settings (categories, brand_aliases) ← raw SQL
2. Generate questions
3. Loop ผ่านแต่ละ question → script → image → produce
4. จัดการ progress tracking
5. จัดการ error/cancel

**เปลี่ยนเป็น:**

```go
func (o *Orchestrator) ProduceWeekly(ctx context.Context, count int) error {
    cfg, err := o.loadProductionConfig(ctx)
    if err != nil { return err }

    questions, err := o.generateQuestions(ctx, count, cfg)
    if err != nil { return err }

    return o.processClips(ctx, questions, cfg)
}
```

- `loadProductionConfig()` → ใช้ `settingsRepo.GetCategories()` + `settingsRepo.GetBrandAliases()`
- `generateQuestions()` → extract question generation + tracker logic
- `processClips()` → loop + per-clip error handling + progress tracking

### 2.2 ลด Agent Method Parameters

**ปัจจุบัน:** Agent methods รับ 7 parameters:

```go
o.scriptAgent.Generate(ctx, q.Question, q.QuestionerName, q.Category,
    scriptCfg.Model, scriptCfg.BuildSystemPrompt(), scriptCfg.Temperature,
    scriptCfg.PromptTemplate)
```

**เปลี่ยนเป็น:**

```go
o.scriptAgent.Generate(ctx, q, scriptCfg)
```

- `Generate()` รับ `GeneratedQuestion` + `*AgentConfig` แทน parameter แยก
- Apply เหมือนกันกับ `questionAgent`, `scriptAgent`, `imageAgent`

### 2.3 ลบ Direct DB Access จาก Orchestrator

**ปัจจุบัน:** `orchestrator.go` เรียก `o.pool.Exec()` ตรงสำหรับ `clip_metadata` (line 241-245)

**เปลี่ยนเป็น:**

- เพิ่ม `repository/clips.go` → `UpsertMetadata(ctx, ClipMetadata) error`
- Orchestrator ใช้ `o.clipsRepo.UpsertMetadata()` แทน raw SQL
- ลบ `pool *pgxpool.Pool` ออกจาก Orchestrator struct

### 2.4 ย้าย Retry Logic เข้า Orchestrator

**ปัจจุบัน:** Retry loop อยู่ใน `handler/orchestrator.go` (lines 102-126)

**เปลี่ยนเป็น:**

```go
// orchestrator package
func (o *Orchestrator) RetryAllFailed(ctx context.Context, maxRetries int) error {
    failed, err := o.clipsRepo.ListFailed(ctx, maxRetries)
    // ... loop + progress tracking + per-clip retry
}

// handler — slim
func (h *OrchestratorHandler) RetryFailed(w, r) {
    go func() { h.orch.RetryAllFailed(ctx, 2) }()
    writeJSON(w, 202, ...)
}
```

---

## Layer 3: Frontend — Shared Hooks + Page Splitting

### 3.1 สร้าง `useEditableList` Hook

**ปัจจุบัน:** Settings, Knowledge, Agents ทั้ง 3 pages ซ้ำ pattern เดียวกัน (~50 lines/page):

```tsx
const [edits, setEdits] = useState<Record<string, Partial<T>>>({});
const [dirty, setDirty] = useState<Record<string, boolean>>({});
const [expanded, setExpanded] = useState<Record<string, boolean>>({});
const handleEdit = (id, field, value) => { ... };
const toggleExpand = (id) => { ... };
```

**เปลี่ยนเป็น:**

```tsx
// hooks/useEditableList.ts
function useEditableList<T extends { id: string }>(items: T[] | undefined) {
  return {
    edits, dirty, expanded,
    handleEdit, toggleExpand, isDirty, getEditValue, resetDirty
  };
}
```

**ลดโค้ดซ้ำ:** ~150 lines รวมทุก page

### 3.2 สร้าง `useMutationWithToast` Hook

**ปัจจุบัน:** Mutation + toast pattern ซ้ำ 10+ ครั้งทั้ง project:

```tsx
const save = useMutation({
  mutationFn: ...,
  onSuccess: () => { qc.invalidateQueries(...); success('...'); },
  onError: (e) => showError(`...: ${(e as Error).message}`),
});
```

**เปลี่ยนเป็น:**

```tsx
// hooks/useMutationWithToast.ts
function useMutationWithToast<TData, TVariables>(opts: {
  mutationFn: (v: TVariables) => Promise<TData>;
  invalidateKey: string[];
  successMsg: string;
  onSuccess?: () => void;
})
```

### 3.3 แยก Settings.tsx → Components

**ปัจจุบัน:** 387 lines ผสม 3 domains (API keys + voice + Zernio accounts + agent models)

**เปลี่ยนเป็น:**

```
pages/Settings.tsx                      → layout + compose cards (~40 lines)
components/settings/ApiKeysCard.tsx      → API keys + test button (~100 lines)
components/settings/VoiceSettingsCard.tsx → TTS voice dropdown (~40 lines)
components/settings/ConnectedAccountsCard.tsx → Zernio accounts (~80 lines)
components/settings/AgentModelsCard.tsx  → Agent model config (~80 lines)
```

### 3.4 รวม Sidebar เป็นตัวเดียว

**ปัจจุบัน:** `sidebar.tsx` (115 lines) กับ `mobile-sidebar.tsx` (44 lines) เหมือนกัน 90%

**เปลี่ยนเป็น:**

```tsx
// components/sidebar.tsx — unified
function Sidebar({ mobile?: boolean }) {
  // shared nav items, shared header, shared theme toggle
  // mobile: render ใน Sheet/Drawer wrapper
  // desktop: render ปกติ
}
```

- ลบ `mobile-sidebar.tsx`
- ทั้ง desktop + mobile ใช้ component เดียวกัน

### 3.5 ปรับ `api.ts` — Error Handling

**ปัจจุบัน:** 16 lines, ไม่แยก error types, ไม่ check HTTP status:

```tsx
const json = await res.json();
if (json.error) throw new Error(json.error);
return json.data;
```

**เปลี่ยนเป็น:**

```tsx
export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

export async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(API_KEY && { 'Authorization': `Bearer ${API_KEY}` }),
      ...options?.headers,
    },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new ApiError(res.status, body?.error || res.statusText);
  }
  const json = await res.json();
  return json.data;
}
```

### 3.6 Route Constants

**ปัจจุบัน:** Path hardcoded ใน sidebar nav items + App.tsx routes

**เปลี่ยนเป็น:**

```tsx
// lib/routes.ts
export const ROUTES = {
  CONTENT: '/',
  SCHEDULES: '/schedules',
  AGENTS: '/agents',
  KNOWLEDGE: '/knowledge',
  ANALYTICS: '/analytics',
  PROMPT_HISTORY: '/prompt-history',
  SETTINGS: '/settings',
} as const;
```

---

## Implementation Order

```
Phase 1 (Backend foundation):
  1.1 แยก models → ไฟล์ย่อย
  1.2 เพิ่ม settings repo methods
  1.4 KieAI config struct

Phase 2 (Backend orchestrator):
  2.1 แยก ProduceWeekly
  2.2 ลด agent parameters
  2.3 ลบ direct DB access
  2.4 ย้าย retry logic
  1.3 slim handlers

Phase 3 (Frontend foundation):
  3.5 ปรับ api.ts
  3.6 route constants
  3.1 useEditableList hook
  3.2 useMutationWithToast hook

Phase 4 (Frontend pages):
  3.4 รวม sidebar
  3.3 แยก Settings.tsx
  Apply hooks to Knowledge + Agents pages
```

## Files Changed Summary

### Backend (modify)
- `internal/models/models.go` → แยกเป็น 7 ไฟล์
- `internal/repository/settings.go` → เพิ่ม GetCategories, GetBrandAliases
- `internal/repository/clips.go` → เพิ่ม UpsertMetadata
- `internal/orchestrator/orchestrator.go` → refactor ProduceWeekly, ลบ pool dependency
- `internal/handler/orchestrator.go` → slim down, ย้าย retry logic
- `internal/producer/kieai.go` → inject KieConfig
- `internal/agent/question.go` → ลด parameters
- `internal/agent/script.go` → ลด parameters
- `internal/agent/image.go` → ลด parameters

### Frontend (modify)
- `frontend/src/api.ts` → ApiError class + HTTP status check
- `frontend/src/App.tsx` → ใช้ ROUTES constant
- `frontend/src/pages/Settings.tsx` → split เป็น 4 sub-components
- `frontend/src/pages/Knowledge.tsx` → ใช้ shared hooks
- `frontend/src/pages/Agents.tsx` → ใช้ shared hooks
- `frontend/src/components/sidebar.tsx` → unified desktop+mobile

### Frontend (create)
- `frontend/src/hooks/useEditableList.ts`
- `frontend/src/hooks/useMutationWithToast.ts`
- `frontend/src/lib/routes.ts`
- `frontend/src/components/settings/ApiKeysCard.tsx`
- `frontend/src/components/settings/VoiceSettingsCard.tsx`
- `frontend/src/components/settings/ConnectedAccountsCard.tsx`
- `frontend/src/components/settings/AgentModelsCard.tsx`

### Frontend (delete)
- `frontend/src/components/mobile-sidebar.tsx`

### Backend (create)
- `internal/models/clip.go`
- `internal/models/knowledge.go`
- `internal/models/agent.go`
- `internal/models/schedule.go`
- `internal/models/theme.go`
- `internal/models/request.go`
- `internal/models/response.go`

### Backend (delete)
- `internal/models/models.go` (replaced by split files)
