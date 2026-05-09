import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Sun, Moon, Eye, EyeOff, Lock } from 'lucide-react';
import { api } from '@/lib/api';
import { useTheme } from '@/lib/theme';

export function LoginPage() {
  const [password, setPassword] = useState('');
  const [showPwd, setShowPwd] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const nav = useNavigate();
  const { theme, toggle } = useTheme();

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErr(null); setLoading(true);
    try {
      await api('POST', '/api/auth/login', { password });
      nav({ to: '/clusters' });
    } catch (e: unknown) {
      const err = e as { body?: { error?: string } };
      setErr(err?.body?.error ?? 'login failed');
    } finally { setLoading(false); }
  };

  return (
    <div className="login-shell">
      <div className="login-card">
        <div className="login-brand">
          <span className="mark">B</span>
          <div>
            <div className="font-mono font-semibold tracking-wider">BANDOLIER</div>
            <div className="text-xs text-muted-foreground">LazerDude Labs</div>
          </div>
        </div>
        <form onSubmit={onSubmit} className="space-y-3">
          <div className="field">
            <label className="field-label">Master password</label>
            <div style={{ position: 'relative' }}>
              <input
                type={showPwd ? 'text' : 'password'}
                className="input mono"
                style={{ paddingRight: 36 }}
                autoFocus
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
              <button
                type="button"
                className="icon-btn"
                style={{ position: 'absolute', right: 4, top: 4, width: 28, height: 28 }}
                aria-label={showPwd ? 'Hide password' : 'Show password'}
                onClick={() => setShowPwd((s) => !s)}
              >
                {showPwd ? <EyeOff size={14} /> : <Eye size={14} />}
              </button>
            </div>
          </div>
          {err ? <div className="field-error">{err}</div> : null}
          <button type="submit" className="btn btn-primary w-full" disabled={loading}>
            {loading ? 'Signing in…' : 'Sign in'}
          </button>
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
