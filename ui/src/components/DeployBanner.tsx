import { CheckCircle, XCircle, Loader2, Slash } from 'lucide-react';

type Kind = 'success' | 'failed' | 'running' | 'cancelled';

export function DeployBanner({
  kind, message, action,
}: {
  kind: Kind;
  message: string;
  action?: { label: string; onClick?: () => void; disabled?: boolean; comingSoon?: boolean };
}) {
  const Icon =
    kind === 'success'   ? CheckCircle :
    kind === 'failed'    ? XCircle     :
    kind === 'cancelled' ? Slash       :
                           Loader2;
  return (
    <div className={`deploy-banner ${kind}`}>
      <Icon size={16} className={kind === 'running' ? 'animate-spin' : ''} />
      <span>{message}</span>
      <span className="spacer" />
      {action ? (
        action.comingSoon ? (
          <span className="tt-wrap">
            <button className="btn btn-outline btn-sm" disabled>{action.label}</button>
            <span className="tt">Coming soon</span>
          </span>
        ) : (
          <button className="btn btn-outline btn-sm" disabled={action.disabled} onClick={action.onClick}>
            {action.label}
          </button>
        )
      ) : null}
    </div>
  );
}
