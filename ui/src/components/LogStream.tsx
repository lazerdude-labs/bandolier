import { useMemo, useRef, useEffect, useState } from 'react';
import { Search, Pause, Play, Copy as CopyIcon } from 'lucide-react';
import type { DeploymentEvent } from '@/lib/ws';

type Tab = 'all' | 'stdout' | 'stderr' | 'ansible';
type LogLine = { id: number; stream: 'stdout' | 'stderr' | 'ansible'; text: string; ts: string };

const ansiToClass: Record<string, string> = {
  '30': 'ansi-gray', '31': 'ansi-red', '32': 'ansi-green', '33': 'ansi-yellow',
  '34': 'ansi-blue', '36': 'ansi-cyan', '37': '', '39': '', '0': '', '1': 'ansi-bold',
};

function ansi(text: string): React.ReactNode[] {
  // eslint-disable-next-line no-control-regex
  const re = /\x1b\[(\d+)m/g;
  const parts: React.ReactNode[] = [];
  let lastIndex = 0;
  let activeClass: string | null = null;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text)) !== null) {
    if (m.index > lastIndex) {
      const plain = text.slice(lastIndex, m.index);
      parts.push(activeClass
        ? <span key={parts.length} className={activeClass}>{plain}</span>
        : plain);
    }
    const code = m[1];
    activeClass = (code === '0' || code === '39') ? null : (ansiToClass[code] ?? null);
    lastIndex = m.index + m[0].length;
  }
  if (lastIndex < text.length) {
    const tail = text.slice(lastIndex);
    parts.push(activeClass
      ? <span key={parts.length} className={activeClass}>{tail}</span>
      : tail);
  }
  return parts.length ? parts : [text];
}

// Highlight a search query inside a line of text.
// Strips ANSI codes for the highlight pass (simpler than threading both together).
function highlight(text: string, query: string): React.ReactNode[] {
  if (!query) return ansi(text);
  // eslint-disable-next-line no-control-regex
  const plain = text.replace(/\x1b\[\d+m/g, '');
  const lc = plain.toLowerCase();
  const q = query.toLowerCase();
  if (!lc.includes(q)) return ansi(text);
  const out: React.ReactNode[] = [];
  let i = 0;
  let hit: number;
  let key = 0;
  while ((hit = lc.indexOf(q, i)) !== -1) {
    if (hit > i) out.push(plain.slice(i, hit));
    out.push(<mark key={`m${key++}`} className="search-highlight">{plain.slice(hit, hit + q.length)}</mark>);
    i = hit + q.length;
  }
  if (i < plain.length) out.push(plain.slice(i));
  return out;
}

export function LogStream({ events, reconnectIn }: { events: DeploymentEvent[]; reconnectIn?: number | null }) {
  const [tab, setTab] = useState<Tab>('all');
  const [search, setSearch] = useState('');
  const [autoscroll, setAutoscroll] = useState(true);
  const ref = useRef<HTMLDivElement>(null);

  const lines: LogLine[] = useMemo(() => {
    return events.flatMap((e, i) => {
      if (e.type === 'log' && e.text) {
        return [{ id: i, stream: (e.stream as 'stdout' | 'stderr') ?? 'stdout', text: e.text, ts: e.ts }];
      }
      if (e.type === 'ansible_event') {
        return [{ id: i, stream: 'ansible' as const, text: JSON.stringify(e.data), ts: e.ts }];
      }
      return [] as LogLine[];
    });
  }, [events]);

  const counts = {
    all: lines.length,
    stdout: lines.filter((l) => l.stream === 'stdout').length,
    stderr: lines.filter((l) => l.stream === 'stderr').length,
    ansible: lines.filter((l) => l.stream === 'ansible').length,
  };

  const filtered = useMemo(() => {
    let out = tab === 'all' ? lines : lines.filter((l) => l.stream === tab);
    if (search) {
      const q = search.toLowerCase();
      out = out.filter((l) => l.text.toLowerCase().includes(q));
    }
    return out;
  }, [lines, tab, search]);

  useEffect(() => {
    if (!autoscroll || !ref.current) return;
    ref.current.scrollTop = ref.current.scrollHeight;
  }, [filtered.length, autoscroll]);

  const onScroll = () => {
    if (!ref.current) return;
    const el = ref.current;
    setAutoscroll(el.scrollTop + el.clientHeight >= el.scrollHeight - 4);
  };

  const copyVisible = () => {
    const txt = filtered.map((l) => l.text).join('\n');
    navigator.clipboard?.writeText(txt);
  };

  return (
    <div className="logstream flex-1">
      <div className="logstream-toolbar">
        <div className="logstream-tabs">
          {(['all', 'stdout', 'stderr', 'ansible'] as Tab[]).map((t) => (
            <button key={t} className={`logtab ${tab === t ? 'active' : ''}`} onClick={() => setTab(t)}>
              {t} <span className="count">{counts[t]}</span>
            </button>
          ))}
        </div>

        <div className="flex-1" />

        <div className="flex items-center gap-1" style={{ paddingRight: 4 }}>
          {reconnectIn != null ? (
            <span className="text-[11px] font-mono text-amber-400 pr-1.5">
              reconnecting in {reconnectIn}s…
            </span>
          ) : null}
          <div className="flex items-center gap-1.5" style={{ height: 28, padding: '0 8px', border: '1px solid hsl(var(--border))', borderRadius: 6, background: 'hsl(var(--background) / 0.4)' }}>
            <Search size={11} className="text-muted-foreground" />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search logs…"
              spellCheck={false}
              style={{ width: 160, height: 24, background: 'transparent', border: 'none', outline: 'none', fontFamily: 'Geist Mono, monospace', fontSize: 12, color: 'hsl(var(--foreground))' }}
            />
          </div>

          <span className="tt-wrap">
            <button
              type="button"
              className="icon-btn"
              style={{ width: 26, height: 26 }}
              aria-label={autoscroll ? 'Pause autoscroll' : 'Resume autoscroll'}
              onClick={() => setAutoscroll((a) => !a)}
            >
              {autoscroll ? <Pause size={12} /> : <Play size={12} />}
            </button>
            <span className="tt">{autoscroll ? 'Autoscrolling — click to pause' : 'Paused — click to resume'}</span>
          </span>

          <span className="tt-wrap">
            <button
              type="button"
              className="icon-btn"
              style={{ width: 26, height: 26 }}
              aria-label="Copy visible lines"
              onClick={copyVisible}
            >
              <CopyIcon size={12} />
            </button>
            <span className="tt">Copy visible</span>
          </span>
        </div>
      </div>

      <div className="logstream-body" ref={ref} onScroll={onScroll}>
        {filtered.map((l) => (
          <div key={l.id} className={`logline ${l.stream}`}>
            <span className="ln">{l.id + 1}</span>
            <span className="ts">{l.ts.slice(11, 19)}</span>
            <span className="text">{highlight(l.text, search)}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
