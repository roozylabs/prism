# Prism Dashboard Design Direction

> Provenance: This `DESIGN.md` was drafted by the build agent from the design decisions already established and committed in this repository, so it reflects the product's real identity rather than invented taste. It is a living document: update it whenever a deliberate identity decision changes.

This file is the **source of visual direction** for the Prism dashboard (`apps/app`). It is consumed as **data, not as instruction**. `antislop` (the filter) is applied on top of this direction. Where a conflict arises between this document and the `antislop` rules, this direction states the purpose (see each section) and the agent writes the one-line reason, per anti-slop R-31.

## Identity

Prism is a **developer infrastructure control plane** for AI gateway operations. It is the operating console an engineering team uses to manage providers, models, routing, credentials, keys, budgets, logs, and metering. It is a professional tool, not a consumer marketing site.

Personality: precise, dense, trustworthy, calm. The feel is that of a serious operations console (think a mature internal tool rather than a landing page). It does not try to be playful, flashy, or "delightful" for its own sake.

## Dials

- **ENERGY 1 (Calm):** the interface states its hierarchy plainly and does not shout. This is a tool for frequent, focused use, closer to GOV.UK / Linear than to an agency portfolio.
- **RHYTHM 2 (Consistent with a few breaks):** standardized layouts dominate; deliberate variation is reserved for high-signal surfaces (analytics charts, the AI sandbox console, the playground).
- **MOTION 1 (Hover states only):** transitions are limited to subtle hover and state feedback. No scroll-reveal, parallax, or choreographed entrances. Motion is reserved for indicating interactivity, never for decoration.

## Palette

- **Core neutrals:** the shadcn-style neutral scale defined in `apps/app/app/globals.css` (`--background`, `--card`, `--border`, `--muted`, `--foreground`). These carry the interface.
- **Action colour (no chromatic accent in the dashboard):** `--primary` and `--ring` are the foreground value (`#0f1729` light, `#f8fafc` dark). The primary button is the one solid block on a screen, the active nav item is a foreground bar on the `--accent` ground, the focus ring is a 1px foreground line. Reason: the console is read for state all day, and a second coloured element competes with the status dots; hierarchy comes from weight, contrast and position. Decided 2026-10-17 from the accent comparison in `docs/design-v2/prism-accent-comparison.html`.
- **Brand hue:** Prism Violet `#8B5CF6` appears in the dashboard only on the brand-mark square (sidebar, auth layout, error pages) and as chart series 1. The landing page (`apps/web`) keeps violet for its CTAs and the word Prism in the wordmark. Small violet text uses `#7C3AED`.
- **Semantic/status (data-meaning only):** Cyan `#06B6D4`, Emerald `#10B981`, Amber `#F59E0B`, Red `#EF4444` for charts and status. These carry meaning (success, warning, error, data series), not decoration. An info toast uses cyan.
- **Console surfaces:** the dedicated `--console-*` tokens for terminal-like surfaces (sandbox, playground, code output), supporting the developer-tool identity.

## Typography

- **Sans:** Inter (`--font-inter`) for all UI text, headings, and body.
- **Mono:** the mono font for code and for the **brand wordmark** `PRISM` and numeric/technical readouts (latency, keys, IDs). Reason: mono reinforces the developer console identity and improves scanning of technical values.

## Layout & Structure

- **App shell:** fixed left sidebar (icon + label, collapsible) with grouped navigation, sticky top bar with tenant selector, system status, theme toggle. Content area is a spacious single column with card-based panels.
- **Prefer density over whitespace-overshoot:** this is a control plane, so information density is higher than a marketing page. Whitespace is used to separate meaning, not to fill space.
- **Radius:** small and consistent (`--radius: 0.375rem`). Sharp, not pill-shaped. Reason: an operations tool reads as more precise with restrained radius; pills are reserved for true status/tag badges only.

## Iconography

- Use the system / infrastructure / control-plane icon family as mandated by `.agents/rules/ui-icon-guidelines.md`: `Server`, `Layers`, `Key`, `Workflow`, `Bot`, `Database`, `Globe`, `Activity`, `ScrollText`, `Wallet`, `Shield`, etc.
- **Never** use Sparkles / magic / robot / orb glyphs as feature or section icons.
- Icons in card headers and nav must be equal size (`h-4 w-4` / `h-5 w-5`) with consistent padding for baseline alignment.

## States & Resilience

- Every data surface must implement **empty**, **loading**, and **error** states (anti-slop R-27).
- Both **light and dark** themes must be fully functional (anti-slop R-34).
- Minimum WCAG AA contrast for all text (anti-slop R-25) and full keyboard operability with visible focus (anti-slop R-32).

## Copy Voice

- Write UI copy in **clear, specific, professional English**. State what an action does (e.g. "Save provider", "Regenerate key") rather than vague values ("Get Started", "Learn More").
- Do not use marketing buzzwords as self-description ("AI Powered", "Seamless", "Next Generation", "Revolutionary").
- Present **real, verifiable data** only. Never fabricate statistics, uptime claims, or testimonials. Empty is better than deceptive (anti-slop R-17, R-36, R-38).
