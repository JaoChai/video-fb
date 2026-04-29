import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '../api';
import { Card, CardHeader, CardTitle, CardContent } from './ui/card';
import { Button } from './ui/button';
import { cn } from '../lib/utils';
import { X } from 'lucide-react';

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
    <Card className="mb-5">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {status.active && (
              <span className="size-2 rounded-full bg-orange-500 animate-pulse" />
            )}
            <CardTitle className="text-sm font-semibold">
              Production Progress
            </CardTitle>
          </div>
          <div className="flex items-center gap-3">
            {!status.active && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setDismissed(true)}
                className="h-6 px-2 text-xs text-muted-foreground"
              >
                <X className="size-3" />
              </Button>
            )}
            {status.total_clips > 0 && (
              <span className="text-xs text-muted-foreground">
                Clip {status.current_clip}/{status.total_clips}
              </span>
            )}
            {totalElapsed > 0 && (
              <span className="text-xs text-muted-foreground font-mono">
                {formatTime(totalElapsed)}
              </span>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {status.clip_title && (
          <p className="text-xs text-muted-foreground mb-3 truncate">
            {status.clip_title}
          </p>
        )}

        <div className="grid gap-1.5 mb-3">
          {steps.map((step) => (
            <div key={step.name} className="flex items-center gap-2.5 py-1">
              <span
                className={cn(
                  "size-2.5 rounded-full shrink-0",
                  step.status === 'completed' && "bg-green-500",
                  step.status === 'running' && "bg-orange-500 shadow-[0_0_8px_rgba(245,133,31,0.4)]",
                  step.status === 'failed' && "bg-destructive",
                  step.status === 'pending' && "bg-muted",
                )}
              />
              <span
                className={cn(
                  "text-xs flex-1",
                  step.status === 'completed' && "text-muted-foreground",
                  step.status === 'running' && "text-foreground font-medium",
                  step.status === 'failed' && "text-destructive",
                  step.status === 'pending' && "text-muted-foreground/60",
                )}
              >
                {STEP_LABELS[step.name] || step.name}
              </span>
              {step.elapsed_seconds > 0 && (
                <span className="text-[10px] text-muted-foreground font-mono min-w-[30px] text-right">
                  {formatTime(step.elapsed_seconds)}
                </span>
              )}
              {step.status === 'completed' && (
                <span className="text-[10px] text-green-500">&#10003;</span>
              )}
              {step.status === 'failed' && (
                <span className="text-[10px] text-destructive">&#10007;</span>
              )}
            </div>
          ))}
        </div>

        <div className="h-1 rounded-full bg-muted overflow-hidden">
          <div
            className={cn(
              "h-full rounded-full transition-all duration-500",
              failedStep ? "bg-destructive" : "bg-orange-500",
            )}
            style={{ width: `${progressPercent}%` }}
          />
        </div>

        {failedStep?.error && (
          <div className="mt-2.5 p-2.5 rounded-md bg-destructive/10 border border-destructive/20 text-xs text-destructive leading-relaxed">
            {failedStep.error}
          </div>
        )}

        {status.error_logs && status.error_logs.length > 0 && (
          <div className="mt-2.5 p-2.5 rounded-md bg-destructive/10 border border-destructive/20 text-xs text-destructive leading-relaxed">
            {status.error_logs.map((log, idx) => (
              <div key={idx}>{log}</div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
