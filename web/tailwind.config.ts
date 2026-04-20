import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./src/pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        bg: {
          DEFAULT: "var(--bg)",
          primary: "var(--bg-primary)",
          secondary: "var(--bg-secondary)",
          elevated: "var(--bg-elevated)",
          inset: "var(--bg-inset)",
          2: "var(--bg-2)",
        },
        paper: "var(--paper)",
        ink: {
          DEFAULT: "var(--ink)",
          2: "var(--ink-2)",
        },
        muted: {
          DEFAULT: "var(--muted)",
          2: "var(--muted-2)",
        },
        line: {
          DEFAULT: "var(--line)",
          2: "var(--line-2)",
        },
        text: {
          primary: "var(--text-primary)",
          secondary: "var(--text-secondary)",
          tertiary: "var(--text-tertiary)",
          inverse: "var(--text-inverse)",
        },
        border: {
          DEFAULT: "var(--border-default)",
          subtle: "var(--border-subtle)",
          strong: "var(--border-strong)",
        },
        accent: {
          DEFAULT: "var(--accent)",
          hover: "var(--amber-hover)",
          muted: "var(--amber-muted)",
          subtle: "var(--amber-subtle)",
          soft: "var(--accent-soft)",
        },
        ok: "var(--ok)",
        danger: "var(--danger)",
        status: {
          active: "var(--status-active)",
          "active-bg": "var(--status-active-bg)",
          closed: "var(--status-closed)",
          "closed-bg": "var(--status-closed-bg)",
          archived: "var(--status-archived)",
          "archived-bg": "var(--status-archived-bg)",
          hold: "var(--status-hold)",
          "hold-bg": "var(--status-hold-bg)",
          live: "var(--status-live)",
          disc: "var(--status-disc)",
          broken: "var(--status-broken)",
          pseud: "var(--status-pseud)",
        },
        av: {
          a: "var(--av-a)",
          b: "var(--av-b)",
          c: "var(--av-c)",
          d: "var(--av-d)",
          e: "var(--av-e)",
        },
      },
      fontSize: {
        xs: "var(--text-xs)",
        sm: "var(--text-sm)",
        base: "var(--text-base)",
        lg: "var(--text-lg)",
        xl: "var(--text-xl)",
        "2xl": "var(--text-2xl)",
        "3xl": "var(--text-3xl)",
      },
      spacing: {
        xs: "var(--space-xs)",
        sm: "var(--space-sm)",
        md: "var(--space-md)",
        lg: "var(--space-lg)",
        xl: "var(--space-xl)",
        "2xl": "var(--space-2xl)",
      },
      borderRadius: {
        sm: "var(--radius-sm)",
        DEFAULT: "var(--radius)",
        md: "var(--radius-md)",
        lg: "var(--radius-lg)",
        xl: "var(--radius-xl)",
        full: "var(--radius-full)",
      },
      fontFamily: {
        heading: ['var(--font-heading)', 'Fraunces', 'serif'],
        body: ['var(--font-body)', 'Inter', 'sans-serif'],
        mono: ['var(--font-mono)', 'JetBrains Mono', 'monospace'],
      },
      transitionTimingFunction: {
        "out-expo": "var(--ease-out-expo)",
      },
      transitionDuration: {
        fast: "var(--duration-fast)",
        normal: "var(--duration-normal)",
        slow: "var(--duration-slow)",
      },
      maxWidth: {
        content: "var(--maxw)",
        dashboard: "1400px",
      },
      boxShadow: {
        xs: "var(--shadow-xs)",
        sm: "var(--shadow-sm)",
        md: "var(--shadow-md)",
        lg: "var(--shadow-lg)",
        xl: "var(--shadow-xl)",
      },
    },
  },
  plugins: [],
};
export default config;
