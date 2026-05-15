# Monospace Web Redesign for Roster Bot frontend

## Overview

Redesign `web/index.html` and its companion CSS/JS to match the Monospace Web design bundle the user fetched from `api.anthropic.com/v1/design/h/zqn8EusNnB1kl0wM2essUw`. The design is a Telegram-WebApp-width (max 480px) page with:

- Header (mini glyph + `Roster·Bot v2.3` + `@user` status dot)
- Status strip (3 tabular counters: open chores, queue days, this week)
- Pending chores list
- Queue balance bars (▮ volunteered, ▯ admin outlined, dashed padding)
- Monday-first month calendar with duty pills, today outline, queue badges
- Day modal showing duty cards with Volunteer/Withdraw actions
- Legend + footer with blinking cursor & live clock

The redesign is purely visual/structural — backend APIs are untouched, all real data shapes (`/api/v1/users`, `/api/v1/chores/active`, `/api/v1/schedule/{y}/{m}`, `/api/v1/prognosis/{y}/{m}`) keep working.

## Context (from discovery)

- **Existing frontend**: vanilla JS modules under `web/js/`, custom CSS replacing Tailwind, Vanilla Calendar Pro library for the calendar.
- **Design bundle**: React + Babel prototype (`.design-tmp/dutyassistant/project/{index.html,app.jsx,app.css,tokens.css,data.jsx,tweaks-panel.jsx}`). README says recreate the *visual output*, not the internal structure.
- **Server**: `internal/http/server.go` serves `web/` statically under `/css`, `/js`, `/vendor`; `/` and `/index.html` go to `web/index.html`.

## Development Approach

- **Testing approach**: Regular (visual redesign — verify by running the bot binary and viewing the page; no new unit tests required for CSS, light smoke for JS render functions if practical).
- Drop Vanilla Calendar Pro entirely — the design's calendar grid is bespoke and the library's DOM/CSS fights it. Build a custom Monday-first grid in `web/js/ui/calendar.js`.
- Drop the Tweaks panel — that's the design-time iteration tool, not a real product surface. Light theme by default, dark tokens preserved for `prefers-color-scheme: dark`.
- Keep current module file layout (`api.js`, `store.js`, `ui/{calendar,chores,queue,components}.js`).
- Vendor JetBrains Mono woff2 files at `web/fonts/`.

## Implementation Steps

### Task 1: Design tokens + fonts
- [ ] copy `JetBrainsMono-400.woff2` and `JetBrainsMono-400italic.woff2` from bundle to `web/fonts/`
- [ ] port `tokens.css` to `web/css/tokens.css` (light + dark, size scale, monospace family, square borders, offset shadows)
- [ ] update `internal/http/server.go` to serve `/fonts` static dir

### Task 2: New `web/css/style.css`
- [ ] replace existing Tailwind-replacement styles with adapted Monospace Web stylesheet
- [ ] keep namespacing inside `.shell`, `.section`, `.cal-*`, `.chore`, `.queue-row`, `.modal`, `.btn`, etc.
- [ ] include the dotted blueprint page background, 4px offset card shadow, hard 1px rules

### Task 3: Rewrite `web/index.html` markup
- [ ] swap to the new shell skeleton: `.page > .shell > header / status / sections / footer`
- [ ] drop Vanilla Calendar Pro CSS link + inline overrides
- [ ] add `<link rel="stylesheet" href="/css/tokens.css">` before `style.css`
- [ ] keep Telegram WebApp script tag and `/js/main.js` entry

### Task 4: Bespoke calendar in `web/js/ui/calendar.js`
- [ ] remove import of Vanilla Calendar Pro
- [ ] add `buildMonthCells(year, monthIdx)` (Mon-first, 42 cells) and DOW header
- [ ] render `.cal-day` cells with day number, queue badge, duty pills (max 2, `+N` overflow)
- [ ] handle prev/next/today navigation, today outline, weekend tint, prognosis dashed pill
- [ ] click handler opens duty modal via `ui/components.js`

### Task 5: Reskin chores + queue panels
- [ ] update `displayPendingChores` to emit `.chore > .mark + .body{.desc,.meta} + .age` rows with "Xh ago" / "Xd Xh" age strings
- [ ] update `displayQueueSummary` to emit sorted `.queue-row` rows with initials avatar, ▮/▯/dashed bar viz, V·n / A·n tally
- [ ] both helpers derive `[N]` count for section header and "no items" italic empty state

### Task 6: Duty modal
- [ ] rewrite the modal builder in `ui/components.js` to match `.scrim > .modal > .modal-hdr + .modal-body` structure
- [ ] each duty rendered as `.duty.vol|.admin|.prog` card with name, kind label, queue chip
- [ ] modal actions: Volunteer (primary, if user not on duty), Withdraw (if user on duty), Close (ghost)
- [ ] Escape key closes modal; backdrop click closes modal

### Task 7: Status strip + footer
- [ ] add `renderStatusStrip(choresCount, queueTotal, dutiesThisWeek)` in main.js or a tiny new `ui/header.js`
- [ ] derive `dutiesThisWeek` from the current month's schedule payload
- [ ] add footer with `connected · telegram` blink cursor and live HH:MM clock (updates once on load — no setInterval churn)

### Task 8: Server route for fonts
- [ ] add `staticRoutes.Static("/fonts", "./web/fonts")` next to existing static mounts

### Task 9: Verify acceptance
- [ ] `go vet ./...` clean
- [ ] `go build -o /tmp/roster-bot ./cmd/roster-bot/` succeeds
- [ ] start server locally, load `http://localhost:PORT/`, confirm:
  - header + status strip render with monospace JetBrains font
  - chores section renders mocked or real chores in the new style
  - queue rows show bar viz proportional to V+A counts
  - calendar renders Mon-first grid with pills, today is outlined, prev/next works, day click opens modal
  - footer shows blinking cursor and a clock
- [ ] no console errors; no remaining Tailwind class strings

### Task 10: Cleanup
- [ ] remove `web/css/styles.css` (legacy duplicate) if unused
- [ ] remove `web/vendor/vanilla-calendar/` directory and `web/node_modules/vanilla-calendar-pro` if calendar lib is no longer imported
- [ ] remove `web/dist/output.css` if unused

## Technical Details

- **Color tokens** (light): `--bg-0: #f4f1ea` paper, `--fg-0: #16140f` warm near-black, `--line-1` hard rules, `--line-2/3` soft dividers.
- **Shadow**: offset 4px 4px 0 of `--line-1` on `.shell` and modal; 2px offset on buttons.
- **Calendar dimensions**: 7-column grid, 1px gap on `--line-2`, min-height 56px (44px in compact variant).
- **Backend payload mapping**:
  - User: `IsActive`, `FirstName`, `VolunteerQueueDays`, `AdminQueueDays` → bar viz and tally.
  - Duty: `date`, `user_name`, `assignment_type` ("voluntary"|"admin"), `volunteer_queue_days`, `admin_queue_days`.
  - Prognosis: `date`, `user_name` → dashed pill, kind=`prognosis`.
- **Modal action wiring**: `duty.id` carried through; Volunteer/Withdraw posts via existing `api.js` and reloads the month.

## Post-Completion

**Manual verification**:
- Open `https://t.me/<bot>?startapp=schedule` (or equivalent Telegram WebApp launch) and confirm the layout fits a 360-414px viewport.
- Toggle macOS dark mode and confirm `prefers-color-scheme: dark` swaps tokens cleanly.

**Out of scope**:
- Tweaks panel UI (design-time only).
- Dark-mode user toggle (relies on OS preference for now).
- Rating system / sviniya screens (not in the design bundle).
