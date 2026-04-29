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
