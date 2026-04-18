---
name: specstar-resume-feature
description: Resume a SpecStar feature from workflow memory and continue from the correct next action.
argument-hint: feature=<name>
agent: SpecStar Coordinator
---

Resume work for feature `${input:featureName:feature-name}` from SpecStar workflow memory.

Work inside:
- `.agent-specstar/features/${input:featureName:feature-name}/`

Required behavior:
1. Reconstruct the current workflow state from artifacts before taking action.
2. Inspect:
   - `feature-state.json`
   - `design.md`
   - `clarifications.md` if present
   - all relevant `steps/`
   - all relevant `reviews/`
   - latest `sessions/` artifacts
3. Determine:
   - current feature status
   - latest completed step
   - active step, if any
   - whether the feature is blocked, in progress, or ready to continue
4. If `feature-state.json` is missing or stale, rebuild the best grounded state from the existing artifacts and update it.
5. Continue the workflow from the correct next action.
6. If the next action is unclear, stop and surface the ambiguity explicitly instead of guessing.

Output expectations:
- reconstructed or updated `feature-state.json`
- resumed execution from the correct point
- no redundant re-planning if valid artifacts already exist