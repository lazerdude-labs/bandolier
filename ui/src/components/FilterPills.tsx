type Pill = { value: string; label: string; count?: number; dotColor?: string };

export function FilterPills({
  value, onChange, pills,
}: {
  value: string;
  onChange: (v: string) => void;
  pills: Pill[];
}) {
  return (
    <div className="filter-pills">
      {pills.map((p) => (
        <button
          key={p.value}
          onClick={() => onChange(p.value)}
          className={`filter-pill ${value === p.value ? 'active' : ''}`}
          aria-pressed={value === p.value}
        >
          {p.dotColor ? <span className="dot" style={{ background: p.dotColor }} /> : null}
          <span>{p.label}</span>
          {p.count !== undefined ? <span className="filter-pill-count">{p.count}</span> : null}
        </button>
      ))}
    </div>
  );
}
