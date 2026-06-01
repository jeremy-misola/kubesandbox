/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        background: 'var(--background)',
        surface: {
          DEFAULT: 'var(--surface)',
          raised: 'var(--surface-raised)',
        },
        foreground: 'var(--foreground)',
        primary: {
          DEFAULT:    'var(--primary)',
          dim:        'var(--primary-dim)',
          foreground: 'var(--primary-foreground)',
          glow:       'var(--primary-glow)',
        },
        accent: {
          DEFAULT:    'var(--accent)',
          dim:        'var(--accent-dim)',
          foreground: 'var(--accent-foreground)',
          glow:       'var(--accent-glow)',
        },
        secondary: {
          DEFAULT:    'var(--secondary)',
          dim:        'var(--secondary-dim)',
          foreground: 'var(--secondary-foreground)',
          glow:       'var(--secondary-glow)',
        },
        card: {
          DEFAULT:    'var(--card)',
          foreground: 'var(--card-foreground)',
          border:     'var(--card-border)',
        },
        muted: {
          DEFAULT:    'var(--muted)',
          foreground: 'var(--muted-foreground)',
        },
        border:  'var(--border)',
        success: 'var(--success)',
        warning: 'var(--warning)',
        error:   'var(--error)',
      },
      fontFamily: {
        sans: ['Outfit', 'Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'Fira Code', 'Consolas', 'monospace'],
      },
      boxShadow: {
        'glass-sm':    '0 2px 10px 0 rgba(0,0,0,0.35), inset 0 0 0 1px rgba(255,255,255,0.05)',
        'glass':       '0 8px 40px 0 rgba(0,0,0,0.45), inset 0 0 0 1px rgba(255,255,255,0.07)',
        'glass-neon':  '0 8px 40px 0 rgba(164,255,79,0.12), inset 0 0 0 1px rgba(164,255,79,0.18)',
        'glass-cyan':  '0 8px 40px 0 rgba(34,211,238,0.12), inset 0 0 0 1px rgba(34,211,238,0.18)',
        'glass-error': '0 8px 40px 0 rgba(248,113,113,0.1),  inset 0 0 0 1px rgba(248,113,113,0.18)',
        'neon-sm':     '0 0 12px rgba(164,255,79, 0.4)',
        'cyan-sm':     '0 0 12px rgba(34, 211,238,0.4)',
      },
      animation: {
        'fade-in':     'fade-in 0.35s ease both',
        'card-in':     'card-in 0.45s cubic-bezier(0.22,1,0.36,1) both',
        'nav-in':      'nav-in  0.4s  cubic-bezier(0.22,1,0.36,1) both',
        'float':       'float 6s ease-in-out infinite',
        'spin-slow':   'spin  3s linear infinite',
        'ping-slow':   'ping  2s cubic-bezier(0,0,0.2,1) infinite',
      },
      backdropBlur: {
        'glass': '20px',
      },
    },
  },
  plugins: [],
}
