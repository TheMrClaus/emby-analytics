---
name: Emby Analytics
description: Self-hosted media-server analytics dashboard — a clean ops console for Emby, Plex, and Jellyfin
colors:
  void: "#0b0e13"
  panel: "#121722"
  panel-raised: "#262626"
  panel-hover: "#404040"
  chrome-deep: "#171717"
  gold-bright: "#ffd54a"
  gold: "#ffc838"
  gold-deep: "#f5b515"
  signal-green: "#34d399"
  signal-red: "#ef4444"
  ink-bright: "#ffffff"
  ink-muted: "#9aa4b2"
  ink-faint: "rgba(255,255,255,0.5)"
  hairline: "rgba(255,255,255,0.1)"
typography:
  display:
    fontFamily: "ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif"
    fontSize: "1.875rem"
    fontWeight: 800
    lineHeight: 1.25
    letterSpacing: "-0.025em"
  title:
    fontFamily: "ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif"
    fontSize: "1.5rem"
    fontWeight: 600
    lineHeight: 1.33
    letterSpacing: "-0.025em"
  heading:
    fontFamily: "ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 600
    lineHeight: 1.55
    letterSpacing: "-0.025em"
  body:
    fontFamily: "ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.43
  label:
    fontFamily: "ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 600
    lineHeight: 1.33
    letterSpacing: "0.05em"
  mono:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.43
rounded:
  sm: "4px"
  md: "8px"
  lg: "12px"
  xl: "16px"
  full: "9999px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "16px"
  lg: "24px"
  xl: "32px"
components:
  button-primary:
    backgroundColor: "{colors.chrome-deep}"
    textColor: "{colors.gold}"
    rounded: "{rounded.lg}"
    padding: "6px 12px"
  button-danger:
    backgroundColor: "#1a0e0e"
    textColor: "#ffdede"
    rounded: "{rounded.lg}"
    padding: "6px 12px"
  button-ghost:
    backgroundColor: "{colors.panel-raised}"
    textColor: "{colors.ink-bright}"
    rounded: "{rounded.md}"
    padding: "8px 12px"
  button-ghost-hover:
    backgroundColor: "{colors.panel-hover}"
    textColor: "{colors.ink-bright}"
  card:
    backgroundColor: "{colors.panel}"
    textColor: "{colors.ink-bright}"
    rounded: "{rounded.xl}"
    padding: "16px 24px"
  badge:
    backgroundColor: "transparent"
    textColor: "{colors.ink-muted}"
    rounded: "{rounded.full}"
    padding: "2px 8px"
---

# Design System: Emby Analytics

## 1. Overview

**Creative North Star: "The Server Room"**

Emby Analytics looks like the room where the machines live: dark, calm, and instrument-grade. Surfaces are deep near-black panels stacked in subtle tonal steps; the only light in the room comes from the data itself — gold chart bars, green status lights, live session art. The chrome never competes. If a screen feels bland, the answer is sharper data presentation (better chart, tighter table, clearer headline number), never more ornament on the frame.

The system explicitly rejects two failure modes named in PRODUCT.md. First, the **neon gamer aesthetic**: no RGB glows, decorative radial gradients, bloom animations, or cyberpunk styling. The existing gold-glow "premium" button treatment and the emerald radial body gradient lean this way and are scheduled for retirement, not extension. Second, the **generic SaaS dashboard**: no identical stat-card grids, no gradient hero metrics, no admin-template scaffolding. Layout density and structure follow what each dataset needs.

**Key Characteristics:**
- Deep, flat, near-black tonal stack (`#0b0e13` → `#121722` → `#262626`) with 1px hairline borders
- One accent: gold (`#ffc838`) — used sparingly, where attention is earned
- Green and red are status semantics (live / healthy / error), never decoration
- System font stack, tight tracking on headings, data in tables and Nivo charts
- Motion only on live surfaces (Now Playing); historical views are still

## 2. Colors

A dark instrument palette: cold near-black neutrals lit by a single warm gold accent, with green and red reserved as status signals.

### Primary
- **Projector Gold** (#ffc838): The single accent of the system. Carries chart data (bars, lines), key highlighted values, the active state of primary actions, and brand moments in the header. Flanked by **Bright Gold** (#ffd54a) for hover/emphasis and **Deep Gold** (#f5b515) for borders and pressed states.

### Secondary
- **Signal Green** (#34d399): Status only — a session is playing, a server is healthy, a sync succeeded. Never used as a decorative accent or background wash.
- **Signal Red** (#ef4444): Status only — errors, failed transcodes, destructive actions. Danger buttons use its dark-tinted family (#1a0e0e background, #ffdede text).

### Neutral
- **Void** (#0b0e13): The page background. Flat; no decorative gradients on top of it.
- **Panel** (#121722) and **Raised Panel** (#262626): Card and control surfaces, stepped one tone up per layer. **Panel Hover** (#404040) is the interactive hover tone.
- **Bright Ink** (#ffffff): Primary text and data values.
- **Muted Ink** (#9aa4b2): Secondary text, table headers, captions. This is the floor for body-size text; do not go fainter for anything that must be read.
- **Faint Ink** (rgba(255,255,255,0.5)): Tertiary captions and timestamps only, never sentences.
- **Hairline** (rgba(255,255,255,0.1)): The universal 1px border for panels, table rows, and dividers.

### Named Rules
**The Single Lamp Rule.** Gold is the only accent in the chrome, and it covers no more than ~10% of any screen. If two things glow gold, neither is important.

**The Status Light Rule.** Green means live or healthy; red means failed or destructive. Neither color ever appears for decoration, branding, or emphasis. (The legacy emerald hero gradient violates this and should be migrated to the neutral panel stack.)

## 3. Typography

**Display Font:** System UI sans (ui-sans-serif → Segoe UI → Roboto)
**Body Font:** Same system stack
**Label/Mono Font:** System monospace (ui-monospace → Menlo) for IDs, codecs, and technical values

**Character:** Utilitarian and quiet — the OS's own voice, which suits a self-hosted tool that lives beside Grafana and Portainer. Hierarchy comes entirely from weight and size contrast, with tight tracking (-0.025em) on headings.

### Hierarchy
- **Display** (800, 30px/1.25, -0.025em): Page-level headline, one per page. Scales to 36px on `sm:` and up.
- **Title** (600, 24px/1.33, -0.025em): Section titles.
- **Heading** (600, 18px/1.55, -0.025em): Card titles.
- **Body** (400, 14px/1.43): Tables, descriptions, controls. The dashboard's working size.
- **Label** (600, 12px, +0.05em, uppercase): Table column headers only.
- **Mono** (400, 14px): Codec names, bitrates, IDs, file paths — anything a user might copy.

### Named Rules
**The Two-Second Rule.** Every card's headline reading (the number, the now-playing title, the trend) must be legible at a glance in Display/Title weight; supporting detail drops to Body below it.

## 4. Elevation

Depth is tonal, not shadowed. Layers step up the neutral stack — Void page (#0b0e13), Panel cards (#121722–#262626), Hover tone (#404040) — and every surface is outlined with the 1px Hairline border (rgba(255,255,255,0.1)). Box shadows are reserved for true overlays: dropdown menus, modals, and the mobile drawer (which uses a strong `shadow-2xl` because it genuinely floats above the page). The large soft shadows currently on resting cards are legacy and should not be replicated on new surfaces.

### Shadow Vocabulary
- **Overlay** (`box-shadow: 0 25px 50px -12px rgba(0,0,0,0.6)`): Modals, drawers, dropdown menus — elements that float above the page plane.

### Named Rules
**The Tonal Stack Rule.** A resting surface gets a background step and a hairline border, never a shadow. If you reach for a shadow, the element must be an overlay that can be dismissed.

## 5. Components

### Buttons
- **Shape:** Softly rounded (12px radius), 14px medium-weight text, compact padding (6px 12px).
- **Primary:** Near-black surface (#171717 family) with Projector Gold text (#ffc838) and a 1px deep-gold border (rgba(245,181,21,0.35)). Calm at rest — no glow, no bloom animation.
- **Hover / Focus:** Slight brightness lift on hover; keyboard focus gets a visible two-ring outline (dark inner ring + gold outer ring at 45% opacity). Active state nudges down 1px.
- **Danger:** Same construction in the red family — #1a0e0e surface, #ffdede text, red border (rgba(255,70,70,0.55)). Per PRODUCT.md, admin/destructive actions look plain and explicit, never showpieces.
- **Ghost (utility):** Raised Panel background (#262626) stepping to Panel Hover (#404040), white text, 8px radius. The workhorse for filters, selectors, and secondary controls.

### Badges / Chips
- **Style:** Pill (full radius), transparent background, 1px Hairline border, 12px text in pale tint.
- **Use:** Counts, codec tags, server-type indicators. Status badges may tint border/text with Signal Green or Signal Red per the Status Light Rule.

### Cards / Containers
- **Corner Style:** 16px radius.
- **Background:** Panel (#121722) or Raised Panel (#262626) on the Void page.
- **Shadow Strategy:** None at rest (Tonal Stack Rule); 1px Hairline border instead.
- **Internal Padding:** 16px, stepping to 24px at `md:`.
- **Header pattern:** Muted Ink title (14px) top-left, optional control top-right, content below.

### Tables
- **Style:** The dashboard's signature component (`.table-dark`). 14px body text, uppercase 12px Muted Ink column labels, rows divided by Hairline borders with 12px vertical padding.
- **Interaction:** Row hover tints background white/5. Numeric columns right-aligned (`.num`).

### Inputs / Fields
- **Style:** Raised Panel background, 1px Hairline border, 8px radius, white text at 14px.
- **Focus:** Border brightens and a visible focus ring appears; never remove the outline without a replacement.

### Navigation
- **Style:** Top header bar on Chrome Deep (#171717) with a Hairline bottom border. Wordmark in bold white, hover-tinting toward gold. Desktop nav is inline; mobile collapses to a right-side drawer (320px, Overlay shadow, 44px minimum touch targets).

### Now Playing Hero (signature)
- The one surface allowed cinematic treatment: blurred backdrop art crossfading between sessions, dark legibility overlay, foreground session details. Motion here signals liveness (PRODUCT.md: "live means live") and already honors `prefers-reduced-motion`. Its emerald tint and gold title treatment should migrate toward the neutral-panel + gold-accent system.

## 6. Do's and Don'ts

### Do:
- **Do** keep chrome neutral and let data carry the color — gold charts, status lights, session artwork (the Single Lamp Rule).
- **Do** build depth with background steps (#0b0e13 → #121722 → #262626) and 1px hairline borders; reserve shadows for overlays.
- **Do** keep body text at 14px/white or 14px/#9aa4b2 minimum contrast; tertiary white/50 text is for short captions only.
- **Do** give every interactive element a visible keyboard focus ring and ≥44px touch targets on mobile.
- **Do** provide a `prefers-reduced-motion` alternative for every animation, as the hero crossfade already does.
- **Do** right-align numeric table columns and use the mono stack for codecs, bitrates, and IDs.

### Don't:
- **Don't** ship the "neon gamer aesthetic" PRODUCT.md forbids: no glow blooms, pulse keyframes, radial-gradient washes, or RGB-tinted borders. The legacy `btn-premium` bloom animation and the emerald body gradient are the named offenders — retire, don't imitate.
- **Don't** ship the "generic SaaS dashboard" PRODUCT.md forbids: no grids of identical stat cards, no gradient hero metrics, no decorative icon+heading+text card rows.
- **Don't** use Signal Green or Signal Red decoratively; they are status semantics only.
- **Don't** introduce a second accent hue. Gold (#ffc838) is the lamp; everything else is neutral or status.
- **Don't** put shadows on resting cards or nest cards inside cards.
- **Don't** drop text below 12px or below Muted Ink (#9aa4b2) contrast for anything that must be read.
- **Don't** animate historical/analytics surfaces; motion belongs to live data only.
