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

