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
  { to: '/', label: 'Content' },
  { to: '/agents', label: 'Agents' },
  { to: '/knowledge', label: 'Knowledge' },
  { to: '/schedules', label: 'Schedules' },
  { to: '/analytics', label: 'Analytics' },
  { to: '/settings', label: 'Settings' },
];

const navLinkStyle = (isActive: boolean): React.CSSProperties => ({
  display: 'block',
  padding: '8px 16px',
  borderRadius: 6,
  background: isActive ? '#fff' : 'transparent',
  color: isActive ? '#000' : '#888',
  fontSize: 14,
  fontWeight: isActive ? 600 : 400,
  transition: 'all 0.15s ease',
  cursor: 'pointer',
});

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <div style={{ display: 'flex', minHeight: '100vh' }}>
          <nav style={{
            width: 200, borderRight: '1px solid #1a1a1a',
            padding: '24px 12px', display: 'flex', flexDirection: 'column', gap: 2,
            position: 'sticky', top: 0, height: '100vh',
          }}>
            <div style={{ fontSize: 15, fontWeight: 700, letterSpacing: '-0.02em', padding: '0 16px', marginBottom: 24 }}>
              Ads Vance
            </div>
            {NAV.map(({ to, label }) => (
              <NavLink key={to} to={to} style={({ isActive }) => navLinkStyle(isActive)}>
                {label}
              </NavLink>
            ))}
          </nav>
          <main style={{ flex: 1, padding: '32px 40px', maxWidth: 1200, overflowY: 'auto' }}>
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
