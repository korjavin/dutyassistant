# Refactor Frontend to Vanilla HTML5/JS and Simplify Deployment

## Overview
Simplify the frontend by removing Tailwind CSS and the Node.js-based build process. Transition to a vanilla HTML5/JS architecture with clear zone separation, direct file serving from the Go backend, and improved client-side caching. Update the Go server and Dockerfile to support this simplified structure.

## Context
- Files involved:
    - `web/index.html`: Update layout and link new CSS/JS.
    - `web/js/main.js`: Coordinate independent zone initialization.
    - `web/js/ui/calendar.js`: Refactor to focus on calendar, remove chores/queue logic.
    - `web/js/ui/queue.js`: (New) Manage Queue Summary zone.
    - `web/js/ui/chores.js`: (New) Manage Pending Chores zone.
    - `web/css/style.css`: (New) Vanilla CSS replacing Tailwind.
    - `internal/http/server.go`: Update static routes and add cache headers.
    - `Dockerfile`: Remove Node.js build stage and update COPY.
- Related patterns:
    - Native ES6 Modules.
    - Semantic HTML5 zones.
    - CSS Grid/Flexbox for layout.
    - Standard HTTP Caching (Cache-Control/ETags).

## Development Approach
- **Testing approach**: Manual UI verification and automated route testing.
- Complete each task fully before moving to the next.
- Use native browser capabilities (no build step).

## Implementation Steps

### Task 1: Remove Frontend Build System and Tailwind

**Files:**
- Delete: `web/package.json`
- Delete: `web/package-lock.json`
- Delete: `web/tailwind.config.js`
- Delete: `web/css/input.css`
- Delete: `web/dist/` (directory)

- [ ] Remove all Node.js/Tailwind configuration files
- [ ] Clean up build artifacts

### Task 2: Implement Modular JS Zones

**Files:**
- Create: `web/js/ui/queue.js`
- Create: `web/js/ui/chores.js`
- Modify: `web/js/ui/calendar.js`
- Modify: `web/js/main.js`

- [ ] Extract `displayQueueSummary` to `web/js/ui/queue.js`
- [ ] Extract `displayPendingChores` to `web/js/ui/chores.js`
- [ ] Update `main.js` to initialize all zones independently
- [ ] Optimize fetching: Ensure `getUsers()` and `getActiveChores()` are called once on load, not on every month change

### Task 3: Vanilla CSS and Semantic HTML

**Files:**
- Create: `web/css/style.css`
- Modify: `web/index.html`
- Modify: `web/js/ui/components.js`

- [ ] Create `web/css/style.css` with component-based classes (card, button, layout)
- [ ] Update `web/index.html` to use semantic tags (`<header>`, `<main>`, `<aside>`) and remove Tailwind classes
- [ ] Update `components.js` to use new CSS classes

### Task 4: Server-Side Cache and Static Serving

**Files:**
- Modify: `internal/http/server.go`

- [ ] Serve `/css` directory from `web/css`
- [ ] Add middleware to set `Cache-Control` headers for `/js`, `/css`, and `/vendor`
- [ ] Ensure `index.html` is served with `Cache-Control: no-cache` or `ETag`

### Task 5: Dockerfile Simplification

**Files:**
- Modify: `Dockerfile`

- [ ] Remove Node.js build stage
- [ ] Update `COPY` to include `web/css`, `web/js`, `web/vendor`
- [ ] Remove/Update `sed` BUILD_TIME replacement logic if redundant with proper caching

### Task 6: Verify acceptance criteria

- [ ] manual test: App loads and functions (calendar, volunteering)
- [ ] manual test: Verify "zones" (chores, queue, calendar) update correctly
- [ ] check network: Verify assets have Cache-Control headers
- [ ] run docker build: Ensure build works without Node.js

### Task 7: Update documentation

- [ ] update README.md/Agent.md to reflect new frontend dev workflow
- [ ] move this plan to `docs/plans/completed/`
