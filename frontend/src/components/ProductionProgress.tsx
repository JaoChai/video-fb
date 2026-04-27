import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';

interface Step {
  name: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  elapsed_seconds: number;
  error?: string;
}

interface ProductionStatus {
  active: boolean;
  current_clip: number;
  total_clips: number;
  clip_title: string;
  steps: Step[];
  error_logs?: string[];
}

const STEP_LABELS: Record<string, string> = {
  question: 'Question Agent',
  script: 'Script Agent',
  image_prompts: 'Image Prompts',
  voice: 'Voice Generation',
  images: 'Image Generation',
  assembly: 'Video Assembly',
  upload: 'Upload Files',
  complete: 'Complete',
};

function formatTime(seconds: number): string {
  if (seconds <= 0) return '';
  if (seconds < 60) return `${Math.round(seconds)}s`;
  return `${Math.floor(seconds / 60)}m ${Math.round(seconds % 60)}s`;
}

export default function ProductionProgress() {
  const [dismissed, setDismissed] = useState(false);
  const { data: status } = useQuery({
    queryKey: ['production-status'],
    queryFn: () => apiFetch<ProductionStatus>('/api/v1/production/status'),
    refetchInterval: (query) => {
      const active = query.state.data?.active;
      if (active && dismissed) setDismissed(false);
      return active ? 2000 : 10000;
    },
  });

  if (dismissed || !status || (!status.active && (!status.steps || status.steps.length === 0))) {
    return null;
  }

  const steps = status.steps ?? [];
  const completedSteps = steps.filter(s => s.status === 'completed').length;
  const progressPercent = Math.round((completedSteps / (steps.length || 1)) * 100);
  const totalElapsed = steps.reduce((sum, s) => sum + s.elapsed_seconds, 0);
  const failedStep = steps.find(s => s.status === 'failed');

  return (
    <div style={{
      background: '#111', borderRadius: 8, border: '1px solid #222',
      padding: '16px 18px', marginBottom: 20,
    }}>
      <div style={{
        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        marginBottom: 14,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {status.active && (
            <span style={{
              width: 8, height: 8, borderRadius: '50%', background: '#f5851f',
              animation: 'pulse 1.5s ease-in-out infinite',
            }} />
          )}
          <span style={{ fontSize: 13, fontWeight: 600, color: '#fafafa' }}>
            Production Progress
          </span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          {!status.active && (
            <button onClick={() => setDismissed(true)} style={{
              background: 'none', border: '1px solid #333', borderRadius: 4,
              color: '#555', fontSize: 11, padding: '2px 8px', cursor: 'pointer',
            }}>Dismiss</button>
          )}
          {status.total_clips > 0 && (
            <span style={{ fontSize: 11, color: '#888' }}>
              Clip {status.current_clip}/{status.total_clips}
            </span>
          )}
          {totalElapsed > 0 && (
            <span style={{ fontSize: 11, color: '#555', fontFamily: 'monospace' }}>
              {formatTime(totalElapsed)}
            </span>
          )}
        </div>
      </div>

      {status.clip_title && (
        <div style={{
          fontSize: 12, color: '#aaa', marginBottom: 14,
          overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis',
        }}>
          {status.clip_title}
        </div>
      )}

      <div style={{ display: 'grid', gap: 6, marginBottom: 14 }}>
        {steps.map((step) => (
          <div key={step.name} style={{
            display: 'flex', alignItems: 'center', gap: 10,
            padding: '4px 0',
          }}>
            <span style={{
              width: 10, height: 10, borderRadius: '50%', flexShrink: 0,
              background:
                step.status === 'completed' ? '#22c55e' :
                step.status === 'running' ? '#f5851f' :
                step.status === 'failed' ? '#ef4444' :
                '#333',
              boxShadow: step.status === 'running' ? '0 0 8px #f5851f66' : 'none',
            }} />
            <span style={{
              fontSize: 12, flex: 1,
              color:
                step.status === 'completed' ? '#888' :
                step.status === 'running' ? '#fafafa' :
                step.status === 'failed' ? '#ef4444' :
                '#444',
              fontWeight: step.status === 'running' ? 500 : 400,
            }}>
              {STEP_LABELS[step.name] || step.name}
            </span>
            {step.elapsed_seconds > 0 && (
              <span style={{ fontSize: 10, color: '#555', fontFamily: 'monospace', minWidth: 30, textAlign: 'right' }}>
                {formatTime(step.elapsed_seconds)}
              </span>
            )}
            {step.status === 'completed' && (
              <span style={{ fontSize: 10, color: '#22c55e' }}>✓</span>
            )}
            {step.status === 'failed' && (
              <span style={{ fontSize: 10, color: '#ef4444' }}>✗</span>
            )}
          </div>
        ))}
      </div>

      <div style={{
        height: 4, borderRadius: 2, background: '#222', overflow: 'hidden',
      }}>
        <div style={{
          height: '100%', borderRadius: 2,
          background: failedStep ? '#ef4444' : '#f5851f',
          width: `${progressPercent}%`,
          transition: 'width 0.5s ease',
        }} />
      </div>

      {failedStep?.error && (
        <div style={{
          marginTop: 10, padding: '8px 10px', borderRadius: 6,
          background: '#1a0000', border: '1px solid #331111',
          fontSize: 11, color: '#ef4444', lineHeight: 1.4,
        }}>
          {failedStep.error}
        </div>
      )}

      {status.error_logs && status.error_logs.length > 0 && (
        <div style={{
          marginTop: 10, padding: '8px 10px', borderRadius: 6,
          background: '#1a0000', border: '1px solid #331111',
          fontSize: 11, color: '#ef4444', lineHeight: 1.4,
        }}>
          {status.error_logs.map((log, idx) => (
            <div key={idx}>{log}</div>
          ))}
        </div>
      )}

      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.4; }
        }
      `}</style>
    </div>
  );
}
