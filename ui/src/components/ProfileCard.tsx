import { Shield, Flag, Eye, AlertTriangle, type LucideIcon } from 'lucide-react';
import { IconBadge } from './IconBadge';
import { ComingSoonPill } from './ComingSoonPill';
import type { ProfileMeta } from '@/lib/api';

const iconMap: Record<string, LucideIcon> = {
  shield: Shield,
  flag: Flag,
  eye: Eye,
  'alert-triangle': AlertTriangle,
};

type Props = {
  profile: ProfileMeta;
  count: number;
  ready: number;
  active?: boolean;
  onClick?: () => void;
};

/**
 * Fleet-page profile summary card. Big number + ready fraction + tag pill.
 * Disabled (v3) profiles render with reduced opacity and a "Coming soon" pill
 * in place of the ready fraction.
 */
export function ProfileCard({ profile, count, ready, active, onClick }: Props) {
  const Icon = iconMap[profile.icon] ?? Shield;
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={!profile.enabled}
      className={`profile-summary ${active ? 'active' : ''}`}
      style={!profile.enabled ? { opacity: 0.6, cursor: 'not-allowed' } : undefined}
    >
      <div className="profile-summary-head">
        <IconBadge icon={Icon} accent={profile.accent} size={24} />
        <span className="profile-summary-label">{profile.label}</span>
        {profile.tag ? <span className="tag-badge">{profile.tag}</span> : null}
      </div>
      <div className="profile-summary-row">
        <div className="profile-summary-count">
          <span className="num">{count}</span>
          <span className="unit">{count === 1 ? 'cluster' : 'clusters'}</span>
        </div>
        {profile.enabled ? (
          <div className="profile-summary-ready">{ready}/{count} ready</div>
        ) : (
          <ComingSoonPill />
        )}
      </div>
    </button>
  );
}
