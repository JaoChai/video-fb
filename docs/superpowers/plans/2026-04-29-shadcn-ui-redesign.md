# Dashboard shadcn/ui Redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the Ads Vance dashboard from inline-CSS dark theme to a shadcn/ui-inspired design system — minimalist, monochrome, clean borders, proper spacing, professional typography.

**Architecture:** Install Tailwind CSS + shadcn/ui component library. Replace all inline styles across 6 pages + sidebar with Tailwind utility classes and shadcn components (Card, Button, Badge, Table, Input, Select, Textarea, Switch, Tabs, Dialog). Keep existing TanStack Query data layer untouched — only the presentation layer changes.

**Tech Stack:** React 19, Vite 8, Tailwind CSS 4, shadcn/ui (manual component copies — no CLI needed), Lucide React icons, Inter font (Google Fonts)

---

## Design System

### Colors (CSS Variables in `style.css`)

```css
:root {
  --background: 0 0% 100%;        /* #FFFFFF */
  --foreground: 240 10% 3.9%;     /* #09090B */
  --card: 0 0% 100%;              /* #FFFFFF */
  --card-foreground: 240 10% 3.9%;
  --popover: 0 0% 100%;
  --popover-foreground: 240 10% 3.9%;
  --primary: 240 5.9% 10%;        /* #18181B */
  --primary-foreground: 0 0% 98%; /* #FAFAFA */
  --secondary: 240 4.8% 95.9%;    /* #F4F4F5 */
  --secondary-foreground: 240 5.9% 10%;
  --muted: 240 4.8% 95.9%;
  --muted-foreground: 240 3.8% 46.1%; /* #71717A */
  --accent: 240 4.8% 95.9%;
  --accent-foreground: 240 5.9% 10%;
  --destructive: 0 84.2% 60.2%;   /* #EF4444 */
  --border: 240 5.9% 90%;         /* #E4E4E7 */
  --input: 240 5.9% 90%;
  --ring: 240 5.9% 10%;
  --radius: 0.5rem;
}

.dark {
  --background: 240 10% 3.9%;     /* #09090B */
  --foreground: 0 0% 98%;         /* #FAFAFA */
  --card: 240 10% 3.9%;
  --card-foreground: 0 0% 98%;
  --primary: 0 0% 98%;
  --primary-foreground: 240 5.9% 10%;
  --secondary: 240 3.7% 15.9%;    /* #27272A */
  --secondary-foreground: 0 0% 98%;
  --muted: 240 3.7% 15.9%;
  --muted-foreground: 240 5% 64.9%;
  --accent: 240 3.7% 15.9%;
  --accent-foreground: 0 0% 98%;
  --destructive: 0 62.8% 30.6%;
  --border: 240 3.7% 15.9%;
  --input: 240 3.7% 15.9%;
  --ring: 240 4.9% 83.9%;
}
```

### Typography
- **Font:** Inter (Google Fonts) — clean, neutral, matches shadcn/ui
- **Scale:** 12px labels / 14px body / 16px large body / 20px h2 / 24px h1 / 30px hero
- **Weight:** 400 regular, 500 medium, 600 semibold

### Icons
- **Library:** Lucide React — same icon set as shadcn/ui
- **Size:** 16px inline, 20px nav, 24px hero

### Layout
- **Sidebar:** 240px fixed, border-right, sticky
- **Content:** max-w-5xl, px-8 py-8
- **Cards:** rounded-lg border bg-card shadow-sm
- **Spacing:** 4px base grid (gap-1 = 4px, gap-2 = 8px, gap-4 = 16px)

---

## File Structure

```
frontend/
  src/
    style.css                    — MODIFY: Replace with Tailwind + shadcn CSS vars
    main.tsx                     — MODIFY: Add Inter font link
    App.tsx                      — MODIFY: Sidebar + layout with Tailwind
    api.ts                       — NO CHANGE
    lib/
      utils.ts                   — CREATE: cn() utility (clsx + twMerge)
    components/
      ui/
        button.tsx               — CREATE: shadcn Button
        card.tsx                 — CREATE: shadcn Card
        badge.tsx                — CREATE: shadcn Badge
        input.tsx                — CREATE: shadcn Input
        textarea.tsx             — CREATE: shadcn Textarea
        select.tsx               — CREATE: shadcn Select
        switch.tsx               — CREATE: shadcn Switch
        table.tsx                — CREATE: shadcn Table
        separator.tsx            — CREATE: shadcn Separator
      sidebar.tsx                — CREATE: Extracted sidebar component
      page-header.tsx            — CREATE: Reusable page header
      status-badge.tsx           — CREATE: Clip status badge
      ProductionProgress.tsx     — MODIFY: Tailwind rewrite
    pages/
      Content.tsx                — MODIFY: Full Tailwind rewrite
      Schedules.tsx              — MODIFY: Full Tailwind rewrite
      Agents.tsx                 — MODIFY: Full Tailwind rewrite
      Knowledge.tsx              — MODIFY: Full Tailwind rewrite
      Analytics.tsx              — MODIFY: Full Tailwind rewrite
      Settings.tsx               — MODIFY: Full Tailwind rewrite
  tailwind.config.ts             — CREATE: Tailwind config with shadcn theme
  postcss.config.js              — CREATE: PostCSS config for Tailwind
  package.json                   — MODIFY: Add dependencies
  index.html                     — MODIFY: Add Inter font preload
```

---

## Task 1: Install Tailwind CSS + Dependencies

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/tailwind.config.ts`
- Create: `frontend/postcss.config.js`
- Modify: `frontend/src/style.css`
- Create: `frontend/src/lib/utils.ts`
- Modify: `frontend/index.html`

- [ ] **Step 1: Install dependencies**

```bash
cd frontend
npm install tailwindcss @tailwindcss/vite clsx tailwind-merge lucide-react
```

- [ ] **Step 2: Configure Vite for Tailwind**

Add Tailwind plugin to `vite.config.ts`:

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
})
```

- [ ] **Step 3: Replace `style.css` with Tailwind base + shadcn CSS variables**

```css
@import "tailwindcss";

@custom-variant dark (&:is(.dark *));

@theme {
  --color-background: hsl(var(--background));
  --color-foreground: hsl(var(--foreground));
  --color-card: hsl(var(--card));
  --color-card-foreground: hsl(var(--card-foreground));
  --color-primary: hsl(var(--primary));
  --color-primary-foreground: hsl(var(--primary-foreground));
  --color-secondary: hsl(var(--secondary));
  --color-secondary-foreground: hsl(var(--secondary-foreground));
  --color-muted: hsl(var(--muted));
  --color-muted-foreground: hsl(var(--muted-foreground));
  --color-accent: hsl(var(--accent));
  --color-accent-foreground: hsl(var(--accent-foreground));
  --color-destructive: hsl(var(--destructive));
  --color-border: hsl(var(--border));
  --color-input: hsl(var(--input));
  --color-ring: hsl(var(--ring));
  --radius-sm: calc(var(--radius) - 4px);
  --radius-md: calc(var(--radius) - 2px);
  --radius-lg: var(--radius);
  --radius-xl: calc(var(--radius) + 4px);
  --font-sans: 'Inter', ui-sans-serif, system-ui, sans-serif;
}

:root {
  --background: 0 0% 100%;
  --foreground: 240 10% 3.9%;
  --card: 0 0% 100%;
  --card-foreground: 240 10% 3.9%;
  --popover: 0 0% 100%;
  --popover-foreground: 240 10% 3.9%;
  --primary: 240 5.9% 10%;
  --primary-foreground: 0 0% 98%;
  --secondary: 240 4.8% 95.9%;
  --secondary-foreground: 240 5.9% 10%;
  --muted: 240 4.8% 95.9%;
  --muted-foreground: 240 3.8% 46.1%;
  --accent: 240 4.8% 95.9%;
  --accent-foreground: 240 5.9% 10%;
  --destructive: 0 84.2% 60.2%;
  --destructive-foreground: 0 0% 98%;
  --border: 240 5.9% 90%;
  --input: 240 5.9% 90%;
  --ring: 240 5.9% 10%;
  --radius: 0.5rem;
}

.dark {
  --background: 240 10% 3.9%;
  --foreground: 0 0% 98%;
  --card: 240 10% 3.9%;
  --card-foreground: 0 0% 98%;
  --popover: 240 10% 3.9%;
  --popover-foreground: 0 0% 98%;
  --primary: 0 0% 98%;
  --primary-foreground: 240 5.9% 10%;
  --secondary: 240 3.7% 15.9%;
  --secondary-foreground: 0 0% 98%;
  --muted: 240 3.7% 15.9%;
  --muted-foreground: 240 5% 64.9%;
  --accent: 240 3.7% 15.9%;
  --accent-foreground: 0 0% 98%;
  --destructive: 0 62.8% 30.6%;
  --destructive-foreground: 0 0% 98%;
  --border: 240 3.7% 15.9%;
  --input: 240 3.7% 15.9%;
  --ring: 240 4.9% 83.9%;
}

*, *::before, *::after {
  border-color: hsl(var(--border));
}

body {
  background-color: hsl(var(--background));
  color: hsl(var(--foreground));
  -webkit-font-smoothing: antialiased;
}
```

- [ ] **Step 4: Create `src/lib/utils.ts`**

```typescript
import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
```

- [ ] **Step 5: Update `index.html` — add Inter font**

Add to `<head>`:
```html
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
```

- [ ] **Step 6: Verify build**

```bash
cd frontend && npm run build
```

- [ ] **Step 7: Commit**

```bash
git add frontend/
git commit -m "chore: install Tailwind CSS + shadcn/ui design tokens"
```

---

## Task 2: Create shadcn/ui Base Components

**Files:**
- Create: `frontend/src/components/ui/button.tsx`
- Create: `frontend/src/components/ui/card.tsx`
- Create: `frontend/src/components/ui/badge.tsx`
- Create: `frontend/src/components/ui/input.tsx`
- Create: `frontend/src/components/ui/textarea.tsx`
- Create: `frontend/src/components/ui/select.tsx`
- Create: `frontend/src/components/ui/switch.tsx`
- Create: `frontend/src/components/ui/table.tsx`
- Create: `frontend/src/components/ui/separator.tsx`

- [ ] **Step 1: Create Button component**

```tsx
// frontend/src/components/ui/button.tsx
import { type ButtonHTMLAttributes, forwardRef } from "react"
import { cn } from "../../lib/utils"

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "default" | "destructive" | "outline" | "secondary" | "ghost" | "link"
  size?: "default" | "sm" | "lg" | "icon"
}

const variants = {
  default: "bg-primary text-primary-foreground hover:bg-primary/90",
  destructive: "bg-destructive text-destructive-foreground hover:bg-destructive/90",
  outline: "border border-input bg-background hover:bg-accent hover:text-accent-foreground",
  secondary: "bg-secondary text-secondary-foreground hover:bg-secondary/80",
  ghost: "hover:bg-accent hover:text-accent-foreground",
  link: "text-primary underline-offset-4 hover:underline",
}

const sizes = {
  default: "h-10 px-4 py-2",
  sm: "h-9 rounded-md px-3",
  lg: "h-11 rounded-md px-8",
  icon: "h-10 w-10",
}

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant = "default", size = "default", ...props }, ref) => (
    <button
      className={cn(
        "inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 cursor-pointer",
        variants[variant],
        sizes[size],
        className
      )}
      ref={ref}
      {...props}
    />
  )
)
Button.displayName = "Button"
export { Button }
```

- [ ] **Step 2: Create Card component**

```tsx
// frontend/src/components/ui/card.tsx
import { type HTMLAttributes, forwardRef } from "react"
import { cn } from "../../lib/utils"

const Card = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn("rounded-lg border bg-card text-card-foreground shadow-sm", className)} {...props} />
  )
)
Card.displayName = "Card"

const CardHeader = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn("flex flex-col space-y-1.5 p-6", className)} {...props} />
  )
)
CardHeader.displayName = "CardHeader"

const CardTitle = forwardRef<HTMLHeadingElement, HTMLAttributes<HTMLHeadingElement>>(
  ({ className, ...props }, ref) => (
    <h3 ref={ref} className={cn("text-2xl font-semibold leading-none tracking-tight", className)} {...props} />
  )
)
CardTitle.displayName = "CardTitle"

const CardDescription = forwardRef<HTMLParagraphElement, HTMLAttributes<HTMLParagraphElement>>(
  ({ className, ...props }, ref) => (
    <p ref={ref} className={cn("text-sm text-muted-foreground", className)} {...props} />
  )
)
CardDescription.displayName = "CardDescription"

const CardContent = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn("p-6 pt-0", className)} {...props} />
  )
)
CardContent.displayName = "CardContent"

export { Card, CardHeader, CardTitle, CardDescription, CardContent }
```

- [ ] **Step 3: Create Badge component**

```tsx
// frontend/src/components/ui/badge.tsx
import { type HTMLAttributes } from "react"
import { cn } from "../../lib/utils"

interface BadgeProps extends HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "secondary" | "destructive" | "outline"
}

const variants = {
  default: "border-transparent bg-primary text-primary-foreground",
  secondary: "border-transparent bg-secondary text-secondary-foreground",
  destructive: "border-transparent bg-destructive text-destructive-foreground",
  outline: "text-foreground",
}

export function Badge({ className, variant = "default", ...props }: BadgeProps) {
  return (
    <div className={cn(
      "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors",
      variants[variant],
      className
    )} {...props} />
  )
}
```

- [ ] **Step 4: Create Input, Textarea, Select, Switch, Table, Separator**

```tsx
// frontend/src/components/ui/input.tsx
import { type InputHTMLAttributes, forwardRef } from "react"
import { cn } from "../../lib/utils"

const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  ({ className, type, ...props }, ref) => (
    <input
      type={type}
      className={cn(
        "flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50",
        className
      )}
      ref={ref}
      {...props}
    />
  )
)
Input.displayName = "Input"
export { Input }
```

```tsx
// frontend/src/components/ui/textarea.tsx
import { type TextareaHTMLAttributes, forwardRef } from "react"
import { cn } from "../../lib/utils"

const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
  ({ className, ...props }, ref) => (
    <textarea
      className={cn(
        "flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50",
        className
      )}
      ref={ref}
      {...props}
    />
  )
)
Textarea.displayName = "Textarea"
export { Textarea }
```

```tsx
// frontend/src/components/ui/switch.tsx
import { type InputHTMLAttributes, forwardRef } from "react"
import { cn } from "../../lib/utils"

interface SwitchProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  checked?: boolean
  onCheckedChange?: (checked: boolean) => void
}

const Switch = forwardRef<HTMLInputElement, SwitchProps>(
  ({ className, checked, onCheckedChange, ...props }, ref) => (
    <button
      role="switch"
      aria-checked={checked}
      className={cn(
        "peer inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50",
        checked ? "bg-primary" : "bg-input",
        className
      )}
      onClick={() => onCheckedChange?.(!checked)}
    >
      <span className={cn(
        "pointer-events-none block h-5 w-5 rounded-full bg-background shadow-lg ring-0 transition-transform",
        checked ? "translate-x-5" : "translate-x-0"
      )} />
    </button>
  )
)
Switch.displayName = "Switch"
export { Switch }
```

```tsx
// frontend/src/components/ui/table.tsx
import { type HTMLAttributes, type TdHTMLAttributes, type ThHTMLAttributes, forwardRef } from "react"
import { cn } from "../../lib/utils"

const Table = forwardRef<HTMLTableElement, HTMLAttributes<HTMLTableElement>>(
  ({ className, ...props }, ref) => (
    <div className="relative w-full overflow-auto">
      <table ref={ref} className={cn("w-full caption-bottom text-sm", className)} {...props} />
    </div>
  )
)
Table.displayName = "Table"

const TableHeader = forwardRef<HTMLTableSectionElement, HTMLAttributes<HTMLTableSectionElement>>(
  ({ className, ...props }, ref) => <thead ref={ref} className={cn("[&_tr]:border-b", className)} {...props} />
)
TableHeader.displayName = "TableHeader"

const TableBody = forwardRef<HTMLTableSectionElement, HTMLAttributes<HTMLTableSectionElement>>(
  ({ className, ...props }, ref) => <tbody ref={ref} className={cn("[&_tr:last-child]:border-0", className)} {...props} />
)
TableBody.displayName = "TableBody"

const TableRow = forwardRef<HTMLTableRowElement, HTMLAttributes<HTMLTableRowElement>>(
  ({ className, ...props }, ref) => (
    <tr ref={ref} className={cn("border-b transition-colors hover:bg-muted/50", className)} {...props} />
  )
)
TableRow.displayName = "TableRow"

const TableHead = forwardRef<HTMLTableCellElement, ThHTMLAttributes<HTMLTableCellElement>>(
  ({ className, ...props }, ref) => (
    <th ref={ref} className={cn("h-12 px-4 text-left align-middle font-medium text-muted-foreground", className)} {...props} />
  )
)
TableHead.displayName = "TableHead"

const TableCell = forwardRef<HTMLTableCellElement, TdHTMLAttributes<HTMLTableCellElement>>(
  ({ className, ...props }, ref) => <td ref={ref} className={cn("p-4 align-middle", className)} {...props} />
)
TableCell.displayName = "TableCell"

export { Table, TableHeader, TableBody, TableRow, TableHead, TableCell }
```

```tsx
// frontend/src/components/ui/separator.tsx
import { type HTMLAttributes, forwardRef } from "react"
import { cn } from "../../lib/utils"

interface SeparatorProps extends HTMLAttributes<HTMLDivElement> {
  orientation?: "horizontal" | "vertical"
}

const Separator = forwardRef<HTMLDivElement, SeparatorProps>(
  ({ className, orientation = "horizontal", ...props }, ref) => (
    <div
      ref={ref}
      className={cn(
        "shrink-0 bg-border",
        orientation === "horizontal" ? "h-[1px] w-full" : "h-full w-[1px]",
        className
      )}
      {...props}
    />
  )
)
Separator.displayName = "Separator"
export { Separator }
```

- [ ] **Step 5: Verify build**

```bash
cd frontend && npm run build
```

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/ui/ frontend/src/lib/
git commit -m "feat: add shadcn/ui base components (Button, Card, Badge, Input, Table, Switch)"
```

---

## Task 3: Redesign Sidebar + App Layout

**Files:**
- Create: `frontend/src/components/sidebar.tsx`
- Create: `frontend/src/components/page-header.tsx`
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Create Sidebar component with Lucide icons**

```tsx
// frontend/src/components/sidebar.tsx
import { NavLink } from "react-router-dom"
import { cn } from "../lib/utils"
import { Play, Bot, BookOpen, Clock, BarChart3, Settings } from "lucide-react"
import { Separator } from "./ui/separator"

const NAV = [
  { to: "/", label: "Content", icon: Play },
  { to: "/agents", label: "Agents", icon: Bot },
  { to: "/knowledge", label: "Knowledge", icon: BookOpen },
  { to: "/schedules", label: "Schedules", icon: Clock },
  { to: "/analytics", label: "Analytics", icon: BarChart3 },
  { to: "/settings", label: "Settings", icon: Settings },
]

export function Sidebar() {
  return (
    <aside className="flex h-screen w-[240px] flex-col border-r bg-background sticky top-0">
      <div className="px-6 py-5">
        <span className="text-lg font-semibold tracking-tight">Ads Vance</span>
      </div>
      <Separator />
      <nav className="flex-1 space-y-1 px-3 py-4">
        {NAV.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-secondary text-foreground"
                  : "text-muted-foreground hover:bg-secondary hover:text-foreground"
              )
            }
          >
            <Icon className="h-4 w-4" />
            {label}
          </NavLink>
        ))}
      </nav>
      <div className="px-6 py-4 border-t">
        <p className="text-xs text-muted-foreground">v2.0 — Automated Pipeline</p>
      </div>
    </aside>
  )
}
```

- [ ] **Step 2: Create PageHeader component**

```tsx
// frontend/src/components/page-header.tsx
import type { ReactNode } from "react"

interface PageHeaderProps {
  title: string
  description?: string
  actions?: ReactNode
}

export function PageHeader({ title, description, actions }: PageHeaderProps) {
  return (
    <div className="flex items-center justify-between mb-8">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        {description && (
          <p className="text-sm text-muted-foreground mt-1">{description}</p>
        )}
      </div>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
  )
}
```

- [ ] **Step 3: Rewrite App.tsx with new layout**

```tsx
// frontend/src/App.tsx
import { BrowserRouter, Routes, Route } from "react-router-dom"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { Sidebar } from "./components/sidebar"
import ContentPage from "./pages/Content"
import AgentsPage from "./pages/Agents"
import KnowledgePage from "./pages/Knowledge"
import SchedulesPage from "./pages/Schedules"
import AnalyticsPage from "./pages/Analytics"
import SettingsPage from "./pages/Settings"

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
})

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <div className="flex min-h-screen bg-background">
          <Sidebar />
          <main className="flex-1 overflow-y-auto px-8 py-8 max-w-5xl">
            <Routes>
              <Route path="/" element={<ContentPage />} />
              <Route path="/agents" element={<AgentsPage />} />
              <Route path="/knowledge" element={<KnowledgePage />} />
              <Route path="/schedules" element={<SchedulesPage />} />
              <Route path="/analytics" element={<AnalyticsPage />} />
              <Route path="/settings" element={<SettingsPage />} />
            </Routes>
          </main>
        </div>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
```

- [ ] **Step 4: Verify build + dev server**

```bash
cd frontend && npm run build && npm run dev
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/App.tsx frontend/src/components/sidebar.tsx frontend/src/components/page-header.tsx
git commit -m "feat: redesign sidebar + app layout with shadcn/ui style"
```

---

## Task 4: Redesign Content Page

**Files:**
- Modify: `frontend/src/pages/Content.tsx`
- Create: `frontend/src/components/status-badge.tsx`
- Modify: `frontend/src/components/ProductionProgress.tsx`

- [ ] **Step 1: Create StatusBadge component**

```tsx
// frontend/src/components/status-badge.tsx
import { Badge } from "./ui/badge"

const STATUS_MAP: Record<string, { label: string; variant: "default" | "secondary" | "destructive" | "outline" }> = {
  published: { label: "Published", variant: "default" },
  ready: { label: "Ready", variant: "outline" },
  producing: { label: "Producing", variant: "secondary" },
  failed: { label: "Failed", variant: "destructive" },
  draft: { label: "Draft", variant: "secondary" },
}

export function StatusBadge({ status }: { status: string }) {
  const config = STATUS_MAP[status] ?? { label: status, variant: "secondary" as const }
  return <Badge variant={config.variant}>{config.label}</Badge>
}
```

- [ ] **Step 2: Rewrite Content.tsx with Table + Card components**

Full rewrite of Content.tsx using:
- `PageHeader` with action buttons (Produce, Retry, Publish)
- `Table` for clips list with `StatusBadge`
- `Button` variants for actions
- `Card` for ProductionProgress
- Lucide icons: `Plus`, `RotateCcw`, `Send`, `Trash2`

- [ ] **Step 3: Rewrite ProductionProgress.tsx with Tailwind**

Replace inline styles with Tailwind classes and Card component.

- [ ] **Step 4: Verify on dev server**

```bash
cd frontend && npm run dev
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Content.tsx frontend/src/components/
git commit -m "feat: redesign Content page with shadcn Table + StatusBadge"
```

---

## Task 5: Redesign Schedules Page

**Files:**
- Modify: `frontend/src/pages/Schedules.tsx`

- [ ] **Step 1: Rewrite with Card + Switch components**

Each schedule as a `Card` with:
- `CardHeader`: name + description (action label + cron human-readable)
- `Switch` for enable/disable toggle
- `Badge` showing "Active" / "Inactive"
- Muted text for last_run_at

- [ ] **Step 2: Verify + commit**

```bash
git add frontend/src/pages/Schedules.tsx
git commit -m "feat: redesign Schedules page with shadcn Cards + Switch"
```

---

## Task 6: Redesign Agents Page

**Files:**
- Modify: `frontend/src/pages/Agents.tsx`

- [ ] **Step 1: Rewrite with Card + Input + Textarea**

Each agent as a collapsible `Card` with:
- `CardHeader`: agent name + model badge
- `CardContent`: Textarea for system_prompt, Input for model/temperature, skills
- `Button` for save
- `Switch` for enable/disable

- [ ] **Step 2: Verify + commit**

```bash
git add frontend/src/pages/Agents.tsx
git commit -m "feat: redesign Agents page with shadcn Card + form components"
```

---

## Task 7: Redesign Knowledge Page

**Files:**
- Modify: `frontend/src/pages/Knowledge.tsx`

- [ ] **Step 1: Rewrite with Card + grouped sections**

- `PageHeader` with "Add Document" + "Rebuild Embeddings" buttons
- Group by category with section headers
- Each document as collapsible `Card`
- `Input` for name, `select` for category, `Textarea` for content
- `Button` variants for Save, Embed, Delete

- [ ] **Step 2: Verify + commit**

```bash
git add frontend/src/pages/Knowledge.tsx
git commit -m "feat: redesign Knowledge page with shadcn Card + grouped layout"
```

---

## Task 8: Redesign Analytics + Settings Pages

**Files:**
- Modify: `frontend/src/pages/Analytics.tsx`
- Modify: `frontend/src/pages/Settings.tsx`

- [ ] **Step 1: Rewrite Analytics with Card grid**

- KPI cards using `Card` with large numbers
- Use Lucide icons for metrics (Eye, ThumbsUp, MessageSquare, Share2)

- [ ] **Step 2: Rewrite Settings with grouped Card sections**

- API Keys section in `Card`
- Voice settings in `Card`
- Zernio connection in `Card`
- `Input` components for all fields
- `Button` for save/test actions

- [ ] **Step 3: Verify + commit**

```bash
git add frontend/src/pages/Analytics.tsx frontend/src/pages/Settings.tsx
git commit -m "feat: redesign Analytics + Settings pages with shadcn components"
```

---

## Task 9: Dark Mode Toggle + Final Polish

**Files:**
- Modify: `frontend/src/components/sidebar.tsx`
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Add dark mode toggle to sidebar**

Add `Moon`/`Sun` icon button in sidebar footer. Toggle `dark` class on `<html>` element. Persist preference in localStorage.

- [ ] **Step 2: Final visual QA**

- Test all 6 pages in light + dark mode
- Verify all form interactions work
- Check responsive behavior at 768px and 375px
- Verify focus states for keyboard navigation

- [ ] **Step 3: Build check + commit**

```bash
cd frontend && npm run build
git add frontend/
git commit -m "feat: add dark mode toggle + final shadcn/ui polish"
```

- [ ] **Step 4: Push to deploy**

```bash
git push origin master
```
