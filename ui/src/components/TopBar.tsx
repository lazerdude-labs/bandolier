import { Link, useNavigate, useMatch } from '@tanstack/react-router';
import { History, Sun, Moon, Settings, LogOut, Activity as ActivityIcon } from 'lucide-react';
import { useTheme } from '@/lib/theme';
import { ClusterSwitcher } from './ClusterSwitcher';
import { Avatar } from './Avatar';
import type { Cluster, ProfileMeta } from '@/lib/api';

type Props = {
  clusters: Cluster[];
  currentClusterId?: string;
  profiles: ProfileMeta[];
  onLogout?: () => void;
};

export function TopBar({ clusters, currentClusterId, profiles, onLogout }: Props) {
  const { theme, toggle } = useTheme();
  const nav = useNavigate();
  const match = useMatch({ from: '/clusters/$clusterId', shouldThrow: false });
  const activeClusterId = match?.params.clusterId ?? currentClusterId;

  return (
    <header className="topbar">
      <div className="flex items-center gap-4 min-w-0">
        <Link to="/clusters" className="flex items-center gap-3">
          <span className="font-mono font-semibold tracking-wider text-sm">BANDOLIER</span>
          <span className="font-mono text-[11px] text-muted-foreground border-l border-border pl-3 ml-1">
            LazerDude Labs
          </span>
        </Link>
      </div>
      <div className="flex-1 flex justify-center">
        <ClusterSwitcher clusters={clusters} profiles={profiles} currentClusterId={activeClusterId} />
      </div>
      <div className="flex items-center gap-1">
        <button
          aria-label="Deployment history"
          className="icon-btn"
          disabled={!activeClusterId}
          onClick={() =>
            activeClusterId &&
            // T30 will register this route; cast avoids literal route type-check
            (nav as (opts: unknown) => void)({
              to: '/clusters/$clusterId/deployments',
              params: { clusterId: activeClusterId },
            })
          }
        >
          <History size={16} />
        </button>
        <Link to="/activity" aria-label="Activity log" className="icon-btn">
          <ActivityIcon size={16} />
        </Link>
        <button aria-label="Toggle theme" className="icon-btn" onClick={toggle}>
          {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
        </button>
        <Link to="/settings" aria-label="Settings" className="icon-btn">
          <Settings size={16} />
        </Link>
        <button aria-label="Logout" className="icon-btn" onClick={onLogout}>
          <LogOut size={16} />
        </button>
        <Avatar letter="O" />
      </div>
    </header>
  );
}
