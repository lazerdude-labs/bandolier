import { Link } from '@tanstack/react-router';
import type { ReactNode } from 'react';

export type Action = {
  key: string;
  label: string;
  onClick?: () => void;
  href?: { to: string; params?: Record<string, string> };
  primary?: boolean;
  destructive?: boolean;
  disabled?: boolean;
  disabledTooltip?: string;
  comingSoon?: boolean;     // shows Coming-soon tooltip when true
  small?: boolean;          // renders btn-sm (kubeconfig, helm)
  icon?: ReactNode;
  /** Inserts a vertical divider before this button. Used to separate
      primary from secondary, then secondary from small. */
  dividerBefore?: boolean;
  /** Inserts a flexible spacer before this button. Used to push small
      actions (kubeconfig/helm) to the right. */
  spacerBefore?: boolean;
};

export function ActionBar({ actions }: { actions: Action[] }) {
  return (
    <div className="action-bar">
      {actions.map((a) => {
        const sizeCls = a.small ? 'btn-sm' : '';
        const tone = a.primary ? 'btn-primary' : a.destructive ? 'btn-destructive' : 'btn-outline';
        const cls = `btn ${tone} ${sizeCls}`.trim();
        const isDisabled = a.disabled || a.comingSoon;
        const tooltip = a.disabledTooltip ?? (a.comingSoon ? 'Coming soon' : undefined);

        const button = isDisabled && tooltip ? (
          <span key={a.key} className="tt-wrap">
            <button className={cls} disabled>{a.icon}{a.label}</button>
            <span className="tt">{tooltip}</span>
          </span>
        ) : a.href ? (
          <Link
            key={a.key}
            to={a.href.to as never}
            params={a.href.params as never}
            className={cls}
          >{a.icon}{a.label}</Link>
        ) : (
          <button key={a.key} className={cls} disabled={isDisabled} onClick={a.onClick}>
            {a.icon}{a.label}
          </button>
        );

        return (
          <span key={a.key} className="contents">
            {a.dividerBefore ? <span key={`${a.key}-div`} className="divider" /> : null}
            {a.spacerBefore ? <span key={`${a.key}-sp`} className="spacer" /> : null}
            {button}
          </span>
        );
      })}
    </div>
  );
}

// Back-compat alias for any straggler imports.
export const ActionRail = ActionBar;
