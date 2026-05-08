import { Check } from 'lucide-react';

type Step = { name: string; subtitle?: string };

export function Stepper({
  steps, currentIndex, onJump,
}: {
  steps: Step[];
  currentIndex: number;
  onJump?: (index: number) => void;
}) {
  return (
    <div className="stepper-vert">
      {steps.map((s, i) => {
        const state = i < currentIndex ? 'done' : i === currentIndex ? 'active' : 'pending';
        const Comp: any = onJump ? 'button' : 'div';
        return (
          <Comp
            key={s.name}
            className={`stepper-item ${state}`}
            onClick={onJump ? () => onJump(i) : undefined}
            type={onJump ? 'button' : undefined}
          >
            <span className={`stepper-circle ${state}`}>
              {state === 'done' ? <Check size={11} /> : <span className="num">{i + 1}</span>}
            </span>
            <span className="stepper-text">
              <span className="stepper-name">{s.name}</span>
              {s.subtitle ? <span className="stepper-sub">{s.subtitle}</span> : null}
            </span>
          </Comp>
        );
      })}
    </div>
  );
}
