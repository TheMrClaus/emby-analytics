# Agent Guidelines

This repository uses a tag-driven release workflow and a branch-first development model. Follow these rules when making changes as an AI coding agent.

## Workflow

- Work on feature/fix branches; do not work directly on `main`.
- Open a Pull Request (PR) to `main` once changes are ready and tested.
- Merging to `main` should NOT publish Docker images.
- Publishing images happens ONLY when a version tag (e.g., `v0.1.2`) is pushed.
- After a successful PR merge:
  - Delete the remote branch.
  - Delete the local branch (`git branch -d <branch>`), and optionally prune remote refs.

## CI / Release

- Docker publish workflow is configured to run on tags only.
- Do not change CI to publish on branch pushes unless explicitly requested.
- When a release is ready:
  1) Merge all required PRs to `main`.
  2) Create an annotated tag `vX.Y.Z` and push it.
  3) Draft GitHub Release notes for the tag.

## Commit & Push Etiquette

- Commit in small, logical steps with descriptive messages.
- When the user asks to “commit each step, but don’t push”, respect it: commit locally and wait.
- Ask for confirmation before pushing, merging, or tagging.
- Include issue references when applicable, e.g., `closes #24`.
- Before adding an issue reference, run `gh issue list` to check for a relevant open issue and then confirm with the user whether it’s okay to link/close it (e.g., `closes #123`). Only include the reference after explicit user approval.

## PR Preparation

- Summarize the problem, approach, and changes.
- Call out user-visible behavior changes.
- Note any CI/workflow or documentation updates included in the PR.

## Code Style & Linting

- Keep changes minimal and focused on the task.
- Maintain existing code style; fix lints in touched files.
- Prefer typed solutions over `any`.

## Safety

- Avoid destructive operations (resets, force pushes) unless explicitly requested.
- Never publish images or tags without explicit user approval.

## DATA GATHERING & SYNC

### Deletion Sync (IMPLEMENTED)

**Library Items**: Hard delete (permanent removal from `library_item` table)
- Automatically synced during `IngestLibraries()` in `go/internal/tasks/library_ingest.go`
- Items not found in current server fetch are deleted in batches (50 per batch)
- Historical data (`play_sessions`, `play_intervals`) is intentionally preserved

**Users**: Soft delete (marked with `deleted_at` timestamp)
- Automatically synced during `runUserSync()` in `go/internal/tasks/usersync.go`  
- Users not found on server are marked with `deleted_at = CURRENT_TIMESTAMP`
- Preserves historical analytics (watch time, top users)
- Users that reappear have `deleted_at` cleared automatically

### Key Design Decisions

**Asymmetric Deletion Strategy** (intentional):
- `library_item` = current catalog state → hard delete
- `emby_user` = historical identity → soft delete
- `play_sessions`/`play_intervals` = immutable history → never deleted

**Statistics Filtering**:
- Most stats endpoints filter `deleted_at IS NULL` for users
- Overview endpoint was fixed to match this pattern
- Item counts use normalized file paths for cross-server deduplication
- Pathless items counted by ID (no deduplication possible)

### Troubleshooting Data Counts

**User count seems wrong?**
- Check for soft-deleted users: `SELECT COUNT(*) FROM emby_user WHERE deleted_at IS NOT NULL`
- Overview endpoint now correctly filters deleted users

**Item count doesn't match Emby?**
- Items without `file_path` are counted by ID (no deduplication)
- Live TV/channels excluded from counts
- Cross-server duplicates detected by normalized file path

**Deletion sync not working?**
- Check logs for "deletion tracking" messages
- Sync failure now aborts instead of silently succeeding
- Next sync will retry deletion of still-missing items
