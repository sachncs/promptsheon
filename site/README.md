# Promptsheon — Marketing Site

The premium product landing page for Promptsheon, deployed to GitHub Pages.

## Stack

- **[Astro 5](https://astro.build)** — Static-site generator. Ships zero JS by
  default and renders the entire page in well under 300 ms. We use it as a
  HTML-first templating engine with the option of adding scoped interactivity
  where needed.
- **Tailwind CSS 3** — Utility-first CSS. All design tokens (colour ramp,
  typography scale, shadows, animation curves) live in
  [`tailwind.config.mjs`](./tailwind.config.mjs).
- **TypeScript** — Strict mode. `astro check` is wired into CI.

The output is a single static folder that GitHub Pages can serve directly.

## Folder layout

```
site/
├── astro.config.mjs        Astro configuration (base path, integrations, vite)
├── tailwind.config.mjs     Design tokens and Tailwind theme
├── tsconfig.json           TypeScript config (strict)
├── public/                 Files served verbatim at the site root
│   ├── favicon.svg
│   ├── logo.svg
│   ├── og.svg / og.png     Open Graph share image
│   └── robots.txt
└── src/
    ├── components/         One Astro component per section of the page
    │   ├── SiteHead.astro
    │   ├── SiteNav.astro
    │   ├── SiteFooter.astro
    │   ├── Hero.astro
    │   ├── LogoCloud.astro
    │   ├── Metrics.astro
    │   ├── Pillars.astro
    │   ├── Features.astro
    │   ├── Workflow.astro
    │   ├── Trust.astro
    │   ├── Comparison.astro
    │   ├── Pricing.astro
    │   ├── CTA.astro
    │   ├── Logo.astro
    │   └── LogoMark.astro
    ├── layouts/
    │   └── BaseLayout.astro   Shared <head>, theme bootstrap, scroll-spy
    ├── pages/
    │   ├── index.astro        The single product landing page
    │   └── 404.astro
    └── styles/
        └── global.css         Design system (CSS variables, primitives)
```

## Design system

All colours, type ramps, and motion curves live in
[`src/styles/global.css`](./src/styles/global.css) as CSS custom properties,
exposed to Tailwind via [`tailwind.config.mjs`](./tailwind.config.mjs).

The system is intentionally restrained:

- **One accent** — indigo `#4F46E5` (light) / `#818CF8` (dark).
- **Two surface scales** — `--bg-*` for light, `--ink-*` for text, with a
  matching dark set that mirrors the light scale.
- **Five type sizes** — `display-2xl … display-sm` for headlines,
  single-family Inter for body, JetBrains Mono for code.
- **Five shadow scales** — from `inset` (1px hairline) to `lift`
  (deep ambient) to `glow` (used sparingly on the recommended tier).

## Interactions

All interactivity is implemented in plain TypeScript inside Astro's
`<script>` tags. There is no client-side framework:

- **Theme toggle** — `localStorage`-backed dark/light switch with a
  no-flash bootstrap script in `<head>`.
- **Smooth-scroll anchors** — header-offset aware.
- **Intersection observer** — adds `is-visible` to `.ps-reveal` elements as
  they enter the viewport.
- **Mobile menu** — disclosure that closes on link tap.

## Build

```bash
# Install
pnpm install

# Dev server (auto-reload)
pnpm --filter @promptsheon/site dev

# Production build
pnpm --filter @promptsheon/site build
#   → site/dist/

# Preview production output
pnpm --filter @promptsheon/site preview
```

## Deployment

GitHub Actions (`.github/workflows/pages.yml`) builds the site on every push
to `master` and deploys `site/dist/` via `actions/deploy-pages`. No
additional configuration required.