---
name: specstar-design-feature
description: Create, refine, or continue a SpecStar feature through requirements design and technical step generation.
argument-hint: feature=<name> [request=<rough intent or clarification answers>]
agent: SpecStar Manager
---

Design the feature `${input:featureName:feature-name}` for execution.

Additional user input for this run:
${input:request:optional rough feature intent or answers to existing clarifications}

Work inside:
- `.agent-specstar/features/${input:featureName:feature-name}/`

Required behavior:
1. Inspect existing workflow memory first:
   - `design.md`
   - `clarifications.md` if present
   - `feature-state.json` if present
   - any existing `steps/` artifacts
   - any relevant `sessions/` artifacts if present
2. Determine which phase this feature is currently in:
   - initial spec creation
   - requirements refinement
   - clarification resolution
   - technical design enrichment
   - step generation or regeneration
3. If no usable `design.md` exists, use the provided request to create an initial requirements-first design.
4. If `clarifications.md` exists, treat the current user input as potential clarification answers and reconcile it into the planning flow.
5. If critical ambiguity remains, update `clarifications.md` and stop without generating steps.
6. If the requirements baseline is usable, proceed with architecture/design decomposition.
7. Enrich `design.md` with the missing technical details needed for implementation and review.
8. Generate or update the minimum ordered `step-XXX.md` files required for safe execution.
9. Keep technical cross-step decisions in `design.md`, not scattered across step files.
10. Keep step files narrow, explicit, and execution-oriented.
11. Do not implement code.
12. Do not review code.

Output expectations:
- `design.md` created or updated as needed
- `clarifications.md` created or updated only when unresolved blockers remain
- `steps/step-XXX.md` created or updated only when the feature is ready for execution planning