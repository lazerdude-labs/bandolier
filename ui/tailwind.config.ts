import type { Config } from 'tailwindcss';

const config: Config = {
  darkMode: ['selector', '[data-theme="dark"]'],
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        card: {
          DEFAULT: 'hsl(var(--card))',
          alt: 'hsl(var(--card-2))',
        },
        border: 'hsl(var(--border))',
        'border-strong': 'hsl(var(--border-strong))',
        muted: { foreground: 'hsl(var(--muted-foreground))' },
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))',
        },
        accent: 'hsl(var(--accent))',
        destructive: 'hsl(var(--destructive))',
        warning: 'hsl(var(--warning))',
        status: {
          ready: 'hsl(var(--status-ready))',
          running: 'hsl(var(--status-running))',
          degraded: 'hsl(var(--status-degraded))',
          error: 'hsl(var(--status-error))',
          pending: 'hsl(var(--status-pending))',
        },
      },
      fontFamily: {
        sans: ['Geist Variable', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        mono: ['Geist Mono', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      },
      borderRadius: { lg: '10px', md: '8px', sm: '6px' },
      keyframes: {
        'pulse-ring': {
          '0%, 100%': { opacity: '1', transform: 'scale(1)' },
          '50%': { opacity: '0.5', transform: 'scale(1.15)' },
        },
      },
      animation: { 'pulse-ring': 'pulse-ring 2s ease-in-out infinite' },
    },
  },
  plugins: [],
};

export default config;
