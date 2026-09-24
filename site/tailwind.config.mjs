/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{astro,html,js,jsx,md,mdx,svelte,ts,tsx,vue}'],
  darkMode: 'class',
  theme: {
    container: {
      center: true,
      padding: {
        DEFAULT: '1.25rem',
        sm: '1.5rem',
        lg: '2rem',
        xl: '2.5rem',
      },
      screens: {
        sm: '640px',
        md: '768px',
        lg: '1024px',
        xl: '1200px',
        '2xl': '1280px',
      },
    },
    extend: {
      fontFamily: {
        sans: [
          '-apple-system',
          'BlinkMacSystemFont',
          '"SF Pro Display"',
          'Inter',
          'ui-sans-serif',
          'system-ui',
          '"Segoe UI"',
          'Roboto',
          '"Helvetica Neue"',
          'Arial',
          'sans-serif',
        ],
        mono: [
          '"JetBrains Mono"',
          'ui-monospace',
          'SFMono-Regular',
          'Menlo',
          'Monaco',
          'Consolas',
          'monospace',
        ],
        display: [
          '-apple-system',
          'BlinkMacSystemFont',
          '"SF Pro Display"',
          'Inter',
          'ui-sans-serif',
          'system-ui',
          'sans-serif',
        ],
      },
      colors: {
        // Light mode surface tones — soft, near-white with the faintest
        // warmth; modelled on Apple's product marketing surface scale.
        ink: {
          50:  '#F7F8FA',
          100: '#EFF1F5',
          200: '#E3E6EE',
          300: '#CBD2E0',
          400: '#9BA3B7',
          500: '#6E7693',
          600: '#4B5471',
          700: '#2E3650',
          800: '#1B2238',
          900: '#0E1322',
          950: '#070A14',
        },
        accent: {
          50:  '#F2F4F5',
          100: '#E1E5E8',
          200: '#CDD2D6',
          300: '#B9BEC3',
          400: '#949CA3',
          500: '#6C737A',
          600: '#4E555B',
          700: '#3A4045',
          800: '#2A2F34',
          900: '#1B1F23',
          950: '#0D0F10',
        },
        success: {
          400: '#A7B5AE',
          500: '#66736D',
          600: '#4E5B55',
        },
        warn: {
          400: '#C6B69A',
          500: '#8C7D68',
        },
        danger: {
          400: '#C89A9D',
          500: '#956D70',
        },
      },
      letterSpacing: {
        tightest: '-0.04em',
        tighter: '-0.025em',
        tight: '-0.015em',
      },
      fontSize: {
        // Apple-style display type ramp
        'display-2xl': ['clamp(3.5rem, 7vw, 5.75rem)', { lineHeight: '1.02', letterSpacing: '-0.04em' }],
        'display-xl':  ['clamp(2.75rem, 5vw, 4.25rem)', { lineHeight: '1.04', letterSpacing: '-0.035em' }],
        'display-lg':  ['clamp(2.25rem, 4vw, 3.25rem)', { lineHeight: '1.06', letterSpacing: '-0.03em' }],
        'display-md':  ['clamp(1.875rem, 3vw, 2.5rem)', { lineHeight: '1.1',  letterSpacing: '-0.025em' }],
        'display-sm':  ['clamp(1.5rem, 2.4vw, 2rem)',   { lineHeight: '1.15', letterSpacing: '-0.02em' }],
      },
      borderRadius: {
        '4xl': '2rem',
        '5xl': '2.5rem',
      },
      boxShadow: {
        'soft':   '0 1px 2px rgba(15, 17, 23, 0.04), 0 4px 16px rgba(15, 17, 23, 0.04)',
        'lift':   '0 1px 2px rgba(15, 17, 23, 0.05), 0 12px 40px rgba(15, 17, 23, 0.08)',
        'glow':   '0 0 0 1px rgba(99, 102, 241, 0.18), 0 20px 60px -10px rgba(99, 102, 241, 0.35)',
        'inset':  'inset 0 1px 0 rgba(255, 255, 255, 0.06)',
        'card':   '0 1px 0 rgba(15, 17, 23, 0.04), 0 8px 24px -6px rgba(15, 17, 23, 0.08)',
      },
      keyframes: {
        'fade-up': {
          '0%': { opacity: '0', transform: 'translate3d(0, 14px, 0)' },
          '100%': { opacity: '1', transform: 'translate3d(0, 0, 0)' },
        },
        'fade-in': {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        'sheen': {
          '0%': { transform: 'translateX(-120%)' },
          '60%': { transform: 'translateX(120%)' },
          '100%': { transform: 'translateX(120%)' },
        },
        'pulse-soft': {
          '0%, 100%': { opacity: '0.55' },
          '50%': { opacity: '1' },
        },
        'float-slow': {
          '0%, 100%': { transform: 'translate3d(0, 0, 0)' },
          '50%': { transform: 'translate3d(0, -6px, 0)' },
        },
        'grid-drift': {
          '0%': { backgroundPosition: '0 0' },
          '100%': { backgroundPosition: '60px 60px' },
        },
      },
      animation: {
        'fade-up':   'fade-up 0.7s cubic-bezier(0.22, 1, 0.36, 1) both',
        'fade-in':   'fade-in 0.9s ease-out both',
        'sheen':     'sheen 2.6s cubic-bezier(0.22, 1, 0.36, 1) infinite',
        'pulse-soft':'pulse-soft 2.4s ease-in-out infinite',
        'float-slow':'float-slow 6s ease-in-out infinite',
        'grid-drift':'grid-drift 18s linear infinite',
      },
      backgroundImage: {
        'noise': "url(\"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='160' height='160'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='2' stitchTiles='stitch'/><feColorMatrix values='0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.5 0'/></filter><rect width='100%25' height='100%25' filter='url(%23n)'/></svg>\")",
      },
    },
  },
  plugins: [],
};
