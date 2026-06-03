# Manual Produce Button — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Produce 1 Clip" button on the Content page so users can manually trigger video production from the dashboard without using CLI or API.

**Architecture:** Frontend-only change. The backend endpoint `POST /api/v1/orchestrator/produce` already exists and accepts `{"count": N}` in the body. The button calls this endpoint with `count: 1`, then invalidates `production-status` query so the existing `ProductionProgress` component picks up and displays real-time progress.

**Tech Stack:** React 19, TanStack Query, existing `apiFetch` client

---

## File Structure

| Action | File | Responsibility |
|--------|------|---------------|
| Modify | `frontend/src/pages/Content.tsx` | Add produce button + handler |

No new files needed. No backend changes.

---

## Existing Code Context

The implementing engineer needs to know:

1. **`apiFetch`** (`frontend/src/api.ts`) — generic fetch wrapper that adds auth headers and extracts `.data` from response. Throws on `.error`.

2. **`POST /api/v1/orchestrator/produce`** (`internal/handler/orchestrator.go:29-63`) — accepts optional `{"count": N}` body (defaults to 7). Returns 202 on success, 409 if production already active. Runs production in background goroutine.

3. **`ProductionProgress`** (`frontend/src/components/ProductionProgress.tsx`) — polls `/api/v1/production/status` every 2s when active. Auto-shows when production starts, auto-hides when idle. Already rendered in Content page.

4. **`isProducing`** — already computed in Content.tsx from the shared `production-status` query key. Use this to disable the button during production.

---

### Task 1: Add Manual Produce Button to Content Page

**Files:**
- Modify: `frontend/src/pages/Content.tsx:1-126`

- [ ] **Step 1: Add producing state and handler function**

Add a `producing` state and `handleProduce` function after the existing `handleRetryAll` function (after line 51):

```tsx
const [producing, setProducing] = useState(false);

async function handleProduce(): Promise<void> {
  setProducing(true);
  try {
    await apiFetch('/api/v1/orchestrator/produce', {
      method: 'POST',
      body: JSON.stringify({ count: 1 }),
    });
    queryClient.invalidateQueries({ queryKey: ['production-status'] });
    queryClient.invalidateQueries({ queryKey: ['clips'] });
  } catch (e) {
    console.error('Manual produce failed:', e);
  } finally {
    setProducing(false);
  }
}
```

- [ ] **Step 2: Add the Produce button in the header bar**

In the header `div` (line 105-121), add the Produce button before the existing Retry Failed button. The button should:
- Be disabled when `isProducing` or `producing` is true
- Use brand orange color `#f5851f`
- Show "Producing..." text when in progress

```tsx
<div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 32 }}>
  <h1 style={{ fontSize: 20, fontWeight: 600 }}>Content</h1>
  <div style={{ display: 'flex', gap: 8 }}>
    {!isProducing && (
      <button
        onClick={handleProduce}
        disabled={producing}
        style={{
          padding: '8px 16px', fontSize: 13, fontWeight: 500,
          background: producing ? '#333' : '#f5851f', color: '#fff',
          border: 'none', borderRadius: 6, cursor: producing ? 'not-allowed' : 'pointer',
          opacity: producing ? 0.6 : 1, transition: 'all 0.15s',
        }}
      >
        {producing ? 'Producing...' : 'Produce 1 Clip'}
      </button>
    )}
    {failedCount > 0 && !isProducing && (
      <button
        onClick={handleRetryAll}
        disabled={retrying}
        style={{
          padding: '8px 16px', fontSize: 13, fontWeight: 500,
          background: retrying ? '#333' : '#ef4444', color: '#fff',
          border: 'none', borderRadius: 6, cursor: retrying ? 'not-allowed' : 'pointer',
          opacity: retrying ? 0.6 : 1, transition: 'all 0.15s',
        }}
      >
        {retrying ? 'Retrying...' : `Retry Failed (${failedCount})`}
      </button>
    )}
  </div>
</div>
```

- [ ] **Step 3: Verify build passes**

Run: `cd frontend && npm run build`
Expected: Build succeeds with no TypeScript errors.

- [ ] **Step 4: Visual verification**

Run: `cd frontend && npm run dev`
Check in browser:
1. Button "Produce 1 Clip" visible on Content page (orange, top-right)
2. Click button → button shows "Producing..." briefly → ProductionProgress component appears
3. If production is already active → button is hidden
4. Retry Failed button still works alongside Produce button

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Content.tsx
git commit -m "feat: add manual Produce 1 Clip button on Content page"
```

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Hide button during production (not just disable) | Consistent with existing Retry Failed behavior — both hide when `isProducing` |
| Hardcode count=1 | This is a manual test button, not batch production. User wants to test 1 clip at a time |
| No dropdown for count | YAGNI — user asked for a simple test button. Can add count selector later if needed |
| Wrap both buttons in flex container | Keeps layout clean when both buttons are visible |
