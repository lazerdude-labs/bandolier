import type { DeploymentEvent } from '@/lib/ws';

type State = 'pending' | 'running' | 'done' | 'failed';

function reduce(events: DeploymentEvent[]): { name: string; state: State }[] {
  // Deploy executor emits step_end with `exit_code` (0 = success); destroy
  // executor uses `status: "succeeded" | "failed"`. Treat the step as failed
  // only when one of those signals explicitly says so; otherwise succeed.
  const m = new Map<string, State>();
  for (const e of events) {
    if (e.type === 'step_start' && e.step) m.set(e.step, 'running');
    if (e.type === 'step_end' && e.step) {
      const failed =
        e.status === 'failed' ||
        (typeof e.exit_code === 'number' && e.exit_code !== 0);
      m.set(e.step, failed ? 'failed' : 'done');
    }
  }
  return Array.from(m.entries()).map(([name, state]) => ({ name, state }));
}

export function StepList({ events }: { events: DeploymentEvent[] }) {
  const steps = reduce(events);
  return (
    <div className="steplist">
      {steps.map((s) => (
        <div key={s.name} className={`step ${s.state === 'running' ? 'active' : ''}`}>
          <span className={`step-circle ${s.state}`}>
            {s.state === 'running' ? <span className="step-spinner" /> : null}
          </span>
          <span className="step-name">{s.name}</span>
          <span className="step-meta">{s.state}</span>
        </div>
      ))}
    </div>
  );
}
