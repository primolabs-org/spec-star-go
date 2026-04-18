---
applyTo: "**"
---

# Cleanup Instructions

Follow these rules when modifying code in this repository.

## Touched-area hygiene

- Leave touched areas in a cleaner state than you found them.
- Remove dead code exposed by the change.
- Remove stale comments that no longer describe real behavior.
- Remove misleading comments, obsolete notes, and commented-out code.
- Remove unused imports, members, helpers, variables, and properties introduced or exposed by the change.
- Remove temporary implementation leftovers once they are no longer needed.

## Scope discipline

- Restrict cleanup to files and areas directly touched by the active task.
- Do not perform broad cleanup outside the active scope unless the task explicitly requires it.
- Do not turn a narrow implementation into a repository-wide cleanup pass.
- Do not rewrite unrelated code for style only.

## Refactor hygiene

- When a refactor changes behavior or structure, update surrounding names, comments, and references so they stay accurate.
- Do not leave old names that no longer match the responsibility of the code.
- Do not leave compatibility shims, transitional branches, or duplicate paths unless the task explicitly requires them.
- Remove superseded logic when the new logic fully replaces it within the approved scope.

## Comment discipline

- Keep comments only when they add real value that is not obvious from the code.
- Prefer updating incorrect comments over preserving them.
- Do not add obvious comments that simply restate the code.
- Do not leave TODO or placeholder comments unless the active task explicitly allows them.

## File and symbol hygiene

- Keep files focused on their actual responsibility.
- Do not leave unused types, functions, methods, constants, or configuration entries behind.
- Do not create new helpers or abstractions without removing the duplication or clutter they are meant to replace.
- When code is moved or consolidated, clean up the old location within the approved scope.

## What to avoid

- Leaving dead code behind “just in case”.
- Preserving stale comments after refactors.
- Keeping unused helpers because they might be useful later.
- Mixing unrelated cleanup with active feature work.
- Expanding a small task into a broad style rewrite.

## Examples of acceptable behavior

- Removing an obsolete private method after inlining or consolidation.
- Updating or removing comments that no longer match the implementation.
- Deleting unused imports, members, helpers, variables, and properties exposed by a change.
- Removing a dead branch that became unreachable because of the current task.

## Examples of unacceptable behavior

- Reformatting unrelated modules during a narrow feature change.
- Renaming unrelated symbols outside the touched scope.
- Keeping commented-out legacy code after the new path is complete.
- Leaving stale TODOs or outdated comments in modified files.