import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import { ChevronDown, Search, Check } from 'lucide-react';
import type { Cluster, ProfileMeta } from '@/lib/api';
import { StatusBadge, type ClusterStatus } from './StatusBadge';

const accentToHsl: Record<string, string> = {
  emerald: 'hsl(158 70% 52%)',
  rose:    'hsl(0 72% 55%)',
  sky:     'hsl(217 91% 60%)',
  amber:   'hsl(38 92% 50%)',
};

function profileAccent(profiles: ProfileMeta[], name: string): string {
  const p = profiles.find((x) => x.name === name);
  return accentToHsl[p?.accent ?? 'emerald'] ?? accentToHsl.emerald;
}

function profileLabel(profiles: ProfileMeta[], name: string): string {
  return profiles.find((x) => x.name === name)?.label ?? name;
}


export function ClusterSwitcher({
  clusters,
  profiles,
  currentClusterId,
}: {
  clusters: Cluster[];
  profiles: ProfileMeta[];
  currentClusterId?: string;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);
  const navigate = useNavigate();

  const current = clusters.find((c) => c.id === currentClusterId);
  const label = current?.name ?? 'All clusters';
  const profileLab = current ? profileLabel(profiles, current.profile) : '';
  const dot = current ? profileAccent(profiles, current.profile) : 'hsl(var(--status-ready))';

  // Filter + group clusters by profile (preserving the order of profiles[])
  const grouped = useMemo(() => {
    const q = query.trim().toLowerCase();
    const filtered = clusters.filter(
      (c) =>
        !q ||
        c.name.toLowerCase().includes(q) ||
        (c.network?.fqdn ?? '').toLowerCase().includes(q),
    );
    const out: Array<{ profile: ProfileMeta; rows: Cluster[] }> = [];
    for (const p of profiles) {
      const rows = filtered.filter((c) => c.profile === p.name);
      if (rows.length > 0) out.push({ profile: p, rows });
    }
    return out;
  }, [clusters, profiles, query]);

  // Auto-focus the search input when dropdown opens; Escape closes it.
  useEffect(() => {
    if (!open) return;
    const id = setTimeout(() => inputRef.current?.focus(), 30);
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('keydown', onKey);
    return () => {
      clearTimeout(id);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  // Global ⌘K / Ctrl-K opens the dropdown.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && (e.key === 'k' || e.key === 'K')) {
        e.preventDefault();
        setOpen(true);
      }
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, []);

  return (
    <div className="relative">
      <button
        onClick={() => setOpen((o) => !o)}
        className="cluster-pill"
        aria-label="Switch cluster"
        aria-expanded={open}
      >
        <span className="dot" style={{ background: dot }} />
        <span>{label}</span>
        {profileLab ? (
          <>
            <span className="sep">·</span>
            <span className="profile-label">{profileLab}</span>
          </>
        ) : null}
        <ChevronDown size={12} />
      </button>
      {open ? (
        <div
          className="dropdown-panel"
          style={{ top: 38, left: '50%', transform: 'translateX(-50%)' }}
          onMouseLeave={() => setOpen(false)}
        >
          <div className="dropdown-search">
            <Search size={12} className="text-muted-foreground" />
            <input
              ref={inputRef}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Switch cluster…"
              spellCheck={false}
            />
            <span className="kbd">⌘K</span>
          </div>
          <div className="dropdown-list">
            {grouped.length === 0 ? (
              <div className="px-3 py-3 text-xs text-muted-foreground">No clusters match.</div>
            ) : null}
            {grouped.map((g) => (
              <div key={g.profile.name} className="dropdown-group">
                <div className="dropdown-group-header label-tiny">
                  {g.profile.label}
                  {g.profile.tag ? <> · {g.profile.tag}</> : null}
                </div>
                {g.rows.map((c) => {
                  const active = c.id === currentClusterId;
                  return (
                    <button
                      key={c.id}
                      type="button"
                      className={`dropdown-row ${active ? 'active' : ''}`}
                      onClick={() => {
                        setOpen(false);
                        navigate({ to: '/clusters/$clusterId', params: { clusterId: c.id } });
                      }}
                    >
                      <span className="dot" style={{ background: profileAccent(profiles, c.profile) }} />
                      <span className="dropdown-row-text">
                        <span className="dropdown-row-name">{c.name}</span>
                        <span className="dropdown-row-sub">
                          {c.network?.fqdn ?? '—'}
                          {c.node_count != null ? <> · {c.node_count} nodes</> : null}
                        </span>
                      </span>
                      <span className="flex items-center gap-2">
                        <StatusBadge status={c.status as ClusterStatus} />
                        {active ? <Check size={14} className="text-primary" /> : null}
                      </span>
                    </button>
                  );
                })}
              </div>
            ))}
          </div>
          <div className="dropdown-footer">
            <Link to="/clusters" className="btn btn-ghost btn-sm flex-1" onClick={() => setOpen(false)}>
              All clusters
            </Link>
            <Link to="/clusters/new" className="btn btn-ghost btn-sm flex-1" onClick={() => setOpen(false)}>
              + New cluster
            </Link>
          </div>
        </div>
      ) : null}
    </div>
  );
}
