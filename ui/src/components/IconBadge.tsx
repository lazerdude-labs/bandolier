import type { LucideIcon } from 'lucide-react';

const accentToBg: Record<string, string> = {
  emerald: 'hsl(158 70% 52% / 0.14)',
  rose:    'hsl(0 72% 55% / 0.14)',
  sky:     'hsl(217 91% 60% / 0.14)',
  amber:   'hsl(38 92% 50% / 0.14)',
};
const accentToFg: Record<string, string> = {
  emerald: 'hsl(158 70% 52%)',
  rose:    'hsl(0 72% 55%)',
  sky:     'hsl(217 91% 60%)',
  amber:   'hsl(38 92% 50%)',
};

export function IconBadge({ icon: Icon, accent, size = 24 }: { icon: LucideIcon; accent?: string; size?: number }) {
  const a = accent ?? 'emerald';
  return (
    <span
      className="icon-badge"
      style={{ background: accentToBg[a] ?? accentToBg.emerald, color: accentToFg[a] ?? accentToFg.emerald, width: size, height: size }}
    >
      <Icon size={size * 0.6} />
    </span>
  );
}
