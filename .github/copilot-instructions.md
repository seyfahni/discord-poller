# Instructions

- When in doubt or when requirements are unclear, ask the user.

## Task Planning and Preparation

When starting work on a bigger task, a solution design should be created first.
It must be written to `docs/solution-designs`. Its name should start with the current date and a running two-digit number ordering it below all previously existing, e.g. `2026-10-11_01_<title>.md`
Do not guess requirements, ask the user.
When writing a solution design, do not proceed to implementation whithout explicit permission from the user.

## `git`

### Mutating `git` Commands

Do not use any mutating git commands ever.
That includes but is not limited to:
- `git commit`
- `git add`
- `git reset`
- `git restore`
- `git revert`

It is the user's responsibility to use these. Instead of using them yourself, you can tell the user that changes are ready to be committed; or that there is something that needs to be done and provide guidance on how to do it.
