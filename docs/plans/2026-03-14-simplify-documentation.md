# Simplify and Consolidate Documentation

## Overview
Consolidate overlapping markdown documentation into two primary files: `README.md` for users and customers, and `Agent.md` for technical contributors and AI agents. Remove all redundant and historical documentation files.

## Context
- Files to modify: `README.md`, `Agent.md`
- Files to remove: `logic.md`, `CONTEXT.md`, `CHANGES.md`, `IMPLEMENTATION_PLAN.md`
- Related patterns: Maintain the high-quality technical structure of `Agent.md` while making `README.md` a comprehensive user guide.

## Development Approach
- **Testing approach**: Not applicable for documentation, but will verify no broken links or missing critical information.
- Complete each task fully before moving to the next.
- **CRITICAL: all documentation must be clear, concise, and logically organized.**

## Implementation Steps

### Task 1: Refactor Agent.md into a Technical/Contribution Guide

**Files:**
- Modify: `Agent.md`

- [ ] Add a "Development Guidelines" section (moved from current `README.md`).
- [ ] Add the logging standard (log/slog) to the guidelines.
- [ ] Ensure the file is clearly marked as the guide for contributors and AI agents.
- [ ] Verify that all technical architecture details from the current file are preserved.
- [ ] Verify content is correctly formatted and organized.

### Task 2: Refactor README.md into a User/Customer Guide

**Files:**
- Modify: `README.md`

- [ ] Retain the project title and high-level introduction.
- [ ] Incorporate the detailed "Duty Assignment Logic" from `logic.md`.
- [ ] Incorporate command formats (User and Admin) from `logic.md`.
- [ ] Add a section explaining the "User Status" (Active, Off-Duty, Inactive).
- [ ] Add a "Contribution" section that points technical users/agents to `Agent.md`.
- [ ] Ensure the tone is appropriate for end-users of the bot.

### Task 3: Remove Redundant and Historical Files

**Files:**
- Delete: `logic.md`
- Delete: `CONTEXT.md`
- Delete: `CHANGES.md`
- Delete: `IMPLEMENTATION_PLAN.md`

- [ ] Delete `logic.md` after verifying its content is fully absorbed into `README.md`.
- [ ] Delete `CONTEXT.md` (ephemeral development context).
- [ ] Delete `CHANGES.md` (historical migration summary).
- [ ] Delete `IMPLEMENTATION_PLAN.md` (completed plan).

### Task 4: Verify acceptance criteria

- [ ] Verify `README.md` contains all information a user or admin needs to operate the bot.
- [ ] Verify `Agent.md` contains all information an agent or developer needs to understand the architecture and contribute.
- [ ] Check for any broken links in the remaining documentation.
- [ ] Run a quick check to ensure no other redundant `.md` files exist in the root.

### Task 5: Update documentation

- [ ] Update `README.md` with user-facing changes.
- [ ] Update `Agent.md` with technical guidelines.
- [ ] Move this plan to `docs/plans/completed/` after implementation is finished.
