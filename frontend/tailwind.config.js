/** @type {import('tailwindcss').Config} */
export default {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        border: "hsl(var(--border))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        success: "hsl(var(--success))",
        warning: "hsl(var(--warning))",
        danger: "hsl(var(--danger))",
      },
      fontFamily: {
        // Luxury/Editorial pairing. Playfair Display (high-contrast serif) for
        // headlines and emphasis; Inter (humanist sans) for all body/UI;
        // JetBrains Mono reserved for terminal/data readouts.
        display: ['"Playfair Display"', "ui-serif", "Georgia", "serif"],
        sans: ['"Inter"', "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ['"JetBrains Mono"', "ui-monospace", "SFMono-Regular", "monospace"],
      },
      // Architectural precision: strictly rectangular. `rounded-full` keeps
      // Tailwind's default (functional status dots); everything else squares.
      borderRadius: {
        sm: "0px",
        md: "0px",
        lg: "0px",
        xl: "0px",
      },
      letterSpacing: {
        // Wide tracking is the core editorial tell on uppercase labels.
        label: "0.25em",
        overline: "0.3em",
        button: "0.2em",
      },
      boxShadow: {
        // Subtle, layered depth — never harsh drops.
        card: "0 2px 8px rgba(0,0,0,0.03)",
        "card-hover": "0 8px 24px rgba(0,0,0,0.07)",
        hero: "0 8px 32px rgba(0,0,0,0.12)",
        cta: "0 4px 16px rgba(0,0,0,0.15)",
        "cta-hover": "0 8px 24px rgba(0,0,0,0.25)",
      },
      transitionTimingFunction: {
        // Cinematic luxury easing — smooth, deliberate, never mechanical.
        luxury: "cubic-bezier(0.25, 0.46, 0.45, 0.94)",
      },
    },
  },
  plugins: [],
};
