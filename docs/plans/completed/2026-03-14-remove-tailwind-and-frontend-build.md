# Remove Tailwind CSS and Frontend Build Stage

## Overview
This plan removes Tailwind CSS from the web interface, replacing it with a well-structured vanilla CSS setup. It also eliminates the frontend build step by serving CSS and JS directly from the source directories.

## Context
- Files involved:
    - `web/index.html`: Update links and classes.
    - `web/js/ui/components.js`: Replace Tailwind classes with custom ones.
    - `web/js/ui/calendar.js`: Replace Tailwind classes with custom ones.
    - `web/css/styles.css`: (New) Replicate Tailwind styles in vanilla CSS.
    - `internal/http/server.go`: Serve `/css` instead of `/dist`.
    - `Dockerfile`: Update static asset paths and `BUILD_TIME` replacement.
    - `web/package.json`, `web/package-lock.json`, `web/tailwind.config.js`, `web/css/input.css`, `web/dist/`: Delete these.
- Related patterns:
    - Using semantic CSS classes for UI components.
    - Using CSS variables for theme and colors.
    - Serving static assets directly from source.

## Development Approach
- **Testing approach**: Regular (code first, then tests).
- Complete each task fully before moving to the next.
- **CRITICAL: every task MUST include new/updated tests if applicable.**
- **CRITICAL: all tests must pass before starting next task.**

## Implementation Steps

### Task 1: Create Vanilla CSS Setup

**Files:**
- Create: `web/css/styles.css`

- [ ] Define CSS variables for colors, spacing, and typography (mimicking Tailwind defaults used in the project)
- [ ] Create base styles (reset, typography)
- [ ] Implement layout classes (container, flex, etc.)
- [ ] Implement component classes (card, button, badge, modal, alert, spinner)
- [ ] Implement utility classes used in the project (text-sm, font-bold, etc.)
- [ ] run project test suite - must pass before task 2

### Task 2: Update HTML and JS to use custom CSS classes

**Files:**
- Modify: `web/index.html`
- Modify: `web/js/ui/components.js`
- Modify: `web/js/ui/calendar.js`

- [ ] Update `web/index.html` to link to `/css/styles.css` instead of `/dist/output.css`
- [ ] Replace Tailwind utility classes with new custom classes in `web/index.html`
- [ ] Replace Tailwind utility classes with new custom classes in `web/js/ui/components.js`
- [ ] Replace Tailwind utility classes with new custom classes in `web/js/ui/calendar.js`
- [ ] run project test suite - must pass before task 3

### Task 3: Update Server and Build Configuration

**Files:**
- Modify: `internal/http/server.go`
- Modify: `Dockerfile`

- [ ] Update `internal/http/server.go` to serve the `web/css/` directory as `/css`
- [ ] Remove `router.Static("/dist", "./web/dist")` from `internal/http/server.go`
- [ ] Update `Dockerfile` to use the new CSS path and ensure `BUILD_TIME` is replaced in the new CSS link
- [ ] run project test suite - must pass before task 4

### Task 4: Clean up Tailwind and Build Files

**Files:**
- Delete: `web/package.json`
- Delete: `web/package-lock.json`
- Delete: `web/tailwind.config.js`
- Delete: `web/css/input.css`

- [ ] Remove all Tailwind-specific configuration and build artifacts
- [ ] Verify that the frontend works without any node-based build step
- [ ] run project test suite - must pass before task 5

### Task 5: Verify acceptance criteria

- [ ] manual test: Check the web interface in a browser to ensure it looks and functions as before
- [ ] run project test suite (e.g., `go test ./...`)
- [ ] verify no build errors in `Dockerfile`
- [ ] verify static assets are served correctly by the Go server

### Task 6: Update documentation

- [ ] update README.md if user-facing changes
- [ ] move this plan to `docs/plans/completed/`
