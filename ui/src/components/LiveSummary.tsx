import { Eye } from 'lucide-react';

export type SummarySection = {
  title: string;
  items: Array<{ label: string; value?: string | null; mono?: boolean }>;
};

export function LiveSummary({ sections }: { sections: SummarySection[] }) {
  return (
    <div className="card card-pad live-summary">
      <div className="flex items-center gap-2 mb-1">
        <Eye size={14} className="text-muted-foreground" />
        <span className="card-title">Live summary</span>
      </div>
      <p className="text-[11px] text-muted-foreground mb-3">Sanity-check before submit. Updates as you type.</p>
      {sections.map((s) => (
        <div key={s.title} className="live-summary-section">
          <div className="label-tiny">{s.title}</div>
          <dl className="kv-grid kv-grid-sm">
            {s.items.map((it) => (
              <span key={it.label} className="contents">
                <dt>{it.label}</dt>
                <dd className={it.mono === false ? '' : 'mono'}>
                  {it.value && it.value.length > 0 ? it.value : <span className="text-muted-foreground">—</span>}
                </dd>
              </span>
            ))}
          </dl>
        </div>
      ))}
    </div>
  );
}
