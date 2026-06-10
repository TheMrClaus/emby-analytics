# Product

## Register

product

## Users

Self-hosters and homelab admins running Emby, Plex, or Jellyfin media servers. They check the dashboard in short, purposeful sessions — usually on a desktop browser, often in a dim room next to other ops tools (Grafana, Portainer, Tautulli). Their jobs: see what's playing right now, understand watch patterns over time, spot transcoding/quality problems, and run occasional admin maintenance (syncs, cleanup). Many share read access with household members who just want "what have we been watching" stats.

## Product Purpose

Emby Analytics is a self-hosted analytics dashboard and API for media servers. It collects playback history, library metadata, and live "now playing" sessions into SQLite, and surfaces them as real-time and historical visualizations. Success looks like: a user opens the dashboard and within seconds knows what's playing, who's been watching, and whether anything (transcodes, storage, sync) needs attention — without digging through their media server's own admin UI.

## Brand Personality

Clean ops console. Three words: **precise, calm, trustworthy**. The interface should feel like a well-built monitoring tool — data-first, quiet chrome, sharp hierarchy — not a themed entertainment app. Emotion to evoke: confidence that the numbers are right and the system is healthy. Think "Grafana/Tautulli done well": restrained surfaces that let charts and live data carry the visual interest.

## Anti-references

- **Neon gamer aesthetic**: no RGB glows, cyberpunk purples, decorative gradients, or over-animated chrome. The existing gold-glow "premium" button treatment and radial-gradient body background lean this way and should not be extended.
- **Generic SaaS dashboard**: no identical stat-card grids, gradient hero metrics, or stock admin-template scaffolding. Cards only where they're the right affordance; vary density and structure by what the data needs.

## Design Principles

1. **Data is the decoration.** Charts, live sessions, and numbers provide the color and motion; the chrome around them stays quiet. When a surface feels bland, sharpen the data presentation before adding ornament.
2. **Answer first, explore second.** Every screen leads with the question it answers (what's playing? who watched most?) — the headline reading is legible at a glance, with drill-down beneath.
3. **Live means live.** Real-time surfaces (Now Playing) earn motion and prominence; historical surfaces stay still and scannable. Motion signals freshness, never decoration.
4. **Density with hierarchy.** Self-hosters want information-rich screens. Achieve density through tight, consistent rhythm and clear type hierarchy — not by shrinking text below readable sizes or cramming identical cards.
5. **Admin actions are boring on purpose.** Sync, cleanup, and destructive controls look plain and explicit, with clear states and confirmation — never styled as showpieces.

## Accessibility & Inclusion

- WCAG 2.1 AA: body text ≥4.5:1 contrast against its background (audit the white/50–white/70 muted text in the current dark theme), large text ≥3:1.
- Full keyboard navigation with visible focus states on all interactive controls.
- `prefers-reduced-motion` alternatives for all animation (the hero crossfade already does this; keep it the rule, not the exception).
- Charts must not rely on color alone — pair hue with labels, patterns, or direct values for color-blind users.
