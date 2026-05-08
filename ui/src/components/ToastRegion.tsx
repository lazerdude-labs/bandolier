import { useToasts, type Toast } from '@/store/toasts';

const kindClass: Record<Toast['kind'], string> = {
  success: 'border-l-status-ready',
  error: 'border-l-destructive',
  info: 'border-l-accent',
};

export function ToastRegion() {
  const { toasts, dismiss } = useToasts();
  return (
    <div className="fixed top-[72px] right-6 z-[80] flex flex-col gap-2">
      {toasts.map((t) => (
        <div
          key={t.id}
          role="status"
          className={`flex gap-2.5 px-3.5 py-3 bg-card border border-border ${kindClass[t.kind]} border-l-4 rounded-md min-w-[320px] max-w-[420px] text-[13px] shadow-lg cursor-pointer`}
          onClick={() => dismiss(t.id)}
        >
          <div className="flex-1">
            <div className="font-medium">{t.title}</div>
            {t.body ? <div className="text-muted-foreground mt-0.5">{t.body}</div> : null}
          </div>
        </div>
      ))}
    </div>
  );
}
