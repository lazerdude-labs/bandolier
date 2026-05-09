import { useState } from 'react';
import { useQuery, useMutation } from '@tanstack/react-query';
import { useNavigate, Link } from '@tanstack/react-router';
import { Shield, Flag, Eye, AlertTriangle, ArrowRight, type LucideIcon } from 'lucide-react';
import { listProfiles, api, type ProfileMeta , errMessage } from '@/lib/api';
import { useToasts } from '@/store/toasts';
import { newClusterSchema } from '@/schemas/profile';
import { IconBadge } from '@/components/IconBadge';
import { ComingSoonPill } from '@/components/ComingSoonPill';

const iconMap: Record<string, LucideIcon> = {
  shield: Shield, flag: Flag, eye: Eye, 'alert-triangle': AlertTriangle,
};

export function ClustersNew() {
  const profiles = useQuery({ queryKey: ['profiles'], queryFn: listProfiles });
  const [profile, setProfile] = useState<string>('homelab');
  const [name, setName] = useState('');
  const push = useToasts((s) => s.push);
  const navigate = useNavigate();

  const create = useMutation({
    mutationFn: async (v: { name: string; profile: string }) =>
      api<{ id: string }>('POST', '/api/clusters', v),
    onSuccess: (cluster) => {
      try {
        navigate({ to: '/clusters/$clusterId/initialize', params: { clusterId: cluster.id } });
      } catch {
        window.location.href = `/clusters/${cluster.id}/initialize`;
      }
    },
    onError: (err: unknown) =>
      push({ kind: 'error', title: 'Could not create cluster', body: errMessage(err, 'unknown') }),
  });

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const parsed = newClusterSchema.safeParse({ name, profile });
    if (!parsed.success) {
      push({ kind: 'error', title: 'Invalid input', body: parsed.error.issues[0].message });
      return;
    }
    create.mutate(parsed.data);
  };

  const profileList = profiles.data ?? [];

  return (
    <div className="space-y-6 page-narrow">
      <div className="crumbs">
        <Link to="/clusters">Clusters</Link>
        <span className="sep">/</span>
        <span>New</span>
      </div>
      <div>
        <h1 className="h1">New cluster</h1>
        <p className="text-[13px] text-muted-foreground mt-1">
          Pick a profile. Profiles bundle Terraform modules, Ansible playbooks, and Helm charts that fit a scenario.
        </p>
      </div>

      <form onSubmit={onSubmit} className="space-y-6">
        {/* Profile picker grid */}
        <div className="grid grid-cols-2 gap-3">
          {profileList.map((p) => (
            <ProfilePickerCard
              key={p.name}
              profile={p}
              selected={profile === p.name}
              onSelect={() => p.enabled && setProfile(p.name)}
            />
          ))}
        </div>

        {/* Name input */}
        <div className="card card-pad space-y-3">
          <div className="card-title">Cluster name</div>
          <div className="field">
            <input
              id="cluster-name"
              className="input mono"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={profile === 'homelab' ? 'homelab-prod' : `${profile}-alpha`}
            />
            <span className="field-hint">
              Will be created at <span className="font-mono">vault://clusters/{`{cluster-id}`}/*</span>. Lowercase letters, digits, hyphens.
            </span>
          </div>
        </div>

        {/* Submit */}
        <div className="flex justify-end gap-2">
          <Link to="/clusters" className="btn btn-ghost">Cancel</Link>
          <button type="submit" className="btn btn-primary" disabled={create.isPending || !name}>
            {create.isPending ? 'Creating…' : 'Create & configure'}
            {!create.isPending ? <ArrowRight size={14} /> : null}
          </button>
        </div>
      </form>
    </div>
  );
}

function ProfilePickerCard({
  profile, selected, onSelect,
}: {
  profile: ProfileMeta;
  selected: boolean;
  onSelect: () => void;
}) {
  const Icon = iconMap[profile.icon] ?? Shield;
  return (
    <button
      type="button"
      disabled={!profile.enabled}
      onClick={onSelect}
      className={`card card-pad text-left ${selected ? 'ring-2 ring-primary' : ''} ${!profile.enabled ? 'opacity-60 cursor-not-allowed' : ''}`}
      style={{ position: 'relative' }}
    >
      <div className="flex items-center gap-3 mb-2">
        <IconBadge icon={Icon} accent={profile.accent} size={36} />
        <div className="flex-1 min-w-0">
          <div className="font-mono text-sm font-semibold">{profile.label}</div>
          {profile.tag ? <span className="tag-badge mt-1">{profile.tag}</span> : null}
        </div>
      </div>
      <div className="text-xs text-muted-foreground leading-relaxed">{profile.description}</div>
      {!profile.enabled ? <div className="mt-3"><ComingSoonPill /></div> : null}
    </button>
  );
}
