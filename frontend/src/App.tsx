import { BrowserRouter, Routes, Route, NavLink } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ContentPage from './pages/Content';
import AgentsPage from './pages/Agents';
import KnowledgePage from './pages/Knowledge';
import SchedulesPage from './pages/Schedules';
import AnalyticsPage from './pages/Analytics';
import SettingsPage from './pages/Settings';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
});

const NAV = [
  { to: '/', label: 'Content', icon: '▶' },
  { to: '/agents', label: 'Agents', icon: '◉' },
  { to: '/knowledge', label: 'Knowledge', icon: '◈' },
  { to: '/schedules', label: 'Schedules', icon: '◷' },
  { to: '/analytics', label: 'Analytics', icon: '◫' },
  { to: '/settings', label: 'Settings', icon: '◎' },
];

const navLinkStyle = (isActive: boolean): React.CSSProperties => ({
  display: 'flex',
  alignItems: 'center',
  gap: 10,
  padding: '10px 14px',
  borderRadius: 8,
  background: isActive ? 'rgba(255,255,255,0.08)' : 'transparent',
  color: isActive ? '#fff' : '#666',
  fontSize: 13,
  fontWeight: isActive ? 500 : 400,
  letterSpacing: '0.01em',
  transition: 'all 0.15s ease',
  cursor: 'pointer',
  textDecoration: 'none',
});

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <div style={{ display: 'flex', minHeight: '100vh', background: '#000' }}>
          <nav style={{
            width: 220,
            borderRight: '1px solid #141414',
            padding: '28px 14px',
            display: 'flex',
            flexDirection: 'column',
            gap: 2,
            position: 'sticky',
            top: 0,
            height: '100vh',
          }}>
            <div style={{
              fontSize: 16,
              fontWeight: 700,
              letterSpacing: '-0.03em',
              padding: '0 14px',
              marginBottom: 32,
              color: '#fff',
            }}>
              Ads Vance
            </div>
            {NAV.map(({ to, label, icon }) => (
              <NavLink key={to} to={to} style={({ isActive }) => navLinkStyle(isActive)}>
                <span style={{ fontSize: 14, width: 18, textAlign: 'center', opacity: 0.7 }}>{icon}</span>
                {label}
              </NavLink>
            ))}
            <div style={{ flex: 1 }} />
            <div style={{
              padding: '12px 14px',
              fontSize: 11,
              color: '#333',
              borderTop: '1px solid #141414',
              marginTop: 8,
            }}>
              v2.0 — Automated Pipeline
            </div>
          </nav>
          <main style={{
            flex: 1,
            padding: '36px 48px',
            maxWidth: 1200,
            overflowY: 'auto',
          }}>
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
  );
}
