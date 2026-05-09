import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Sun, Moon, Eye, EyeOff, Lock, Key } from 'lucide-react';
import { api } from '@/lib/api';
import { useTheme } from '@/lib/theme';

export function SetupPage() {
  const [pw, setPw] = useState('');
  const [confirm, setConfirm] = useState('');
  const [showPw, setShowPw] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const nav = useNavigate();
  const { theme, toggle } = useTheme();

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErr(null);
    if (pw !== confirm) { setErr('passwords do not match'); return; }
    if (pw.length < 12) { setErr('password must be at least 12 characters'); return; }
    try { await api('POST', '/api/auth/setup', { password: pw }); nav({ to: '/login' }); }
    catch (e: unknown) {
      const err = e as { body?: { error?: string } };
      setErr(err?.body?.error ?? 'setup failed');
    }
  };

  return (
    <div className="login-shell">
      <div className="login-card space-y-4">
        <div className="login-brand">
          <span className="mark">B</span>
          <div>
            <div className="font-mono font-semibold tracking-wider">BANDOLIER</div>
            <div className="text-xs text-muted-foreground">First-run setup · LazerDude Labs · v1.0.0</div>
          </div>
        </div>

        {/* Vault unseal keys info box */}
        <div style={{ padding: 12, background: 'hsl(var(--card-2))', border: '1px solid hsl(var(--border))', borderRadius: 8 }}>
          <div className="flex items-start gap-2">
            <Key size={14} className="text-muted-foreground mt-0.5" />
            <div>
              <div className="text-[12px] font-mono mb-1">Vault unseal keys (5 of 5, 3-threshold)</div>
              <div className="text-[11px] text-muted-foreground leading-relaxed">
                After login, Vault generates 5 unseal keys. Save at least 3 — they're shown once.
                The auto-unseal share is encrypted with this password.
              </div>
            </div>
          </div>
        </div>

        <form onSubmit={onSubmit} className="space-y-3">
          <div className="field">
            <label className="field-label">Master password</label>
            <div style={{ position: 'relative' }}>
              <input
                type={showPw ? 'text' : 'password'}
                className="input mono"
                style={{ paddingRight: 36 }}
                autoFocus
                value={pw}
                onChange={(e) => setPw(e.target.value)}
              />
              <button
                type="button"
                className="icon-btn"
                style={{ position: 'absolute', right: 4, top: 4, width: 28, height: 28 }}
                aria-label={showPw ? 'Hide password' : 'Show password'}
                onClick={() => setShowPw((s) => !s)}
              >
                {showPw ? <EyeOff size={14} /> : <Eye size={14} />}
              </button>
            </div>
          </div>
          <div className="field">
            <label className="field-label">Confirm</label>
            <div style={{ position: 'relative' }}>
              <input
                type={showConfirm ? 'text' : 'password'}
                className="input mono"
                style={{ paddingRight: 36 }}
                value={confirm}
                onChange={(e) => setConfirm(e.target.value)}
              />
              <button
                type="button"
                className="icon-btn"
                style={{ position: 'absolute', right: 4, top: 4, width: 28, height: 28 }}
                aria-label={showConfirm ? 'Hide password' : 'Show password'}
                onClick={() => setShowConfirm((s) => !s)}
              >
                {showConfirm ? <EyeOff size={14} /> : <Eye size={14} />}
              </button>
            </div>
          </div>
          {err ? <div className="field-error">{err}</div> : null}
          <button type="submit" className="btn btn-primary w-full">Continue</button>
        </form>

        <div className="flex items-center justify-between text-[11px] text-muted-foreground mt-4 pt-4" style={{ borderTop: '1px solid hsl(var(--border))' }}>
          <div className="flex items-center gap-1.5"><Lock size={11} /><span className="font-mono">127.0.0.1:443 · self-signed TLS</span></div>
          <button className="hover:text-foreground" disabled style={{ opacity: 0.6 }}>vault sealed?</button>
        </div>
      </div>
      <button onClick={toggle} className="icon-btn fixed bottom-5 right-5" aria-label="Toggle theme">
        {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
      </button>
    </div>
  );
}
